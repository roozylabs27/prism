package repository

import (
	"context"
	"fmt"

	"github.com/roozylabs/prism/internal/database"
)

func tenantOrgID(ctx context.Context) string {
	return database.TenantContextFrom(ctx).OrgID
}

func tenantUserID(ctx context.Context) string {
	return database.TenantContextFrom(ctx).UserID
}

func tenantWorkspaceID(ctx context.Context) string {
	return database.TenantContextFrom(ctx).WorkspaceID
}

func tenantProjectID(ctx context.Context) string {
	return database.TenantContextFrom(ctx).ProjectID
}

// tenantOwnerClause builds a SQL ownership predicate plus updated positional args.
// When orgID is present in ctx, records owned by the org match directly and legacy
// records with NULL/empty org_id still match via ownerUserID. When orgID is empty,
// ownership falls back strictly to user_id = ownerUserID.
func tenantOwnerClause(ctx context.Context, colPrefix string, ownerUserID string, args []any) (string, []any) {
	orgCol := colPrefix + "org_id"
	userCol := colPrefix + "user_id"
	orgID := tenantOrgID(ctx)
	if orgID == "" {
		args = append(args, ownerUserID)
		return fmt.Sprintf("%s = $%d", userCol, len(args)), args
	}
	args = append(args, orgID, ownerUserID)
	orgIdx := len(args) - 1
	userIdx := len(args)
	return fmt.Sprintf("(%s = $%d OR ((%s IS NULL OR %s = '') AND %s = $%d))", orgCol, orgIdx, orgCol, orgCol, userCol, userIdx), args
}

// appendOptionalWorkspaceClause narrows queries when X-Prism-Workspace-ID is active.
// Rows with empty/NULL workspace_id remain visible at organization level; rows pinned
// to a specific workspace are visible only inside that workspace.
func appendOptionalWorkspaceClause(ctx context.Context, colPrefix string, args []any) (string, []any) {
	wsID := tenantWorkspaceID(ctx)
	if wsID == "" {
		return "", args
	}
	wsCol := colPrefix + "workspace_id"
	args = append(args, wsID)
	return fmt.Sprintf("(%s IS NULL OR %s = '' OR %s = $%d)", wsCol, wsCol, wsCol, len(args)), args
}
