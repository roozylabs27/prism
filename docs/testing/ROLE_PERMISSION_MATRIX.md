# Prism — Role & Permission Matrix

**Source of Truth:** `apps/api/migrations/064_enterprise_rbac_oauth_billing.up.sql`, `077_harden_authorization_and_workspace_members.up.sql`, `078_normalize_role_id.up.sql`, `106_routing_permissions.up.sql`, and `apps/api/cmd/server/main.go`.  
**Status:** Baseline Expected Matrix (Stage 1) — Live UI & API Verification Columns to be populated in Stages 2–5.

---

## 1. System Role Availability

| Role Slug | Display Name | Seeded in DB (`roles`) | Assignable via UI/API | Notes |
|---|---|---|---|---|
| `owner` | Owner / Org Admin | Yes | Yes | Holds all permissions including `workspace:admin` |
| `admin` | Admin | **No** | **No** (`400 INVALID_ROLE`) | Exists only in frontend `UserRole` enum (`types/roles.ts`) |
| `developer` | Developer / AI Engineer | Yes | Yes | No `workspace:admin` |
| `agent_manager` | Agent Administrator | Yes | Yes | No `workspace:admin` |
| `finops_manager` | FinOps / Budget Manager | Yes | Yes | No `workspace:admin` |
| `auditor` | Security Auditor | Yes | Yes | No `workspace:admin` |
| `member` | Member | **No** | **No** (`400 INVALID_ROLE`) | Exists only as fallback in frontend `normalizeRoleSlug` |
| `viewer` | Viewer / Guest | Yes | Yes | No `workspace:admin` |

---

## 2. Backend Permission Matrix (6 Active System Roles)

Legend: `ALLOW` = granted in `role_permissions` | `DENY` = not granted (backend returns `403 PERMISSION_DENIED`)

| Permission Code | Protected API Endpoints (`main.go`) | `owner` | `developer` | `agent_manager` | `finops_manager` | `auditor` | `viewer` |
|---|---|---|---|---|---|---|---|
| `organization:read` | `GET /settings`, `GET /sse`, `GET /workspaces`, `GET /workspaces/:id/projects` | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| `organization:update` | `PUT /settings`, `POST/PUT/DELETE /workspaces`, `POST/PUT/DELETE /workspaces/:id/projects` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `organization:delete` | *(no route registered)* | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `workspace:admin` | Cross-workspace override in `AuthorizationEngine.Can` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `member:read` | `GET /organizations/members`, `GET /organizations/invites` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `member:invite` | `POST /organizations/invites`, `DELETE /organizations/invites/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `member:update` | `PUT /organizations/members/:userId` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `member:remove` | `DELETE /organizations/members/:userId` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `api_key:read` | `GET /gateway-keys` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `api_key:create` | `POST /gateway-keys` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `api_key:revoke` | `DELETE /gateway-keys/:id` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `credential:read` | `GET /credentials`, `GET /providers/:id/credentials`, `/:credId` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `credential:create` | `POST /providers/:id/credentials`, Google OAuth | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `credential:update` | `PUT /providers/:id/credentials/:credId`, `test`, `reveal`, `reset-cooldown` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `credential:delete` | `DELETE /providers/:id/credentials/:credId`, `DELETE /credentials/:credId` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `model:read` | `GET /providers`, `GET /models`, `GET /dashboard/health` | ALLOW | ALLOW | DENY | DENY | DENY | ALLOW |
| `model:create` | `POST /providers`, `POST /providers/:id/models` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `model:update` | `PUT /providers/:id`, `PUT /providers/:id/models/:id`, `PATCH .../capabilities` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `model:delete` | `DELETE /providers/:id`, `DELETE /providers/:id/models/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `routing:read` | `GET /routing-rules`, `GET /routing-policies`, `GET /routing/decisions`, `POST /routing/simulate`, `/replay` | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| `routing:update` | `POST/PUT/DELETE /routing-rules`, `POST/PUT/DELETE /routing-policies`, `PUT .../default` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `playground:execute` | `POST /sandbox/chat/completions`, `/async`, `GET /jobs/:jobId` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `agent:read` | `GET /agents`, `/:id`, `/:id/stats`, `GET /agent-templates` | ALLOW | ALLOW | ALLOW | DENY | DENY | ALLOW |
| `agent:create` | `POST /agents`, `POST /agent-templates`, `.../instantiate` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `agent:update` | `PUT /agents/:id` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `agent:delete` | `DELETE /agents/:id` | ALLOW | DENY | ALLOW | DENY | DENY | DENY |
| `tool:read` | `GET /tools`, `/:id` | ALLOW | ALLOW | ALLOW | DENY | DENY | ALLOW |
| `tool:create` | `POST /tools` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `tool:update` | `PUT /tools/:id` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `tool:delete` | `DELETE /tools/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `tool:execute` | `POST /tools/:id/test` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `resource:read` | `GET /resources`, `/:id` | ALLOW | ALLOW | DENY | DENY | DENY | ALLOW |
| `resource:create` | `POST /resources` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `resource:update` | `PUT /resources/:id` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `resource:delete` | `DELETE /resources/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `resource:query` | `POST /resources/:id/test` | ALLOW | ALLOW | DENY | DENY | DENY | DENY |
| `mcp:read` | `GET /mcp/servers`, `/:id`, `.../tools`, `.../stats`, `GET /mcp/registry` | ALLOW | ALLOW | ALLOW | DENY | DENY | ALLOW |
| `mcp:create` | `POST /mcp/servers`, `POST /mcp/registry` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `mcp:update` | `PUT /mcp/servers/:id`, `POST .../sync` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `mcp:delete` | `DELETE /mcp/servers/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `mcp:execute` | `POST /mcp/servers/:id/test` | ALLOW | ALLOW | ALLOW | DENY | DENY | DENY |
| `governance:read` | `GET /governance/policies`, `/:id`, `GET /approvals`, `/:id` | ALLOW | DENY | ALLOW | DENY | ALLOW | DENY |
| `governance:create` | `POST /governance/policies` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `governance:update` | `PUT /governance/policies/:id`, `POST /approvals/:id/approve`, `/deny` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `governance:delete` | `DELETE /governance/policies/:id` | ALLOW | DENY | DENY | DENY | DENY | DENY |
| `governance:evaluate` | `POST /governance/evaluate` | ALLOW | DENY | ALLOW | DENY | DENY | DENY |
| `budget:read` | `GET /budgets`, `/status`, `/:id` | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| `budget:create` | `POST /budgets` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `budget:update` | `PUT /budgets/:id` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `budget:delete` | `DELETE /budgets/:id` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `quota:read` | `GET /quotas`, `/:target_type/:target_id` | ALLOW | ALLOW | DENY | ALLOW | ALLOW | ALLOW |
| `quota:update` | `PUT /quotas/:target_type/:target_id` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `finops:read` | `GET /analytics/finops`, `/anomalies`, `/budget-alerts` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `finops:manage_budget` | `POST /analytics/finops/budget-alerts/:id/acknowledge` | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `billing:read` | `GET /billing/*` *(feature-flagged off)* | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `billing:manage` | `POST /billing/subscription/upgrade` *(feature-flagged off)* | ALLOW | DENY | DENY | ALLOW | DENY | DENY |
| `logs:read` | `GET /logs`, `GET /analytics/logs`, `GET /dashboard/stats`, `/usage`, `/active-streams` | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| `audit:read` | `GET /audit-trail`, `/logs`, `/:id` | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW | ALLOW |
| `audit:export` | `GET /audit-trail/export` | ALLOW | DENY | DENY | ALLOW | ALLOW | DENY |
| `audit:verify` | `POST /audit-trail/:id/verify` | ALLOW | DENY | DENY | DENY | ALLOW | DENY |

---

## 3. Notable Permission Anomalies Flagged for Live Testing

1. **Dashboard Overview (`/`) calls `GET /api/dashboard/health` (`model:read`)**: `agent_manager`, `finops_manager`, and `auditor` do **not** have `model:read`! We will check whether `/` triggers a `403` error on `/dashboard/health` for those roles.
2. **Developer can `reveal` and `test` credentials (`credential:update`)** and **update providers/models (`model:update`)**, but cannot `create` or `delete` providers/models or `delete` credentials.
3. **Agent Manager cannot read models (`model:read`) or gateway keys (`api_key:read`)** — we will verify whether `/agents` or `/sandbox` works cleanly without `model:read`!
4. **Newly invited non-Owner users lack `workspace_members` rows** — we will verify whether switching to or operating inside a workspace returns `403` for invited non-owners.
5. **Frontend `checkHasPermission` hardcodes `if (role === UserRole.ADMIN) return true`**, even though `admin` is not a seeded system role in PostgreSQL.
