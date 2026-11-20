package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type ProviderRepository struct {
	db *sql.DB
}

func NewProviderRepository(db *sql.DB) *ProviderRepository {
	return &ProviderRepository{db: db}
}

func (r *ProviderRepository) ListByUserID(ctx context.Context, userID string) ([]models.Provider, error) {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, nil)
	query := fmt.Sprintf(
		`SELECT id, COALESCE(user_id, ''), COALESCE(org_id, ''), name, slug, base_url, type, enabled, routing_strategy, created_at, updated_at
		 FROM providers
		 WHERE (%s) OR (user_id IS NULL AND (org_id IS NULL OR org_id = ''))
		 ORDER BY (CASE WHEN %s THEN 0 ELSE 1 END), name ASC`,
		ownerClause, ownerClause,
	)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var providers []models.Provider
	for rows.Next() {
		var p models.Provider
		if err := rows.Scan(&p.ID, &p.UserID, &p.OrgID, &p.Name, &p.Slug, &p.BaseURL,
			&p.Type, &p.Enabled, &p.RoutingStrategy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

func (r *ProviderRepository) FindByID(ctx context.Context, id string) (*models.Provider, error) {
	p := &models.Provider{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, COALESCE(user_id, ''), COALESCE(org_id, ''), name, slug, base_url, type, enabled, routing_strategy, created_at, updated_at
		 FROM providers WHERE id = $1`, id,
	).Scan(&p.ID, &p.UserID, &p.OrgID, &p.Name, &p.Slug, &p.BaseURL,
		&p.Type, &p.Enabled, &p.RoutingStrategy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProviderRepository) Create(ctx context.Context, p *models.Provider) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.OrgID == "" {
		p.OrgID = tenantOrgID(ctx)
	}
	if p.RoutingStrategy == "" {
		p.RoutingStrategy = "round_robin"
	}
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO providers (id, user_id, org_id, name, slug, base_url, type, enabled, routing_strategy, created_at, updated_at)
		 VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, p.UserID, p.OrgID, p.Name, p.Slug, p.BaseURL,
		p.Type, p.Enabled, p.RoutingStrategy, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// Update changes a provider owned by p.OrgID or p.UserID. Shared providers
// (no owner) are changed only by operators, so they never match.
func (r *ProviderRepository) Update(ctx context.Context, p *models.Provider) error {
	if p.RoutingStrategy == "" {
		p.RoutingStrategy = "round_robin"
	}
	p.UpdatedAt = time.Now()
	args := []any{p.Name, p.Slug, p.BaseURL, p.Type, p.Enabled, p.RoutingStrategy, p.UpdatedAt, p.ID}
	ownerClause, args := tenantOwnerClause(ctx, "", p.UserID, args)
	res, err := r.db.ExecContext(ctx,
		`UPDATE providers SET name=$1, slug=$2, base_url=$3, type=$4, enabled=$5, routing_strategy=$6, updated_at=$7
		 WHERE id=$8 AND (`+ownerClause+`)`,
		args...,
	)
	return requireOneRow(res, err)
}

// Delete removes a provider owned by the caller's org or userID. Shared providers never match.
func (r *ProviderRepository) Delete(ctx context.Context, id, userID string) error {
	ownerClause, args := tenantOwnerClause(ctx, "", userID, []any{id})
	res, err := r.db.ExecContext(ctx, `DELETE FROM providers WHERE id = $1 AND (`+ownerClause+`)`, args...)
	return requireOneRow(res, err)
}

func requireOneRow(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
