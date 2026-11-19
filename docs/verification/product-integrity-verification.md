# Prism — Product Integrity, Security & Verification Evidence Report

**Repository:** `github.com/roozylabs27/prism` (`main` @ `0b7ea87` + Phase 4 security/control-plane hardening + Commercial Validation Recommendations 1–3 APIs + live Docker Postgres/Redis E2E verification)  
**Scope:** Backend Go API (`apps/api`), Web Dashboard (`apps/app`), live Docker PostgreSQL (`127.0.0.1:5434`) + Redis (`127.0.0.1:6379`) integration suites, and E2E control-plane boundary verification.

---

## 1. Verification Evidence Classification Legend

Every finding in Prism's assessment and this report uses strict epistemological tagging:

| Evidence Tier | Definition |
|---|---|
| **Confirmed by code inspection** | Directly observed in current repository source files, migrations, Docker/CI configs, or `package.json`. |
| **Confirmed by automated tests** | Directly verified by executed `go vet`, `go test` / `go test -race`, `tsc --noEmit && next build`, or Vitest runs after code edits. |
| **Confirmed through integration/E2E tests** | Directly verified end-to-end against real ephemeral migrated PostgreSQL (`127.0.0.1:5434`) + live Redis (`127.0.0.1:6379`) + HTTP middleware + auth + tenant pinning + Smart Router/MCP/upstream servers. |
| **Verified in a real production environment** | Live hosted multi-tenant cluster verification. **None performed in this assessment.** |
| **Not yet verified** | Live container smoke curls after rebuilding images from uncommitted code, Playwright browser E2E specs (none exist), and real external vendor/agency pilot runs. |

---

## 2. Commands Executed & Exact Observed Results

*No Go or TypeScript code files were modified after the latest backend Go verification runs (`task-1425` + `task-1449`); only markdown documentation under `docs/` was updated afterwards.*

| Component / Check | Exact Command | Result | Exact Observed Scope, Timing & Skips / Failures |
|---|---|---|---|
| **Docker Containers Status** | `docker ps -a --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"` | **Running & Healthy** | `prism-postgres-database` (`127.0.0.1:5434->5432`, healthy), `prism-redis-cache` (`127.0.0.1:6379->6379`, healthy), `prism-api` (`127.0.0.1:8080`, running pre-edit image), `prism-app` (`127.0.0.1:3000`). |
| **Go Static Check & Full Suite (Live Postgres + Redis)** | `go vet ./...; go test -count=1 ./...` (`DATABASE_URL` @ `:5434`, `REDIS_URL` @ `:6379`) | **16 of 17 Packages PASS; 1 Pre-Existing DB Migration Pkg FAIL** | **PASS (15 in `task-1425` + `internal/redis` fixed & re-verified in `task-1449`):** `cmd/server` (1.929s), `config` (2.484s), `featureflags` (1.114s), `handlers` (190.468s), `httputil` (2.151s), `mail` (1.387s), `middleware` (1.648s), `proxy` (223.881s), `queue` (6.199s), `redis` (`-race` 6.306s, 13/13 tests pass), `repository` (555.685s), `security` (3.834s), `service` (0.367s), `telemetry` (0.808s), `utils` (3.967s), `workers` (24.660s); 5 packages `[no test files]`. **Pre-existing FAIL (`internal/database`, 24.439s):** `TestMigrationsFromEmptyDatabase` fails on `m.Down()` at `092_detach_system_providers_from_admin.down.sql` (`providers_user_id_fkey` `23503` because `108_remove_seeded_admin.down.sql` intentionally returns `SELECT 1;`); `TestDirtyConcurrentIndexIsRebuilt` fails on `pq: relation "t" does not exist` (`file://` Windows TempDir handling in `migrate.go:15`). |
| **Targeted Live Postgres + Redis E2E & SQL Suites** | `go test -v -count=1 ./internal/handlers -run "TestControlPlane_PostgresRedis_FullStackE2EJourneys\|TestCommercialValidation_WorkspacesKeysAndPilotOutcomes_Postgres\|TestModelHandlerCRUDAndDeleteGuards\|TestGatewayKeyHandlerIssuesListsAndRevokesKeys"` & `./internal/repository -run "TestAIAuditTrailsFilterAndScope"` | **PASS (Exit 0)** | **PASS (`task-1421` + `task-1449`):** `TestCommercialValidation_WorkspacesKeysAndPilotOutcomes_Postgres` (23.28s), `TestControlPlane_PostgresRedis_FullStackE2EJourneys` (9.48s after `events.go` update), `TestControlPlane_ZeroExternalDeps_E2EJourneys` (0.01s), `TestGatewayKeyHandlerIssuesListsAndRevokesKeys` (41.35s), `TestModelHandlerCRUDAndDeleteGuards` (7.68s), `TestAIAuditTrailsFilterAndScope` (6.42s). |
| **Go Race Detector (Critical Runtime & Redis Packages)** | `go test -race -count=1 ./internal/proxy/... ./internal/middleware/... ./internal/handlers/... ./internal/service/...` + `-race ./internal/redis/...` | **PASS (Exit 0)** | Zero data races across `proxy` (8.672s), `middleware` (7.624s), `handlers` (8.637s), `service` (1.553s), and `redis` (6.306s); plus pre-live handlers/repo/server `-race` (`handlers` 9.171s, `repository` 5.612s, `cmd/server` 2.892s). |
| **Pilot Onboarding CLI Script Syntax** | `bash -n scripts/pilot/onboard-client-workspace.sh` | **PASS (`BASH_SYNTAX_OK=True`)** | Static POSIX/bash syntax verified (`task-1221`); live HTTP execution against running staging container **Not yet verified**. |
| **Frontend Typecheck & Production Build** | `pnpm --filter prism-dashboard build` (`tsc --noEmit && next build`) | **PASS (Exit 0)** | Next.js 16.3.3 Turbopack compiled in 19.2s, TypeScript checked clean, 6/6 static pages generated (no `apps/app/` edits performed). |
| **Frontend Unit Tests (Vitest)** | `pnpm --filter prism-dashboard test -- --testTimeout=15000` | **FAIL (Exit 1)** | **9 passed / 35 tests passed; 1 failed (`lib/api/client.test.ts`).** Failure cause: `TypeError: Cannot read properties of undefined (reading 'getItem')` at `stores/useTenantStore.ts:15` because `useTenantStore.ts` checks `typeof window !== 'undefined'` without checking `typeof localStorage !== 'undefined'` under local Node's `--localstorage-file` quirk. |
| **Frontend Lint** | `pnpm --filter prism-dashboard lint` (`eslint .`) | **FAIL (Exit 1)** | `eslint: command not found` (`spawn ENOENT`); `eslint` is absent from `apps/app/package.json` `devDependencies`. |

---

## 3. User Journey Verification Matrix

| # | Core End-to-End User Journey | Current Verification Status | Verification Tier | Notes, Skips & Exact Gaps |
|---|---|---|---|---|
| **1** | **Auth, Session Security & Backend RBAC** (`routes_rbac_test.go`, `auth_test.go`) | **Verified in unit/handler & live DB/Redis runs** | Confirmed by automated tests & code inspection | Static AST RBAC test guarantees admin/write `/api/*` routes (including 6 new Workspace & Pilot Outcomes routes) enforce permissions; `AuthHandler.SetFailClosed` rejects Redis outage with `503`. Frontend `PermissionProvider` defaults owner fallback & `/api/user/permissions` lacks `TenantMiddleware`. |
| **2** | **Gateway Key `OrgID`/`WorkspaceID`/`ProjectID` Pinning & Anti-Spoofing** | **Verified end-to-end against live Postgres + Redis** | Confirmed through integration/E2E tests | Both `TestControlPlane_ZeroExternalDeps_E2EJourneys` and `TestControlPlane_PostgresRedis_FullStackE2EJourneys` verify spoofed `X-Prism-Workspace-ID` fails with `403 TENANT_SECURITY_ERROR`; `TestCommercialValidation_WorkspacesKeysAndPilotOutcomes_Postgres` verifies `NULL` unscoped FKs + pinned workspace/project FK constraints on Postgres. |
| **3** | **Provider / BYOK Model Ownership & MCP Secret Masking** | **Verified against live Postgres (`ModelHandler`) + unit tests (`MCPHandler`)** | Confirmed through integration/E2E tests | `TestModelHandlerCRUDAndDeleteGuards` (7.68s) & `TestModelHandler_CrossTenantAndSystemProviderGuards` pass against live Postgres; `TestMCPHandler_GetRedactsSensitiveHeadersAndUpdatePreservesThem` confirms header secret redaction & preservation. |
| **4** | **Inference Gateway (`/v1/chat/completions`, Streaming, Failover, Smart Router `X-Routing-Policy`, Agent Model Filter)** | **Verified end-to-end against live Postgres + Redis + in-process HTTP servers** | Confirmed through integration/E2E tests | `TestControlPlane_PostgresRedis_FullStackE2EJourneys` (9.48s) verifies BYOK isolation, `prism-auto` + `X-Routing-Policy: cost-saver` + agent allowlist enforcement, SSE chunking + `[DONE]`, primary `500` failover (`X-Prism-Failovers: 1`), and real DB request log + analytics persistence. Live Postgres run also exposed that `ResolveWithFallback` requires a `RoutingRule` (or bound provider key) when overriding seeded system model slugs (`gpt-4o-mini` in `082`). |
| **5** | **Concurrent Budget Reservation, Agent `MaxBudgetCents`, Workspace/Project Quotas & Daily Request Limits** | **Verified under `-race` + live Postgres/Redis hard-cap E2E** | Confirmed through integration/E2E & `-race` tests | `TestBudgetManager_ConcurrentHardLimitReservation` (20 concurrent goroutines) passes `-race`; live E2E verifies `DailyLimit` `429` denial against real Postgres `budgets` & `request_logs`. Cross-replica Redis Lua budget reservation remains P1 open. |
| **6** | **Agent Execution Boundaries on Tool (`/v1/tools/*`), Resource (`/v1/resources/*`), and Inbound MCP (`/v1/mcp`)** | **Verified against live Postgres + Redis + `mark3labs/mcp-go` test server** | Confirmed through integration/E2E tests | Both zero-dep and Postgres/Redis E2E prove `crm__get_lead` executes upstream while `!drop_records` denies with `-32001` before reaching upstream. |
| **7** | **Delegated AI Task (`delegate_ai_task` over `POST /v1/mcp`) with Disclosure & Hard Cap** | **Verified end-to-end against live Postgres + Redis + Smart Router** | Confirmed through integration/E2E tests | Live E2E invokes `delegate_ai_task`, routes via `cost-saver` Smart Router to `gpt-4o-mini`, returns `answer from gpt-4o-mini` + `disclosure.executed_model` (`X-Prism-Model`), and persists audit trails. Non-fatal log `persist request payload: pq: invalid input syntax for type uuid: ""` noted when `c == nil` in `engine.go:276`. |
| **8** | **Redis Outage Fail-Closed Controls (`REDIS_FAIL_CLOSED`)** | **Verified in unit/E2E & live Redis (`internal/redis` `-race`)** | Confirmed by automated tests & live Redis | `503 RATE_LIMITER_UNAVAILABLE`, `503 idempotency_unavailable`, `503 AUTH_THROTTLE_UNAVAILABLE` tested; all 13 Redis tests pass under `-race`. |
| **9** | **Tenant Audit Trail Scoping & Cross-Tenant Isolation** | **Verified against live PostgreSQL** | Confirmed through integration/E2E tests | Both `TestAIAuditTrailsFilterAndScope` (6.42s) and `TestControlPlane_PostgresRedis_FullStackE2EJourneys` prove Alice retrieves compliant + denied audit trails while Bob receives zero rows (`bobTotal == 0`). |
| **10** | **Claude Code (`ANTHROPIC_BASE_URL` -> `/v1/messages`) & Approval Workflows** | **Absent / Unsupported** | Confirmed by code inspection | `POST /v1/messages` does not exist in `cmd/server/main.go`; human approval workflow does not exist (`0%`). **Not yet verified** with real Claude Code / Claude Desktop clients. |
| **11** | **Frontend Dashboard Flows & Browser E2E** | **Next.js build passes; Vitest 9/10 pass (`client.test.ts` fails); lint fails; zero Playwright specs** | Confirmed by code inspection & automated tests | Known UI workflow breaks (`Playground`, `Sandbox`, `ResourceFormDialog`, `AgentFormDialog`, missing Workspace UI switcher) remain open for P1. |
| **12** | **Commercial Validation Backend APIs (`Workspaces`, `Projects`, Pinned `GatewayKeys`, `PilotOutcomes` Report & Safe CSV Export)** | **Verified end-to-end against live PostgreSQL** | Confirmed through integration/E2E tests | `TestCommercialValidation_WorkspacesKeysAndPilotOutcomes_Postgres` (23.28s) proves org isolation, max workspace limits (`409`), project creation, `NULL` unscoped FKs + pinned key FKs, cross-workspace rejection (`400`), real SQL p95 latency & savings math, `403`/`429` governance counts, workspace filtering, and CSV export. |

---

## 4. Security Verification Summary & Unverified Production Assumptions

### Verified Security Guarantees (Within Enforced Boundaries)
- **Spoofed workspace/project/org headers** on scoped gateway keys are rejected deterministically with `403 TENANT_SECURITY_ERROR` (`TestControlPlane_PostgresRedis_FullStackE2EJourneys`).
- **System provider mutation & foreign tenant provider/model edits** are rejected with `403` and `404` against live Postgres (`TestModelHandlerCRUDAndDeleteGuards`, `TestModelHandler_CrossTenantAndSystemProviderGuards`).
- **Sensitive upstream MCP headers** (`Authorization`, `X-API-Key`) are masked on `GET /api/mcp/servers/:id` and preserved cleanly on `PUT` (`TestMCPHandler_GetRedactsSensitiveHeadersAndUpdatePreservesThem`).
- **Plan tier query parameter spoofing** (`GET /api/features?plan_tier=enterprise`) cannot escalate tiers (`TestFeaturesHandler_IgnoresPlanTierQuerySpoofing`).
- **Local stdio MCP server creation** stays blocked (`403 LOCAL_MCP_DISABLED`) unless explicitly enabled (`TestMCPHandler_CreateLocalServerForbiddenWhenDisabled`).
- **Redis failure modes & nil EventPublisher safety:** Rate limiter, idempotency, and auth fail closed (`503`) when `REDIS_FAIL_CLOSED=true`; nil `EventPublisher` no longer panics in `GatewayHandler.handleProxyError`, while `ErrNoEventOwner` stays enforced.
- **CSV export formula neutralization:** Admin audit & pilot outcomes CSV cells starting with `=`, `+`, `-`, `@`, `\t`, `\r` are escaped with `'`.

### Explicit Unverified Assumptions & Newly Confirmed Blockers
1. **Pre-Existing `internal/database` Migration Defects Confirmed Live:** `TestMigrationsFromEmptyDatabase` fails on rollback at `092_detach_system_providers_from_admin.down.sql` (`providers_user_id_fkey` violated after `108_remove_seeded_admin.down.sql`), and `TestDirtyConcurrentIndexIsRebuilt` fails on Windows path URI formatting. **Do not rely on `migrate Down` rollback scripts in production; use forward-fix migrations only (`P2-1`).**
2. **Remaining Open P0 Items Blocking Unattended Multi-Tenant Production:**
   - `GatewayAPIKey.AgentID` binding (`P0-5`) + exact RBAC matching + `X-Paperclip-Agent-ID` middleware resolution.
   - Systemic `org_id`/`workspace_id` repository scoping + active Postgres RLS (`P0-6`).
   - Durable outbox/DLQ for financial/audit writes & true payload hashes (`P0-7`), plus minor UUID empty-string fix in `engine.go:276` when `requestID == ""`.
3. **No Live Multi-Tenant Production Verification & Unbuilt Live Docker Images:** Running `prism-api` (`:8080`) / `prism-app` (`:3000`) containers use pre-edit images (all live Postgres+Redis E2E tests ran directly against Go binaries compiled from current source). Rebuild compose images before running manual container curls or staging pilots.
4. **No Browser E2E Coverage:** Zero Playwright specs exist; `eslint` is missing and `apps/app/stores/useTenantStore.ts` fails Vitest under Node due to unguarded `localStorage.getItem`.
