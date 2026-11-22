package database

import "context"

// TenantDBContext carries the verified caller identity used to configure
// PostgreSQL session variables (app.current_user_id, app.current_org_id, app.rls_bypass) for RLS.
type TenantDBContext struct {
	UserID      string
	OrgID       string
	WorkspaceID string
	ProjectID   string
	BypassRLS   bool
}

type tenantDBContextKey struct{}

// WithTenantContext attaches the canonical tenant context to ctx for downstream
// database RLS enforcement and tenant-scoped repository helpers.
func WithTenantContext(ctx context.Context, tc TenantDBContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, tenantDBContextKey{}, tc)
}

// WithSystemBypassRLS returns a context authorized to bypass row-level security for
// explicit pre-authentication lookups and cross-tenant background workers.
func WithSystemBypassRLS(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	tc := TenantContextFrom(ctx)
	tc.BypassRLS = true
	return context.WithValue(ctx, tenantDBContextKey{}, tc)
}

// TenantContextFrom returns the TenantDBContext stored in ctx, or a zero-value
// TenantDBContext when no tenant scope is present.
func TenantContextFrom(ctx context.Context) TenantDBContext {
	if ctx == nil {
		return TenantDBContext{}
	}
	if tc, ok := ctx.Value(tenantDBContextKey{}).(TenantDBContext); ok {
		return tc
	}
	return TenantDBContext{}
}
