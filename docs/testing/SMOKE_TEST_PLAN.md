# Prism — Full Platform Interactive Smoke Test Plan

**Repository:** `roozylabs27/prism` (`apps/console`, `apps/api`)  
**Target Environment:** Local Development (`APP_ENV=development`, `TURNSTILE_DISABLED=true`)  
**Console URL:** `http://localhost:3000`  
**API URL:** `http://localhost:8080`  
**Plan Date:** 2026-11-22  
**Status:** Stage 1 Complete — Ready for Review before Stage 2 Execution

---

## 1. Test Prerequisites & Safety Controls

| Prerequisite | Confirmed Value |
|---|---|
| **Environment Type** | Local Development (`APP_ENV=development`, `POSTGRES_DB=prism`) |
| **Console URL** | `http://localhost:3000` |
| **API URL** | `http://localhost:8080` |
| **Test Organizations & Accounts** | Dedicated test organizations & role accounts created via signup/invitations in the local dev environment (plus verification against seeded matrix accounts from migration `079` where applicable for workspace-scoped checks). |
| **Provider Credentials & External Calls** | Permitted to use provider credentials already configured in the local test environment for low-cost test requests. |
| **Test Data Mutation Policy** | Safe to create and delete dedicated test data (orgs, workspaces, projects, keys, policies, budgets, agents, tools, resources, MCP servers). |
| **Turnstile CAPTCHA** | Disabled (`TURNSTILE_DISABLED=true`). |
| **Secret Hygiene** | No raw API keys, passwords, cookies, or session tokens will be recorded in screenshots or reports. |
| **Code Modification Policy** | Zero application code modifications during the smoke test phase. |

---

## 2. Feature Inventory (UI vs Backend Comparison)

| # | Module | Console UI (`apps/console`) | Backend API (`apps/api`) | Implementation Status |
|---|---|---|---|---|
| 1 | **Authentication & Onboarding** | `/signin`, `/signup`, `/onboarding`, `/accept-invite` | `POST /api/auth/login`, `/signup`, `/logout`, `GET /api/auth/me`, `POST /api/auth/change-password`, `/sessions/revoke-all`, `POST /api/onboarding`, `GET /api/auth/invites/:token`, `POST /api/invites/accept`, OAuth routes | **Implemented** (OAuth requires external provider creds; local email/password + invite flow active). |
| 2 | **Dashboard Overview** | `/` (KPIs, usage chart, provider health, active streams, SSE live updates, date period selector) | `GET /api/dashboard/stats`, `/usage`, `/health`, `/active-streams`, `GET /api/sse` | **Implemented**. |
| 3 | **Organization Settings** | `/settings` (redirects to `/settings/organization`), `/settings/organization` | `GET /api/settings`, `PUT /api/settings`, `GET /api/organizations` | **Implemented**. Note: no dedicated user profile/notification preferences page (`/settings` redirects to org settings); no `DELETE /api/organizations/:id` endpoint. |
| 4 | **Team & Invitations** | `/settings/members` (list members, invite sheet with one-time link fallback when SMTP unset, change role, remove member, list/revoke pending invites) | `GET/POST/DELETE /api/organizations/invites`, `GET/PUT/DELETE /api/organizations/members` | **Implemented**. Enforces atomic Last-Owner protection. **Gap:** No UI/API for custom roles (`role:*`) despite `RBACRepository.CreateCustomRole` existing. |
| 5 | **Workspaces & Projects** | `/settings/organization` (`WorkspacesProjectsSection` + `ClientOnboardingModal`) & topbar `TenantSelector` | `GET/POST/PUT/DELETE /api/workspaces`, `GET/POST/PUT/DELETE /api/workspaces/:workspaceId/projects` | **Implemented**. **Gap 1:** Guarded by `organization:read`/`organization:update` rather than `workspace:*`/`project:*`. **Gap 2:** No HTTP endpoint or UI to manage `workspace_members`! |
| 6 | **Providers** | `/providers` | `GET/POST/PUT/DELETE /api/providers` | **Implemented** (guarded by `model:*` permissions). |
| 7 | **Provider Credentials** | `/credentials` | `GET/POST/PUT/DELETE /api/providers/:id/credentials`, `POST .../test`, `.../reveal`, `.../reset-cooldown`, Google OAuth login/callback | **Implemented** (guarded by `credential:*`). |
| 8 | **Models** | `/models` | `GET/POST/PUT/DELETE /api/providers/:id/models`, `GET /api/models`, `PATCH .../capabilities` | **Implemented** (guarded by `model:*`). |
| 9 | **Gateway Keys** | `/gateway-keys` | `GET/POST/DELETE /api/gateway-keys` | **Partially Implemented**: Create, list, revoke, scope to workspace/project/agent. **Gap:** No `PUT`/`PATCH` endpoint to update an existing key's configuration after creation; no key rotation endpoint. |
| 10 | **Routing Policies & Rules** | `/policies` | `GET/POST/PUT/DELETE /api/routing-policies` (plus `/policies`, `/routing/policies` aliases), `PUT .../default`, `GET/POST/PUT/DELETE /api/routing-rules`, `GET /api/routing/decisions` | **Implemented** (guarded by `routing:read` / `routing:update`). |
| 11 | **Routing Simulator (Playground)** | `/playground` | `POST /api/routing/simulate`, `POST /api/routing/replay` | **Implemented** (guarded by `routing:read`). |
| 12 | **AI Sandbox** | `/sandbox` | `POST /api/sandbox/chat/completions`, `/async`, `GET /api/jobs/:jobId` | **Implemented** (guarded by `playground:execute`). |
| 13 | **Budgets, Quotas & FinOps** | `/budgets` | `GET/POST/PUT/DELETE /api/budgets`, `GET /api/budgets/status`, `GET/PUT /api/quotas`, `GET /api/analytics/finops`, `/anomalies`, `/budget-alerts`, `POST .../acknowledge` | **Implemented** (guarded by `budget:*`, `quota:*`, `finops:*`). |
| 14 | **Request Logs & Analytics** | `/logs` | `GET /api/logs`, `GET /api/analytics/logs`, `GET /api/analytics/pilot-outcomes`, `/export` | **Implemented** (guarded by `logs:read`). |
| 15 | **Audit Trail** | `/audit-trail` | `GET /api/audit-trail`, `/logs`, `/export`, `/:id`, `POST /:id/verify` | **Implemented** (guarded by `audit:read`, `audit:export`, `audit:verify`). |
| 16 | **Agents & Templates** | `/agents`, `/agents/[id]` | `GET/POST/PUT/DELETE /api/agents`, `GET /api/agents/:id/stats`, `GET/POST /api/agent-templates`, `POST .../instantiate` | **Implemented** (guarded by `agent:*`). |
| 17 | **Tool Gateway** | `/tools` | `GET/POST/PUT/DELETE /api/tools`, `POST /api/tools/:id/test`, `POST /v1/tools/:toolName/execute` | **Implemented** (guarded by `tool:*`). |
| 18 | **Resource Gateway** | `/resources` | `GET/POST/PUT/DELETE /api/resources`, `POST /api/resources/:id/test`, `POST /v1/resources/:resourceName/query` | **Implemented** (`RESOURCE_POSTGRES_ENABLED=false` in dev `.env`). |
| 19 | **MCP Gateway & Registry** | `/mcp`, `/mcp/[id]` | `GET/POST/PUT/DELETE /api/mcp/servers`, `.../sync`, `.../test`, `.../tools`, `.../stats`, `GET/POST /api/mcp/registry`, `POST /v1/mcp` | **Implemented** (guarded by `mcp:*`). |
| 20 | **Governance & RBAC Policies** | `/governance` | `GET/POST/PUT/DELETE /api/governance/policies`, `POST /api/governance/evaluate` | **Implemented** (guarded by `governance:*`). |
| 21 | **Human-in-the-Loop Approvals** | `/approvals` | `GET /api/approvals`, `/:id`, `POST /:id/approve`, `/:id/deny`, `GET /v1/approvals/:approvalId` | **Implemented** (guarded by `governance:read` / `governance:update`). |
| 22 | **Billing & Plans** | `/settings/billing` (hidden in sidebar unless `billing` flag enabled) | `GET /api/billing/plans`, `/subscription`, `/invoices`, `/usage`, `POST /api/billing/subscription/upgrade` | **Feature-Flagged / Placeholder**: Disabled by default (`FEATURE_FLAG_BILLING=false`); backend returns feature-disabled error unless flag is turned on. |

---

## 3. Route Inventory (`apps/console`)

### Public / Auth / Onboarding Routes
- `/signin` — Email/password & OAuth sign-in
- `/signup` — New account registration
- `/onboarding` — First-time organization, workspace, and gateway key wizard
- `/accept-invite` — Invitation preview & acceptance flow

### Protected Dashboard Routes (all gated by `middleware.ts` `auth_token` check)
- `/` — Overview (`API Gateway Overview`)
- `/sandbox` — AI Sandbox (`Developer Web Sandbox`)
- `/logs` — Request Logs
- `/audit-trail` — End-to-End AI Audit Trail
- `/providers` — Providers
- `/credentials` — Provider Credentials Vault
- `/models` — Model Catalog
- `/policies` — Routing Policies & Rules
- `/gateway-keys` — Gateway API Keys
- `/agents` & `/agents/[id]` — Agent Gateway & Agent Detail
- `/tools` — Tool Gateway
- `/resources` — Resource Gateway
- `/mcp` & `/mcp/[id]` — MCP Gateway & Server Detail
- `/playground` — Routing Simulator & Dry-Run Playground
- `/governance` — Governance & RBAC Policies
- `/approvals` — Human Approval Inbox
- `/budgets` — Budgets, Quotas & FinOps
- `/settings` — Redirects to `/settings/organization`
- `/settings/organization` — Organization Parameters, Workspaces & Projects
- `/settings/members` — Team Members & Pending Invitations
- `/settings/billing` — Billing & Plans (behind `billing` feature flag)

### Proxy / Health Routes
- `/api/health` — Console health check
- `/api/[[...path]]` — Next.js BFF proxy to Go API (`http://localhost:8080`)

---

## 4. Role Inventory (Critical Stage 1 Finding)

The smoke test prompt requests testing 8 roles (`Owner`, `Admin`, `Developer`, `Agent Manager`, `FinOps Manager`, `Auditor`, `Member`, `Viewer`). Codebase inspection of `apps/api/migrations/` (`064`, `077`, `078`, `106`), `apps/api/internal/repository/rbac.go`, and `apps/console/types/roles.ts` reveals:

| Role | Backend DB (`roles` table, `is_system=true`) | Console UI Dropdown (`/settings/members`) | Frontend `UserRole` Enum (`types/roles.ts`) | Testability Status |
|---|---|---|---|---|
| **Owner** (`owner`) | Seeded (`...0001`) | Available | `UserRole.OWNER` | **Fully Testable** |
| **Admin** (`admin`) | **NOT SEEDED** in `roles` table | **Not Available** | `UserRole.ADMIN` (hardcoded frontend bypass in `checkHasPermission`!) | **BLOCKED / Backend Gap** — API `POST /invites` & `PUT /members/:id` return `400 INVALID_ROLE`. Will verify negative rejection explicitly. |
| **Developer** (`developer`) | Seeded (`...0002`) | Available | `UserRole.DEVELOPER` | **Fully Testable** |
| **Agent Manager** (`agent_manager`) | Seeded (`...0003`) | Available | `UserRole.AGENT_MANAGER` | **Fully Testable** |
| **FinOps Manager** (`finops_manager`) | Seeded (`...0004`) | Available | `UserRole.FINOPS_MANAGER` | **Fully Testable** |
| **Auditor** (`auditor`) | Seeded (`...0005`) | Available | `UserRole.AUDITOR` | **Fully Testable** |
| **Member** (`member`) | **NOT SEEDED** in `roles` table | **Not Available** | `UserRole.MEMBER` (default fallback in `normalizeRoleSlug`) | **BLOCKED / Backend Gap** — API `POST /invites` & `PUT /members/:id` return `400 INVALID_ROLE`. Will verify negative rejection explicitly. |
| **Viewer** (`viewer`) | Seeded (`...0006`) | Available | `UserRole.VIEWER` | **Fully Testable** |

---

## 5. Permission Inventory & Expected Matrix

Full role-to-permission and endpoint mapping is maintained in [ROLE_PERMISSION_MATRIX.md](file:///c:/me/roozylabs/projects/prism/docs/testing/ROLE_PERMISSION_MATRIX.md).

### Key Architectural Observations to Verify in Live Testing
1. **Workspace Access Gate vs Invitations:** `AuthorizationEngine.Can` requires `workspace:admin` (only `owner` has this) OR a `workspace_members` row whenever `X-Prism-Workspace-ID` is sent. However, `AcceptInvite` only inserts into `organization_members` (not `workspace_members`), and there is no `/workspaces/:id/members` HTTP endpoint. We will test whether newly invited non-owner users get blocked by `403 PERMISSION_DENIED` on workspace-scoped requests!
2. **Sidebar Visibility:** `DashboardSidebar.tsx` renders all non-flagged routes regardless of role; access control relies on page-level `PermissionGuard` and backend `RequirePermission`. We will verify whether unauthorized pages gracefully show empty/denied states vs noisy 403 error toasts or crashes.
3. **Existing Playwright Coverage (`apps/console/e2e/dashboard.spec.ts`):** Only covers 1 serial Owner happy-path (signup, CSRF origin check, onboarding, SSE, workspace creation/switch, route load for `/approvals`, `/playground`, `/sandbox`, `/gateway-keys`, logout). Zero coverage of RBAC roles, CRUD flows, tenant isolation, or negative authorization.

---

## 6. Test Scenarios & Execution Plan (Stages 2–6)

| Stage | Scope | Scenario IDs | Expected Outcomes |
|---|---|---|---|
| **Stage 2 — Owner & Admin Baseline** | Auth (signin/signup/invalid creds/session persistence/protected routes/logout), Onboarding, Org Settings, Workspaces & Projects CRUD, Members & Invites (including Last-Owner protection & `admin`/`member` role negative test), Providers/Credentials/Models CRUD, Gateway Keys CRUD & `/v1` revocation check, Routing Policies & Simulator, Sandbox execution, Budgets/Quotas, Agents/Tools/Resources/MCP, Governance & Approvals, Logs & Audit Trail (verify/export). | `SMK-AUTH-01..06`, `SMK-ORG-01..07`, `SMK-WS-01..06`, `SMK-PROV-01..06`, `SMK-KEY-01..05`, `SMK-RTE-01..05`, `SMK-SBX-01..03`, `SMK-FIN-01..04`, `SMK-OBS-01..05`, `SMK-AGT-01..06`, `SMK-GOV-01..04`, `SMK-ADM-01` | All authorized Owner mutations succeed, persist across reload, and emit clean network/console states. Last-owner removal/demotion returns 400. Inviting `admin`/`member` returns 400 `INVALID_ROLE`. |
| **Stage 3 — Operational Roles** | Independent browser + API verification for **Developer**, **Agent Manager**, and **FinOps Manager** (Allowed, Denied UI + API 403, Workspace Boundary). | `SMK-DEV-01..06`, `SMK-AGM-01..06`, `SMK-FINM-01..06` | Each role succeeds on its permitted modules and receives HTTP `403 PERMISSION_DENIED` (with no state change) on unauthorized mutations/reads. |
| **Stage 4 — Read-Only & Limited Roles** | Independent browser + API verification for **Auditor** and **Viewer** (plus documenting **Member** unseeded gap). | `SMK-AUD-01..06`, `SMK-VWR-01..06`, `SMK-MEM-01` | Auditor can read/verify/export audit trail & logs, denied on all writes. Viewer can read permitted views, denied on all writes and restricted reads (credentials, gateway keys, members). |
| **Stage 5 — Tenant Isolation & Security Negative Tests** | Two dedicated organizations (Org A & Org B), org switching cache hygiene, cross-org `X-Prism-Org-ID` & resource ID access attempts, cross-workspace access attempts, CSRF untrusted Origin check, Gateway Key scope & revocation enforcement. | `SMK-ISO-01..07` | Zero cross-tenant reads/updates/deletes (`403`/`404`). No stale cache leak after org switch. Revoked gateway key returns `401`. Untrusted Origin returns `403 CSRF_ORIGIN_REJECTED`. |
| **Stage 6 — End-to-End Business Journeys** | Full Journeys A (Owner Setup), B (Developer Usage), C (FinOps), D (Auditor), E (Viewer), F (Tenant Isolation), plus Agent Manager Journey G. | `JRN-A` .. `JRN-G` | All journeys complete end-to-end with verified request logs, audit trail entries, and role boundaries. |

---

## 7. Known Limitations (Discovered Prior to Execution)

1. **`admin` and `member` system roles are not seeded in PostgreSQL** (`roles` table only has 6 system roles), causing a mismatch with `apps/console/types/roles.ts` and the 8-role test specification.
2. **Billing (`/settings/billing` and `/api/billing/*`)** is a placeholder disabled behind `FEATURE_FLAG_BILLING=false`.
3. **PostgreSQL Resource Backends** are disabled (`RESOURCE_POSTGRES_ENABLED=false`).
4. **SMTP Email Delivery** is not configured locally; member invitation links are returned once in the UI (`SecretReveal`) instead of emailed.
5. **No Gateway Key Update/Rotate Endpoint** exists in `apps/api/cmd/server/main.go` (only list, create, delete/revoke).
6. **No Workspace Member Management API/UI** exists despite `AuthorizationEngine.Can` enforcing `workspace_members` for non-`workspace:admin` roles.
7. **No User Profile / Notification Preferences Page** — `/settings` redirects directly to `/settings/organization`.
