package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/roozylabs/prism/internal/database"
)

type inviteBypassCheckDriver struct {
	mu           sync.Mutex
	bypassStates []bool
}

func (d *inviteBypassCheckDriver) recordCtx(ctx context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bypassStates = append(d.bypassStates, database.TenantContextFrom(ctx).BypassRLS)
}

func (d *inviteBypassCheckDriver) snapshot() []bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]bool, len(d.bypassStates))
	copy(out, d.bypassStates)
	return out
}

type inviteBypassConnector struct{ drv *inviteBypassCheckDriver }

func (c *inviteBypassConnector) Connect(context.Context) (driver.Conn, error) {
	return &inviteBypassConn{drv: c.drv}, nil
}
func (c *inviteBypassConnector) Driver() driver.Driver { return c.drv }
func (d *inviteBypassCheckDriver) Open(string) (driver.Conn, error) {
	return &inviteBypassConn{drv: d}, nil
}

type inviteBypassConn struct{ drv *inviteBypassCheckDriver }

func (c *inviteBypassConn) Prepare(query string) (driver.Stmt, error) {
	return &inviteBypassStmt{drv: c.drv, query: query}, nil
}
func (c *inviteBypassConn) Close() error              { return nil }
func (c *inviteBypassConn) Begin() (driver.Tx, error) { return &inviteBypassTx{}, nil }
func (c *inviteBypassConn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.drv.recordCtx(ctx)
	return &inviteBypassTx{}, nil
}
func (c *inviteBypassConn) QueryContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.drv.recordCtx(ctx)
	return &inviteBypassRows{query: query}, nil
}

type inviteBypassStmt struct {
	drv   *inviteBypassCheckDriver
	query string
}

func (s *inviteBypassStmt) Close() error                                    { return nil }
func (s *inviteBypassStmt) NumInput() int                                   { return -1 }
func (s *inviteBypassStmt) Exec([]driver.Value) (driver.Result, error)      { return driver.RowsAffected(0), nil }
func (s *inviteBypassStmt) Query([]driver.Value) (driver.Rows, error)       { return &inviteBypassRows{query: s.query}, nil }
func (s *inviteBypassStmt) QueryContext(ctx context.Context, _ []driver.NamedValue) (driver.Rows, error) {
	s.drv.recordCtx(ctx)
	return &inviteBypassRows{query: s.query}, nil
}

type inviteBypassTx struct{}

func (t *inviteBypassTx) Commit() error   { return nil }
func (t *inviteBypassTx) Rollback() error { return nil }

type inviteBypassRows struct{ query string }

func (r *inviteBypassRows) Columns() []string {
	if strings.Contains(r.query, "FROM member_invites i") {
		return []string{"id", "org_id", "role_id", "role", "email", "token", "status", "invited_by", "accepted_by", "accepted_at", "expires_at", "created_at"}
	}
	return []string{"email", "role", "org", "expires_at"}
}
func (r *inviteBypassRows) Close() error              { return nil }
func (r *inviteBypassRows) Next([]driver.Value) error { return io.EOF }

func TestInvitePreviewAndAcceptEnableSystemBypassRLS(t *testing.T) {
	drv := &inviteBypassCheckDriver{}
	db := sql.OpenDB(&inviteBypassConnector{drv: drv})
	defer db.Close()

	repo := NewRBACRepository(db)

	_, err := repo.PreviewInvite(context.Background(), "raw-token")
	if err != ErrInviteNotFound {
		t.Fatalf("PreviewInvite err = %v, want ErrInviteNotFound", err)
	}

	_, err = repo.AcceptInvite(context.Background(), "raw-token", "u-1", "a@example.com")
	if err != ErrInviteNotFound {
		t.Fatalf("AcceptInvite err = %v, want ErrInviteNotFound", err)
	}

	states := drv.snapshot()
	if len(states) < 2 {
		t.Fatalf("expected recorded DB calls for PreviewInvite and AcceptInvite, got %d", len(states))
	}
	for i, bypass := range states {
		if !bypass {
			t.Fatalf("call %d did not carry SystemBypassRLS context (DEF-009)", i)
		}
	}
}
