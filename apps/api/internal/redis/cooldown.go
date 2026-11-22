package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/utils"
)

type CooldownStore struct {
	rdb *goredis.Client
}

func NewCooldownStore(rdb *goredis.Client) *CooldownStore {
	return &CooldownStore{rdb: rdb}
}

func (s *CooldownStore) SetCooldown(ctx context.Context, credentialID string, seconds int) error {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.Set(ctx, key, "1", time.Duration(seconds)*time.Second).Err()
}

func (s *CooldownStore) IsCoolingDown(ctx context.Context, credentialID string) (bool, error) {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	val, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "1", nil
}

func (s *CooldownStore) GetCooldownTTL(ctx context.Context, credentialID string) (time.Duration, error) {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.TTL(ctx, key).Result()
}

// CoolingSet reports which of the given credentials are cooling down, with a
// single MGET over their cooldown keys. Only cooling credentials are present
// in the result. A nil store returns an empty set.
func (s *CooldownStore) CoolingSet(ctx context.Context, credentialIDs []string) (map[string]bool, error) {
	cooling := make(map[string]bool)
	if s == nil || s.rdb == nil || len(credentialIDs) == 0 {
		return cooling, nil
	}
	keys := make([]string, len(credentialIDs))
	for i, id := range credentialIDs {
		keys[i] = fmt.Sprintf("credential:%s:cooldown", id)
	}
	vals, err := s.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return cooling, err
	}
	for i, v := range vals {
		if v == "1" {
			cooling[credentialIDs[i]] = true
		}
	}
	return cooling, nil
}

func (s *CooldownStore) RemoveCooldown(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:cooldown", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

func (s *CooldownStore) ClearCooldown(ctx context.Context, credentialID string) error {
	return s.RemoveCooldown(ctx, credentialID)
}

func (s *CooldownStore) SetAccessToken(ctx context.Context, credentialID, token string, ttlSeconds int) error {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Set(ctx, key, token, time.Duration(ttlSeconds)*time.Second).Err()
}

func (s *CooldownStore) GetAccessToken(ctx context.Context, credentialID string) (string, error) {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Get(ctx, key).Result()
}

func (s *CooldownStore) DeleteAccessToken(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:access_token", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

type CredentialQuotaInfo struct {
	RemainingRequests int64  `json:"remainingRequests,omitempty"`
	LimitRequests     int64  `json:"limitRequests,omitempty"`
	RemainingTokens   int64  `json:"remainingTokens,omitempty"`
	LimitTokens       int64  `json:"limitTokens,omitempty"`
	ResetDurationSec  int    `json:"resetDurationSec,omitempty"`
	ResetAt           string `json:"resetAt,omitempty"`
	StatusText        string `json:"statusText,omitempty"`
	LastUpdated       int64  `json:"lastUpdated"`
}

func (s *CooldownStore) SaveCredentialQuota(ctx context.Context, credentialID string, quota *CredentialQuotaInfo) error {
	if credentialID == "" || quota == nil {
		return nil
	}
	quota.LastUpdated = time.Now().Unix()
	data, err := json.Marshal(quota)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	return s.rdb.Set(ctx, key, data, 24*time.Hour).Err()
}

func (s *CooldownStore) GetCredentialQuota(ctx context.Context, credentialID string) (*CredentialQuotaInfo, error) {
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	data, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var quota CredentialQuotaInfo
	if err := json.Unmarshal([]byte(data), &quota); err != nil {
		return nil, err
	}
	return &quota, nil
}

func (s *CooldownStore) DeleteCredentialQuota(ctx context.Context, credentialID string) error {
	key := fmt.Sprintf("credential:%s:quota", credentialID)
	return s.rdb.Del(ctx, key).Err()
}

// ActiveRequestsHashKey prefixes one hash per user of in-flight requests,
// so a dashboard reads only its own entries. The hash expires on its own
// when a user goes quiet.
const (
	ActiveRequestsHashKey = "gateway:active_requests"
	activeRequestsTTL     = 10 * time.Minute
	// activeRequestStaleSec drops a record whose request never untracked
	// itself (the instance died mid-request).
	activeRequestStaleSec = 120
)

func activeRequestsKey(userID string) string {
	if userID == "" {
		userID = "-"
	}
	return ActiveRequestsHashKey + ":" + userID
}

func activeRequestsKeyForTenant(orgID, userID string) string {
	if orgID != "" {
		return ActiveRequestsHashKey + ":org:" + orgID
	}
	return activeRequestsKey(userID)
}

type ActiveRequestRecord struct {
	OrgID        string `json:"orgId,omitempty"`
	UserID       string `json:"userId"`
	Model        string `json:"model"`
	GatewayKeyID string `json:"gatewayKeyId"`
	CredentialID string `json:"credentialId,omitempty"`
	Credential   string `json:"credential"`
	ProviderID   string `json:"providerId,omitempty"`
	StartedAt    int64  `json:"startedAt"`
}

type ActiveStreamsSummary struct {
	TotalActive    int64            `json:"totalActive"`
	ByModel        map[string]int64 `json:"byModel"`
	ByCredential   map[string]int64 `json:"byCredential"`
	ByCredentialID map[string]int64 `json:"byCredentialId,omitempty"`
	ByKey          map[string]int64 `json:"byKey"`
}

func (s *CooldownStore) TrackActiveStream(ctx context.Context, reqID, userID, modelSlug, gatewayKeyID, credName string) error {
	return s.TrackActiveStreamWithCredential(ctx, reqID, userID, modelSlug, gatewayKeyID, "", credName)
}

func (s *CooldownStore) TrackActiveStreamWithCredential(ctx context.Context, reqID, userID, modelSlug, gatewayKeyID, credID, credName string) error {
	if reqID == "" || s == nil || s.rdb == nil {
		return nil
	}
	orgID := database.TenantContextFrom(ctx).OrgID
	rec := ActiveRequestRecord{
		OrgID:        orgID,
		UserID:       userID,
		Model:        modelSlug,
		GatewayKeyID: gatewayKeyID,
		CredentialID: credID,
		Credential:   utils.MaskEmailName(credName),
		StartedAt:    time.Now().Unix(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	key := activeRequestsKeyForTenant(orgID, userID)
	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, key, reqID, data)
	pipe.Expire(ctx, key, activeRequestsTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *CooldownStore) UntrackActiveStream(ctx context.Context, userID, reqID string) error {
	if reqID == "" || s == nil || s.rdb == nil {
		return nil
	}
	orgID := database.TenantContextFrom(ctx).OrgID
	key := activeRequestsKeyForTenant(orgID, userID)
	if err := s.rdb.HDel(ctx, key, reqID).Err(); err != nil {
		return err
	}
	if orgID != "" && userID != "" {
		// Also clean legacy user-keyed entry in case an un-org'd track occurred.
		_ = s.rdb.HDel(ctx, activeRequestsKey(userID), reqID).Err()
	}
	return nil
}

func (s *CooldownStore) IncrementActiveStream(ctx context.Context, modelSlug, gatewayKeyID, credName string) error {
	return nil
}

func (s *CooldownStore) DecrementActiveStream(ctx context.Context, modelSlug, gatewayKeyID, credName string) error {
	return nil
}

// GetActiveStreams summarizes in-flight requests scoped to the active org in ctx (when present)
// or the given userID. When neither orgID nor userID is supplied, it scans all active stream hashes.
func (s *CooldownStore) GetActiveStreams(ctx context.Context, userID string) (*ActiveStreamsSummary, error) {
	summary := &ActiveStreamsSummary{
		TotalActive:    0,
		ByModel:        make(map[string]int64),
		ByCredential:   make(map[string]int64),
		ByCredentialID: make(map[string]int64),
		ByKey:          make(map[string]int64),
	}
	if s == nil || s.rdb == nil {
		return summary, nil
	}

	orgID := database.TenantContextFrom(ctx).OrgID
	var keys []string
	if orgID != "" {
		keys = []string{activeRequestsKeyForTenant(orgID, userID)}
	} else if userID == "" {
		// Unscoped global scan only when neither org nor user is provided.
		all, err := s.scanActiveRequestKeys(ctx)
		if err != nil {
			return summary, err
		}
		keys = all
	} else {
		keys = []string{activeRequestsKey(userID)}
	}

	for _, key := range keys {
		if err := s.summarizeActiveRequests(ctx, key, summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (s *CooldownStore) scanActiveRequestKeys(ctx context.Context) ([]string, error) {
	var keys []string
	iter := s.rdb.Scan(ctx, 0, ActiveRequestsHashKey+":*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

// summarizeActiveRequests adds one hash's live records to summary and drops
// the records that went stale.
func (s *CooldownStore) summarizeActiveRequests(ctx context.Context, key string, summary *ActiveStreamsSummary) error {
	items, err := s.rdb.HGetAll(ctx, key).Result()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return err
	}

	now := time.Now().Unix()
	var staleKeys []string
	for reqID, rawJSON := range items {
		var rec ActiveRequestRecord
		if err := json.Unmarshal([]byte(rawJSON), &rec); err != nil {
			staleKeys = append(staleKeys, reqID)
			continue
		}
		if rec.StartedAt > 0 && (now-rec.StartedAt) > activeRequestStaleSec {
			staleKeys = append(staleKeys, reqID)
			continue
		}

		summary.TotalActive++
		if rec.Model != "" {
			summary.ByModel[rec.Model]++
		}
		if rec.Credential != "" {
			summary.ByCredential[rec.Credential]++
		}
		if rec.CredentialID != "" {
			summary.ByCredentialID[rec.CredentialID]++
		}
		if rec.GatewayKeyID != "" {
			summary.ByKey[rec.GatewayKeyID]++
		}
	}

	if len(staleKeys) > 0 {
		_ = s.rdb.HDel(ctx, key, staleKeys...).Err()
	}
	return nil
}

// Circuit Breaker Methods

const (
	CircuitBreakerThreshold = 3  // 3 consecutive 50x/timeout errors
	CircuitBreakerTTLSec    = 60 // 60 seconds quarantine
)

func (s *CooldownStore) RecordServerError(ctx context.Context, credentialID string, statusCode int) (bool, error) {
	if s == nil || s.rdb == nil || credentialID == "" {
		return false, nil
	}

	key := fmt.Sprintf("credential:%s:50x_count", credentialID)
	count, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return false, err
	}

	if count == 1 {
		_ = s.rdb.Expire(ctx, key, time.Duration(CircuitBreakerTTLSec)*time.Second).Err()
	}

	if count >= int64(CircuitBreakerThreshold) {
		// Put credential into quarantine cooldown for 60 seconds
		_ = s.SetCooldown(ctx, credentialID, CircuitBreakerTTLSec)
		_ = s.rdb.Del(ctx, key).Err()

		// Update quota status to note circuit breaker
		_ = s.SaveCredentialQuota(ctx, credentialID, &CredentialQuotaInfo{
			StatusText:  "circuit_breaker_50x",
			LastUpdated: time.Now().Unix(),
		})

		return true, nil
	}

	return false, nil
}

func (s *CooldownStore) RecordSuccess(ctx context.Context, credentialID string) error {
	if s == nil || s.rdb == nil || credentialID == "" {
		return nil
	}
	key := fmt.Sprintf("credential:%s:50x_count", credentialID)
	return s.rdb.Del(ctx, key).Err()
}
