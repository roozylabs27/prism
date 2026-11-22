# RoozyLabs Prism — Production Readiness Matrix & Release Gates

**Audited Baseline:** `main` @ `e85566b6c5d188a1143cc6d425cd05dc4140c365` (`e85566b`)  
**Audit Date:** 2026-11-21  
**Companion Documents:** [prism-comprehensive-audit.md](./prism-comprehensive-audit.md) | [prism-remediation-roadmap.md](./prism-remediation-roadmap.md)

---

## 1. Executive Production Verdict

> **Verdict: NOT READY FOR PUBLIC MULTI-TENANT PRODUCTION**
>
> Prism is currently suitable **only for single-tenant or trusted internal pilot use behind VPN/private network access**. Four P0 findings (`AUD-P0-01`–`AUD-P0-04`) and 17 P1 findings create verified cross-organization leakage, account takeover exposure, broken SDK/CLI authentication, inconsistent financial analytics, and a live `-race` concurrency defect.

---

## 2. Domain Production Readiness Matrix

| Domain | Current Status | Evidence | Remaining Gaps | Test Coverage | Production Readiness | Blocking Issues |
|---|---|---|---|---|---|---|
| **1. Authentication & Sessions** | **Partially Hardened** | `HttpOnly` cookie forwarding works via `/api/[[...path]]` (`route.ts:42-45`); SHA-256 session hashing (`session.go`) | JSON response still leaks `token` (`handlers/auth.go:268,324`); `?token=` accepted globally (`middleware/auth.go:34-36`); no CSRF origin check; `middleware.ts` catch redirect loop; no session rotation/revoke-all | Good unit coverage (`auth_test.go`, `auth_logout_test.go`); **no live E2E or CSRF/token-leak tests** | **Blocked (`P0`/`P1`)** | `AUD-P0-04`, `AUD-P1-01`, `AUD-P1-02`, `AUD-P2-01` |
| **2. OAuth & Account Linking** | **Vulnerable** | `oauth_state` CSRF cookie added (`auth_oauth.go:24-30`); unverified email rejected | Auto-links solely on email even over unverified `credential` signup (`service/auth.go:202-276`); `getBaseURL` trusts `X-Forwarded-Host` with missing compose env (`auth_oauth.go:73-97`) | `auth_oauth_test.go`, `auth_oauth_invite_test.go` mock providers; **no pre-account takeover or live OAuth E2E** | **Blocked (`P0`/`P1`)** | `AUD-P0-03`, `AUD-P1-03` |
| **3. Multi-Tenant & RLS Isolation** | **Incomplete / Bypassed** | Migration `111`, `rls_driver.go`, `tenant_scope.go` exist | Compose connects as superuser `postgres`; empty-GUC RLS fails open; `ModelRepository`, `MCPRegistry`, FinOps workers, `GetAgentStats`, `PilotOutcomes`, Google OAuth & active streams leak/collide cross-org; `TestRLS` SQL broken | Many Go unit/mock security tests; **0 working live PostgreSQL RLS verification tests** | **Blocked (`P0`/`P1`)** | `AUD-P0-01`, `AUD-P0-02`, `AUD-P1-06`, `AUD-P1-07`, `AUD-P1-09`, `AUD-P1-10`, `AUD-P1-11` |
| **4. Workspace & Project Isolation** | **Weak (Hierarchy Only)** | `workspaces` & `projects` exist with header extraction & hierarchy validation | `TenantMiddleware` omits `workspace_members`; single-item CRUD, `/v1/approvals/:id`, `/api/jobs/:jobId`, `BudgetManager` ignore workspace/project; 6 tenant routes lack `RequirePermission`; frontend retains Org A cache on switch | `workspace_test.go`, `tenant_hierarchy_test.go`; **no workspace-member RBAC or cache-purge tests** | **Blocked (`P1`)** | `AUD-P1-08`, `AUD-P1-13` |
| **5. Audit Trail & Tamper Integrity** | **Partially Working** | `AuditRecorder` signs with keyed HMAC (`audit_recorder.go:186-188`) | `MCPServerHandler.recordMCPAudit` omits `OrgID` before signing while `Create` sets `OrgID` on insert -> 100% of inbound MCP audit records fail `Verify`; missing `(org_id, created_at)` index on `ai_audit_trails` | `audit_recorder_test.go`, `audit_trail_test.go`; **no test for MCP audit verify roundtrip** | **Blocked (`P1`)** | `AUD-P1-12`, `AUD-P2-08` |
| **6. SSE & Real-Time Updates** | **Degraded under Load / Multi-Org** | Root `SSEContext` connects with `withCredentials: true` (`D-04` fixed) | User-only channel leaks across orgs for multi-org users & hides teammate traffic; 17+ backend event types trigger ~29 RQ invalidations per request; no `Last-Event-ID` replay | `sse_test.go`, `SSEContext.test.tsx`; **no cross-org SSE or storm-debounce tests** | **Blocked (`P1`)** | `AUD-P1-04`, `AUD-P1-05`, `AUD-P2-02` |
| **7. Dashboard & FinOps Analytics** | **Inconsistent Metrics** | Date selector wired for `7d`/`30d`/`90d` | `'all'` shows All Time KPIs, 30d chart, 90d analytics simultaneously; `INNER JOIN gateway_api_keys` drops deleted-key logs; `24h` chart buckets daily; `ProviderHealth.id` omitted (`json:"-"`); status ignores `credCount==0` | `logs_dashboard_test.go`, `pilot_outcomes_test.go`; **no multi-widget range or deleted-key tests** | **Blocked (`P1`)** | `AUD-P1-14`, `AUD-P1-15` |
| **8. Gateway Proxy, Budget & Quota** | **Core Works; Concurrency Race** | `SafeHTTPTransport` SSRF blocks, Redis Lua rate limits/budgets, graceful stream drain | **`go test -race` failed in `TestCachedQuotaEvaluator_ConcurrentMissesShareOneEvaluation`** (`got 4`); global Redis model telemetry & global `ProviderHealthStore` | Strong adapter & proxy unit coverage (`engine_*_test.go`, `load_chaos_test.go`); **1 active `-race` failure** | **Blocked (`P1`)** | `AUD-P1-17`, `AUD-P2-03` |
| **9. TypeScript SDK & CLI Integration** | **Broken for Gateway Keys** | `packages/sdk` & `packages/cli` build cleanly (`tsc`) | `RequirePermission` builds `PrincipalGatewayKey` which `authz.Can` denies (`403` on all 20 allowlisted `/api/*` routes + Paperclip register); `workspaces`/`analytics`/`mcp-registry` SDK calls missing from `GatewayKeyRoutes` | **`0` TypeScript SDK/CLI tests** (only `python-sdk` has `pytest`) | **Blocked (`P1`)** | `AUD-P1-16`, `AUD-P2-06`, `AUD-P2-10` |
| **10. Frontend Architecture & Forms** | **Strong Hygiene; Structural Debt** | 0 `: any`, RHF+Zod on 14/14 core forms, lint 0 errors, typecheck pass | `26/27` pages `'use client'`, `0` `loading.tsx`, 1,000+ line `sandbox` & overview pages, 12 mutations inside `hooks/queries/`, `ClientOnboardingModal` lacks RHF+Zod & rollback, `PermissionGuard` unused, mobile tenant selector hidden | `17` Vitest files (`59` tests) passing; **2 Playwright E2E skipped by default** | **Conditional (`P2`/`P3`)** *(once `AUD-P1-13` fixed)* | `AUD-P1-13`, `AUD-P2-04`, `AUD-P2-05`, `AUD-P3-02` |
| **11. CI/CD & Backup/Restore Ops** | **Blue-Green Verified; Drill Broken** | `test-blue-green.sh` passes `20/20`; Compose resource limits & `/ready` probes configured | `pg-restore-test.sh` & `rls_role_verification_test.go` fail on dropped `users`/`owner_id` schema; Compose omits `AUDIT_HMAC_KEY`/`REDIS_FAIL_CLOSED`/non-superuser role; Playwright E2E absent in CI | Go `-race` runs in CI against services; E2E not automated; local Docker offline during audit | **Blocked (`P0`/`P2`)** | `AUD-P0-01`, `AUD-P2-09`, `AUD-P2-11` |
| **12. Performance & Database Indexing** | **Good Pools; Index/Query Gaps** | `statement_timeout=30s`, Redis 500ms IO timeout, Go `GOMEMLIMIT=768MiB` | Triple `/api/user/permissions` SSR waterfall, 0/30 `lib/api/*` `AbortSignal` support, missing `(org_id, created_at)` index on `ai_audit_trails`, 33 unbounded `SELECT` queries | Load/chaos unit tests (`load_chaos_test.go`); **no live Postgres `EXPLAIN` or load test run locally** | **Conditional (`P2`)** | `AUD-P2-07`, `AUD-P2-08`, `AUD-P3-03` |

---

## 3. Required 9-Domain Security & Reliability Test Matrix

| Domain | Required Scenario | Present Today? | Exact Verification Target |
|---|---|---|---|
| **Authentication** | Login, signup, logout, expired session, no token in JSON, `?token=` rejected, CSRF origin block | **Partial** | `apps/api/internal/handlers/auth_test.go`, `auth_logout_test.go`, new CSRF & token-omission assertions |
| **OAuth** | Callback, invalid state, failed provider, unverified-email credential takeover prevention | **Partial** | `apps/api/internal/handlers/auth_oauth_test.go`, new `CreateOAuthSession` takeover & host-header regression tests |
| **SSR** | Authenticated/unauthenticated rendering, transient backend failure on `/signin` without redirect loop | **Partial** | `apps/app/middleware.test.ts` (add catch-block & org-cookie hydration specs) |
| **SSE** | Connect, disconnect, reconnect (`Last-Event-ID`), org channel isolation, burst debouncing | **Partial** | `apps/api/internal/handlers/sse_test.go`, `apps/app/context/SSEContext.test.tsx` |
| **Tenant** | Cross-tenant ID access, non-superuser PostgreSQL RLS, Model catalog isolation, MCP Registry, FinOps worker scoping | **Broken / Partial** | Fix `apps/api/internal/database/rls_role_verification_test.go` & `scripts/backup/pg-restore-test.sh`; expand `internal/security/` |
| **Workspace** | Member enforcement, switching, complete React Query cache clear on org/workspace change | **Missing (FE/RBAC)** | Add `TenantSelector.test.tsx` + backend `workspace_members` enforcement tests |
| **Dashboard** | Consistent `24h` (hourly), `7d`, `30d`, `90d`, `All Time`, custom ranges, and deleted-key `LEFT JOIN` persistence | **Partial** | Expand `apps/api/internal/handlers/logs_dashboard_test.go` & `repository/request_log_test.go` |
| **API Client & SDK** | `401`/`500` handling, timeout/abort, TS SDK + CLI gateway key compatibility across all allowlisted routes | **Missing (SDK/CLI)** | `apps/app/lib/api/client.test.ts`; create new Vitest suites in `packages/sdk` & `packages/cli` + Go gateway-key RBAC tests |
| **Forms** | Zod validation, duplicate submit lock, secret preservation on edit, onboarding partial-failure safety | **Partial** | Add Vitest specs for `ClientOnboardingModal`, `MCPServerFormDialog`, `AgentFormDialog`, `ResourceFormDialog` |

---

## 4. Pre-Production Release Gate Checklist

Before approving Prism for public multi-tenant GA, every gate below must pass **post-remediation** against live PostgreSQL 15 & Redis 7 containers:

- [ ] **Gate 1 (M1 Non-Superuser RLS + Restore):** `go test -v ./internal/database -run TestRLS` passes against PostgreSQL 15 under `NOSUPERUSER NOBYPASSRLS` role AND `bash scripts/backup/pg-restore-test.sh` passes exit `0`.
- [ ] **Gate 2 (M1 Auth/Tenant/Isolation Suite):** `go test -race -count=1 ./...` passes with **0 failures** (including fixed `TestCachedQuotaEvaluator_ConcurrentMissesShareOneEvaluation`) against live PostgreSQL & Redis.
- [ ] **Gate 3 (M1 OAuth & Cookie Security):** `/api/auth/login` and `/signup` never emit bearer tokens in JSON for browser logins; OAuth pre-account takeover blocked; CSRF origin check enforced on cookie mutations & logout.
- [ ] **Gate 4 (M2 SDK/CLI Integration):** `@roozylabs/prism` SDK & CLI test suites pass against live Go API using `prism_sk_*` gateway keys across all allowlisted routes.
- [ ] **Gate 5 (M2 Dashboard & Audit Integrity):** All-time vs usage chart vs analytics match; deleted-key logs retained; inbound MCP `POST /v1/mcp` audit trails pass `POST /api/audit-trail/:id/verify`.
- [ ] **Gate 6 (M2/M3 Live Browser E2E):** `PLAYWRIGHT_LIVE_E2E=1 pnpm --filter prism-dashboard test:e2e` passes full login, org/workspace switch (verifying stale cache removal), SSE live updates, approvals, sandbox, and settings flows.
