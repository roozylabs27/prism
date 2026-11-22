package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuthUsers struct {
	byEmail map[string]*models.User
}

func (f *fakeAuthUsers) FindByEmail(_ context.Context, email string) (*models.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakeAuthUsers) FindByID(_ context.Context, id string) (*models.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeAuthUsers) Create(_ context.Context, u *models.User) error {
	f.byEmail[u.Email] = u
	return nil
}

func (f *fakeAuthUsers) UpdateProfile(_ context.Context, u *models.User) error {
	f.byEmail[u.Email] = u
	return nil
}

type fakeAuthSessions struct {
	created       []*models.Session
	revokedBy     []string
	revokedExcept []string
}

func (f *fakeAuthSessions) Create(_ context.Context, s *models.Session) error {
	f.created = append(f.created, s)
	return nil
}

func (f *fakeAuthSessions) FindValidByToken(context.Context, string) (*models.Session, error) {
	return nil, sql.ErrNoRows
}

func (f *fakeAuthSessions) Delete(context.Context, string) error { return nil }

func (f *fakeAuthSessions) DeleteAllByUserID(_ context.Context, userID string) error {
	f.revokedBy = append(f.revokedBy, userID)
	return nil
}

func (f *fakeAuthSessions) DeleteAllByUserIDExcept(_ context.Context, userID, keepToken string) error {
	f.revokedExcept = append(f.revokedExcept, userID+":"+keepToken)
	return nil
}

type fakeAuthAccounts struct {
	byUser          map[string]*models.Account
	deletedCredsFor []string
	upsertedOAuth   []*models.Account
}

func (f *fakeAuthAccounts) FindByUserID(_ context.Context, userID string) (*models.Account, error) {
	if f.byUser != nil {
		if a, ok := f.byUser[userID]; ok {
			return a, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (f *fakeAuthAccounts) Create(context.Context, *models.Account) error { return nil }

func (f *fakeAuthAccounts) DeleteCredentialByUserID(_ context.Context, userID string) error {
	f.deletedCredsFor = append(f.deletedCredsFor, userID)
	return nil
}

func (f *fakeAuthAccounts) UpdateCredentialPassword(_ context.Context, userID, hashedPassword string) error {
	if f.byUser == nil || f.byUser[userID] == nil {
		return sql.ErrNoRows
	}
	f.byUser[userID].Password = hashedPassword
	return nil
}

func (f *fakeAuthAccounts) UpsertOAuth(_ context.Context, a *models.Account) error {
	for i, existing := range f.upsertedOAuth {
		if existing.UserID == a.UserID && existing.ProviderID == a.ProviderID {
			f.upsertedOAuth[i] = a
			return nil
		}
	}
	f.upsertedOAuth = append(f.upsertedOAuth, a)
	return nil
}

func TestAuthService_Structures(t *testing.T) {
	loginReq := LoginRequest{
		Email:          "test@example.com",
		Password:       "Password123!",
		TurnstileToken: "token123",
	}
	if loginReq.Email != "test@example.com" {
		t.Errorf("unexpected email in LoginRequest")
	}

	signupReq := SignupRequest{
		Name:           "Test User",
		Email:          "test@example.com",
		Password:       "Password123!",
		TurnstileToken: "token123",
	}
	if signupReq.Name != "Test User" || signupReq.Email != "test@example.com" {
		t.Errorf("unexpected values in SignupRequest")
	}
}

func TestCreateOAuthSession_NeutralizesUnverifiedPreAccountAndUpserts(t *testing.T) {
	users := &fakeAuthUsers{byEmail: map[string]*models.User{
		"victim@example.com": {
			ID:            "u_victim",
			Email:         "victim@example.com",
			Name:          "Pre-Signup Attacker",
			EmailVerified: false,
		},
		"verified@example.com": {
			ID:            "u_verified",
			Email:         "verified@example.com",
			Name:          "Verified User",
			EmailVerified: true,
		},
	}}
	sessions := &fakeAuthSessions{}
	accounts := &fakeAuthAccounts{}
	svc := &AuthService{users: users, sessions: sessions, accounts: accounts}

	// 1. OAuth over unverified pre-registered email deletes attacker's credential & revokes sessions.
	resp, err := svc.CreateOAuthSession(context.Background(), "victim@example.com", "Real Victim", "", "google", "1.2.3.4", "ua")
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.True(t, resp.User.EmailVerified)
	assert.Equal(t, []string{"u_victim"}, accounts.deletedCredsFor)
	assert.Equal(t, []string{"u_victim"}, sessions.revokedBy)

	// 2. Repeated OAuth login keeps single upserted link without touching verified user's credential.
	_, err = svc.CreateOAuthSession(context.Background(), "verified@example.com", "Verified User", "", "google", "1.2.3.4", "ua")
	require.NoError(t, err)
	_, err = svc.CreateOAuthSession(context.Background(), "verified@example.com", "Verified User", "", "google", "1.2.3.4", "ua")
	require.NoError(t, err)
	assert.Equal(t, []string{"u_victim"}, accounts.deletedCredsFor, "already-verified credential account must be preserved")
	assert.Len(t, accounts.upsertedOAuth, 2, "must maintain at most one OAuth link per (user_id, provider_id)")
}

func TestChangePasswordAndRevokeAllSessions(t *testing.T) {
	initialHash, err := utils.HashPassword("old-password-123")
	require.NoError(t, err)

	sessions := &fakeAuthSessions{}
	accounts := &fakeAuthAccounts{
		byUser: map[string]*models.Account{
			"u_1": {ID: "acc_1", UserID: "u_1", ProviderID: "credential", Password: initialHash},
		},
	}
	svc := &AuthService{sessions: sessions, accounts: accounts}

	// Wrong current password rejected
	err = svc.ChangePassword(context.Background(), "u_1", "wrong-password", "new-password-456", "tok_curr")
	require.ErrorIs(t, err, ErrInvalidCredentials)

	// Weak new password rejected
	err = svc.ChangePassword(context.Background(), "u_1", "old-password-123", "short", "tok_curr")
	require.Error(t, err)

	// Valid password change updates hash and revokes other sessions
	err = svc.ChangePassword(context.Background(), "u_1", "old-password-123", "new-password-456", "tok_curr")
	require.NoError(t, err)
	assert.True(t, utils.CheckPassword("new-password-456", accounts.byUser["u_1"].Password))
	assert.Equal(t, []string{"u_1:tok_curr"}, sessions.revokedExcept)

	// Revoke all sessions
	require.NoError(t, svc.RevokeAllSessions(context.Background(), "u_1"))
	assert.Equal(t, []string{"u_1"}, sessions.revokedBy)
}
