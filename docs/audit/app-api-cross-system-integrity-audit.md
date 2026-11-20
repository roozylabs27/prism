# Prism — Cross-System App & API Product Integrity Audit

**Audit Date:** 2026-11-20  
**Repository:** `c:\me\roozylabs\projects\prism` (`https://github.com/roozylabs/prism`)  
**Audited Baseline:** `main` @ `e31a1af` (clean working tree; 5 local `apps/web` landing-page commits ahead of `origin/main`; only unmerged remote branch: `origin/dependabot/github_actions/github-actions-c8a14cf755`)  
**Scope:** `apps/app` (Next.js 16 Admin Console), `apps/api` (Go Gateway & Control Plane), shared contracts (`packages/sdk`, `packages/cli`), PostgreSQL migrations (`001`–`109`), Redis interactions, and CI/observability configuration. (`apps/web` landing page excluded per task scope.)  
**Audit Mode:** Read-only evidence-based discovery & verification. Zero application code modifications performed.

---

## 1. Executive Summary & Repository Discovery

Prism's Go backend has undergone meaningful security and reliability hardening in prior batches (`docs/Prism Reliability Audit.md` Batches 1–16 and Phase 4 fixes P0-1–P0-4, P1-1), including gateway key tenant pinning (`OrgID`/`WorkspaceID`/`ProjectID`), fail-closed Redis controls in production (`REDIS_FAIL_CLOSED`), an inbound governed JSON-RPC 2.0 MCP endpoint (`POST /v1/mcp`), SHA-256 hashed session and invite tokens (migrations `107`, `109`), removal of seeded default admin credentials (migration `108`), clean `rows.Err()` discipline across all 40 Go repositories, and 0 race conditions across 7 unit-tested backend packages.

However, **cross-system integration between `apps/app` and `apps/api` currently exhibits severe P0 contract breaks that cause live console workflows to fail or silently corrupt stored secrets**, alongside unresolved systemic `user_id`-vs-`org_id` tenant scoping mismatches, dormant PostgreSQL Row-Level Security (RLS), Next.js reverse-proxy path-traversal/auth-bypass risks, and missing CI lint verification.

### Repository Discovery & Graph Coverage Disclosure
- **GitHub PR Metadata:** `gh` CLI is not installed in the local audit environment, so live open GitHub PR statuses could not be queried remotely; remote branch state was inspected directly via `git`.
- **Migrations Inventory (`apps/api/migrations/`):** `109` `.up.sql` files paired 1:1 with `109` `.down.sql` files (`0` orphans, `0` duplicate numeric prefixes). Recent hardening migrations include `105` (`budget_alerts.created_at` index), `106` (`routing:read` / `routing:update` RBAC permissions), `107` (invite audit columns + unique pending invite index), `108` (removal of hardcoded seeded admin account), and `109` (SHA-256 hashing of `session.token`).
- **`codebase-memory-mcp` Coverage Disclosure:** Project `C-me-roozylabs-projects-prism` is indexed (`7,873` nodes, `29,772` edges, `indexed_at: 2026-11-20T09:07:57Z`). `check_index_coverage` reported `no_recorded_issue` on `47/49` checked paths and flagged excluded subtrees (`apps/app/app/(dashboard)/tools`, `apps/app/features/tools`, `apps/app/**/*.test.ts(x)`, `apps/api/migrations`, `apps/api/docs`, plus `parse_partial` on `apps/api/Dockerfile:26-40`). Every excluded path referenced in this report was read directly from source.

### Severity Snapshot
| Category | P0 | P1 | P2 | Notes |
|---|---|---|---|---|
| **Confirmed Critical Defects (`D-01`–`D-15`)** | 10 (`D-01`–`D-10`) | 5 (`D-11`–`D-15`) | — | Live workflow breaks, secret corruption, tenant/proxy gaps |
| **Product Workflow & Dead-Code Gaps** | — | 7 missing UI workflows | 4 incomplete UI + 28 unused files + 1 backend stub | Backend capabilities unexposed or miswired in console/SDK |

---

## 2. Confirmed Working Capabilities (Verified in Source)

Do not re-open these already-verified controls unless extending them:
1. **PostgreSQL Row Error Hygiene (`sqlrowserr`):** Automated inspection across all 40 non-test files in `apps/api/internal/repository/*.go` confirmed `rows.Next()` count matches `rows.Err()` count (`0` mismatched files).
2. **Gateway Key Tenant Pinning & Route Restriction:** `ResolveCanonicalTenantContext` (`apps/api/internal/middleware/tenant.go:38-141`) prevents gateway-key callers from spoofing `X-Prism-Org-ID`, `X-Prism-Workspace-ID`, or `X-Prism-Project-ID`. `RestrictGatewayKeys` (`apps/api/internal/middleware/gateway_key_scope.go:13-48`) confines gateway API keys to an explicit 20-route CLI/SDK allowlist.
3. **Runtime SSRF Protections & High-Risk Backend Guardrails:**
   - Outbound calls in LLM proxy (`apps/api/internal/proxy/engine.go:110`), MCP (`apps/api/internal/proxy/mcp_gateway.go:116-125`), Tool (`apps/api/internal/proxy/tool_gateway.go:74-97`), and Resource gateways (`apps/api/internal/proxy/resource_gateway.go:109,143,197`) route through `security.ValidateOutboundURL` / `ValidatePostgresDSN` and `security.SafeHTTPTransport` dialer checks.
   - Local stdio MCP (`MCPLocalStdioEnabled`) and Postgres resource backends (`ResourcePostgresEnabled`) default to `false` (`apps/api/internal/config/config.go:145,165`).
4. **Startup Config Validation & Secret Handling:** `Config.Validate()` (`apps/api/internal/config/config.go:202-221`) rejects missing/weak `ENCRYPTION_KEY`, default DB/Redis passwords, wildcard/localhost CORS origins, and missing Turnstile config outside `APP_ENV=development`. `GatewayAPIKey.KeyHash` carries `json:"-"` (`apps/api/internal/models/gateway.go:22`). Session and invite tokens are stored SHA-256 hashed (`migrations/107`, `109`).
5. **Inbound Governed MCP Gateway & Async Job Queue:** `POST /v1/mcp` (`apps/api/cmd/server/main.go:590`) supports JSON-RPC 2.0 `initialize`, `tools/list`, `tools/call`, and `delegate_ai_task`. `queue.JobQueue` (`apps/api/internal/queue/job_queue.go:220-377`) implements atomic Redis `BLMove`, worker leases, heartbeats, and crashed-worker reaping.
6. **Backend Workspaces/Projects & Pilot Outcomes APIs:** `handlers/workspace.go` (`GET/POST /api/workspaces`, `GET/POST /api/workspaces/:workspaceId/projects`) and `handlers/pilot_outcomes.go` (`GET /api/analytics/pilot-outcomes`, `/export` CSV/JSON) exist and enforce `TenantMiddleware` + permissions (`main.go:388-391,434-435`).
7. **Logs Server-Side Pagination:** `apps/app/app/(dashboard)/logs/page.tsx` now paginates via backend `PAGE_SIZE = 25` queries (previous 10-row fetch bug resolved).
8. **TypeScript Build & SDK/CLI Parity on Core Routes:** `apps/app` passes `tsc --noEmit` and `next build` cleanly; `packages/sdk` and `packages/cli` build cleanly, and all 20 `/api/*` SDK endpoints match `GatewayKeyRoutes` (`middleware/gateway_key_scope.go:13-34`).

---

## 3. Confirmed Defects (P0 & P1 — Evidence-Backed)

### P0 — Broken Core Workflows, Secret Corruption & Critical Security Flaws

#### D-01 (P0): Credential Delete & Reset Cooldown 100% Broken (Swapped Parameters)
- **Evidence:** `apps/app/hooks/mutations/useCredentialMutations.ts:20-21,44-45` vs `apps/app/lib/api/credentials.ts:24-34` vs `apps/api/internal/handlers/credential.go:396,457`.
- **Mechanism:** `useResetCooldownMutation` and `useDeleteCredentialMutation` pass `(d.providerId, d.credId)` into `apiResetCredentialCooldown(id, providerId)` and `apiDeleteCredential(id, providerId)`. Because the parameters are reversed, the frontend sends `DELETE /api/providers/<credId>/credentials/<providerId>` and `POST /api/providers/<credId>/credentials/<providerId>/reset-cooldown`. Go reads `credID := c.Param("credId")` (`credential.go:396,457`), receives the provider ID instead, and always returns **404 `CREDENTIAL_NOT_FOUND`**.
- **Impact:** Deleting a credential or resetting its cooldown from the dashboard fails 100% of the time.

#### D-02 (P0): Custom Provider Creation End-to-End Failure Chain
- **Evidence:** `apps/app/app/(dashboard)/providers/_components/ProviderFormDialog.tsx:29-33,54-56`, `apps/api/internal/handlers/provider.go:88-104`, `apps/api/internal/handlers/credential.go:137-142`, `apps/api/migrations/002_create_providers.up.sql:5`.
- **Mechanism:**
  1. `ProviderFormDialog` submits only `{ name, type, baseUrl }` (no `enabled`, no `slug`), and calls `createMutation.mutate(values)` (`:55`) without `onSuccess` (to close dialog/show toast) or `onError`.
  2. Go `ProviderHandler.Create` (`provider.go:89-103`) binds `models.Provider` directly, inserting `enabled = false` and `slug = ""`.
  3. Because `providers/page.tsx` has no Edit/Enable provider control, the new provider stays permanently disabled; attempting to attach a credential fails `403 PROVIDER_ACCESS_DENIED` (`credential.go:138,153-154`).
  4. Creating any second custom provider collides on the unique constraint over `providers.slug = ""` (`migrations/002`), returning `409 Conflict` with no error feedback in the UI.

#### D-03 (P0): Unauthenticated Invitees Ejected From `/accept-invite` + Email-Mismatch Loop & Ordering Bugs
- **Evidence:** `apps/app/app/layout.tsx:36`, `apps/app/context/AuthContext.tsx:31-40`, `apps/app/lib/api/client.ts:44-51`, `apps/app/app/(auth)/accept-invite/page.tsx:89-98`, `apps/app/middleware.ts:67-77`, `apps/app/app/(auth)/signin/page.tsx:48-52,121-124,264`, `apps/app/hooks/mutations/useMemberMutations.ts:46-51`.
- **Mechanism:**
  1. Global `<AuthProvider>` (`app/layout.tsx:36`) immediately invokes `apiGetMe()` (`AuthContext.tsx:37`). For an unauthenticated invitee opening `/accept-invite?token=...`, the `401` interceptor in `client.ts:49-51` checks `!['/signin', '/signup'].includes(window.location.pathname)` and redirects to `/signin`, dropping them out of `/accept-invite`.
  2. When a signed-in user visits `/accept-invite` with a mismatched email, `accept-invite/page.tsx:95-97` renders a link to `/signin?invite=...` with no Sign Out button; `middleware.ts:73-77` and `signin/page.tsx:48-52` immediately bounce signed-in users back to `/accept-invite?token=...`, creating an infinite loop.
  3. Signed-in un-onboarded invitees hit `if (!isOnboarded)` (`middleware.ts:67`) before `if (isAuthPage && inviteToken)` (`middleware.ts:73`), while `signin/page.tsx:264` ("Sign up") and OAuth buttons (`:121-123`) omit `?invite=`.
  4. `useAcceptInviteMutation` (`useMemberMutations.ts:46-51`) invalidates `['user', 'organizations']` instead of `['user-organizations']` and omits `['user-permissions']`.

#### D-04 (P0): Real-Time SSE Dead for HttpOnly Sessions -> Permanent Async Sandbox Hang + Logout Deadlock
- **Evidence:** `apps/api/internal/handlers/auth.go:252,308,321`, `apps/api/internal/handlers/auth_oauth.go:232,404`, `apps/api/cmd/server/main.go:354-358`, `apps/app/context/AuthContext.tsx:45-58,72-83`, `apps/app/context/SSEContext.tsx:41-56,126-149`, `apps/app/app/(dashboard)/sandbox/page.tsx:451-491`.
- **Mechanism:**
  1. `Login`, `Signup`, and `OAuthCallback` set `auth_token` with `HttpOnly = true`. For OAuth logins (`auth_oauth.go:232-233`), **no JS-readable cookie or token exists**; even on email/password login, `AuthContext.tsx` writes a redundant non-HttpOnly cookie only in `onSuccess` while `SSEContext.tsx` mounts once at root (`useEffect(..., [queryClient])`) and gates connection on `Cookies.get('auth_token')` (`SSEContext.tsx:42,52,126`). Thus SSE never connects for OAuth users or post-login SPA navigation (even though Next's `/api/[[...path]]` proxy at `route.ts:35-38` already converts the `HttpOnly` cookie into `Authorization: Bearer ...` automatically).
  2. **Consequence:** Async Sandbox (`sandbox/page.tsx:453-490`) sets `activeJobId` and waits exclusively for SSE `async_job_updated` events with **zero polling fallback** to `GET /api/jobs/:jobId`, hanging indefinitely after `202 Accepted`.
  3. **Logout Deadlock:** `POST /api/auth/logout` sits inside `userProtected` behind `AuthMiddleware` (`main.go:354,358`). If a session expires or is revoked in PostgreSQL, `AuthMiddleware` returns `401` before `Logout` can expire the `HttpOnly` cookie, and client JS (`client.ts:45`, `AuthContext.tsx:78-79`) cannot clear `HttpOnly` cookies — potentially trapping `middleware.ts:51` on an expired cookie. (Additionally, `AuthContext.tsx:46,57` unnecessarily weakens session confidentiality for password logins by mirroring the `HttpOnly` token into a JS-readable cookie.)

#### D-05 (P0): MCP Toggle Switch Always Fails `400` AND Edit Overwrites Saved Bearer Token With Bullets
- **Evidence:** `apps/app/app/(dashboard)/mcp/page.tsx:220-221`, `apps/app/app/(dashboard)/mcp/_components/MCPServerFormDialog.tsx:125-139,167-198`, `apps/api/internal/handlers/mcp.go:48,136,150,198,209-214,369-373`.
- **Mechanism:**
  1. Switching the Enabled toggle on `/mcp` calls `PUT /api/mcp/servers/:id` with `{ enabled: checked }` (`mcp/page.tsx:221`). `MCPHandler.Update` (`mcp.go:369-373`) binds JSON into `CreateMCPServerRequest` (`Name string binding:"required"` at `:48`), always returning **`400 INVALID_REQUEST_BODY`**.
  2. On Edit, `MCPHandler.Create/Update` stores `req.AuthToken` inside `HeadersEncrypted["Authorization"]` (leaving `AuthTokenEncrypted = nil`, so `hasAuthToken` at `:150` is always `false`). `toEdit` (`:136`) masks it as `"Bearer fc-b••••1234"`. `MCPServerFormDialog` strips `"Bearer "` into `authToken` (`:130-135`) and submits `authToken: "fc-b••••1234"` (`:192`). `buildMergedConfigHeaders` only checks `"••••"` inside `req.Headers` (`:198`) and **never checks `req.AuthToken`** (`:209-214`), **silently overwriting the encrypted upstream Bearer token with `"Bearer fc-b••••1234"` on every save!**

#### D-06 (P0): Tool Update Silently Wipes All Backend Auth Tokens (+ Missing Save-Time SSRF Validation)
- **Evidence:** `apps/app/app/(dashboard)/tools/_components/ToolFormDialog.tsx:102-108,133-141`, `apps/api/internal/handlers/tool.go:68-177,218-242`.
- **Mechanism:** `ToolFormDialog` initializes every backend's `authToken: ""` on edit (`:105`) and submits `backends` on `PUT /api/tools/:id`. `ToolHandler.Update` (`tool.go:162-173`) calls `h.backends.DeleteByToolID` and inserts fresh `ToolBackend` rows, **erasing every existing `AuthTokenEncrypted` unless re-entered on every edit**. Additionally, unlike `ResourceHandler` (`resource.go:59`) and `MCPRegistryHandler` (`mcp_registry.go:83`), `ToolHandler.Create/Update` never calls `security.ValidateOutboundURL` before persisting `EndpointURL` (though runtime execution in `proxy/tool_gateway.go:74,97` blocks private IPs at dial time).

#### D-07 (P0): Agent Edit Silently Wipes `systemPromptOverride` & Backend Cannot Clear Allowlists/Budget
- **Evidence:** `apps/app/app/(dashboard)/agents/_components/AgentFormDialog.tsx:118-128`, `apps/api/internal/handlers/agent.go:169-221`.
- **Mechanism:** `AgentFormDialog` omits `systemPromptOverride`, `allowedModels`, and `enabled` from its form state and `Update` payload (`:119-128`). `AgentHandler.Update` unconditionally executes `existing.SystemPromptOverride = req.SystemPromptOverride` (`agent.go:174`), wiping custom system prompts to `""` on every UI edit. Conversely, `agent.go:180-217` guards `AllowedModels`, `AllowedTools`, `AllowedResources`, `AllowedMCPServers` with `if len(...) > 0` and `MaxBudgetCents` with `if maxBudgetCents > 0`, making it impossible via UI or API to empty an allowlist or reset budget caps to `0`.

#### D-08 (P0): Routing Policy Weight Scale Mismatch (`0–100` UI vs `0.0–1.0` Go Scorer)
- **Evidence:** `apps/app/app/(dashboard)/policies/_components/PolicyFormDialog.tsx:32-37,62-66`, `apps/app/app/(dashboard)/policies/_components/PolicyTuner.tsx:39-43,66-69,128,154,178`, `apps/api/internal/proxy/scorer.go:47-52,145`, `apps/api/internal/proxy/router.go:535`, `apps/api/internal/handlers/simulate.go:191`.
- **Mechanism:** UI sliders and schemas operate on `0–100` percentages (`quality: 40, cost: 30, speed: 20`, omitting `task_match`). `scorer.go:145` sums `taskScore*taskWeight + qualityScore*qualityWeight + costScore*costWeight + speedScore*speedWeight` without weight normalization, and `router.go:535` / `simulate.go:191` check `sc.Score < 0.5`. Seeded policies (`0.35`) render as `0.35%` in the UI, whereas UI-saved policies produce scores of `70.0–90.0` (`7000%`), completely bypassing budget downgrade checks.

#### D-09 (P0): Next.js Reverse Proxy Path Traversal, Missing Abort Signal & Middleware Auth Bypass
- **Evidence:** `apps/app/app/api/[[...path]]/route.ts:18-26,50-56,80-108`, `apps/app/app/v1/[[...path]]/route.ts:5-17,31-64`, `apps/app/middleware.ts:36-39`.
- **Mechanism:**
  1. Both catch-all proxies join `pathArray.join('/')` into `${backendApiUrl}/api/${subPath}` and `${backendApiUrl}/v1/${subPath}` without rejecting `..`, `%2e%2e`, or control characters, allowing path traversal out of `/api/` or `/v1/` to internal backend paths (`/metrics`, `/ready`, `/swagger/*`).
  2. Neither proxy forwards `signal: request.signal` to `fetch()`, leaking backend goroutines/upstream streams when browsers disconnect.
  3. `/v1/[[...path]]/route.ts` lacks the dev fallback `http://localhost:8080`, `redirect: 'manual'`, `transfer-encoding` header removal, and `204/304` null-body guard; `/api/[[...path]]/route.ts:81-93` forces `Content-Type: text/event-stream` even when `/api/sse` returns JSON errors (`401`/`403`/`500`) or when responses are `application/x-ndjson`.
  4. `middleware.ts:36-39` skips auth and onboarding redirect protection for any request carrying `User-Agent: ...Next.js...` or header `x-next-data`.

#### D-10 (P0): Systemic `user_id`-Only Scoping Across 14+ Domain Repositories & Dormant PostgreSQL RLS
- **Evidence:** All 40 `apps/api/internal/repository/*.go` files (`set_config` hits = `0`), `apps/api/internal/handlers/{gateway_key.go:130,208,219; credential.go:85,115,398,421-428,459; dashboard.go:25,40,58,70; logs.go:21,57; user_permissions.go:38-45; gateway.go:558-563}`, `apps/api/internal/middleware/tenant.go:104-125`, `apps/api/cmd/server/main.go:362`, `apps/app/context/AuthContext.tsx:72-83`, `apps/app/stores/useTenantStore.ts:33-45`, `apps/app/lib/api/client.ts:22-35`, `apps/app/hooks/mutations/useSandboxMutation.ts:16-44`, `apps/app/app/(dashboard)/sandbox/page.tsx:453-465`.
- **Mechanism:**
  1. **Dormant RLS:** `0` occurrences of `set_config`, `app.current_user_id`, or `app.current_org_id` exist in `apps/api/internal/`.
  2. **Single-User Queries on Multi-Tenant Entities:** 14 domain repositories (`agent`, `agent_template`, `budget`, `budget_alert`, `cost_anomaly`, `governance_policy`, `mcp_invocation`, `mcp_server`, `model`, `provider`, `routing_decision`, `routing_policy`, `routing_rule`, `tool`) lack `org_id` filtering altogether; handlers for `org_id`-capable repos (`gateway_key.go`, `credential.go`, `dashboard.go`, `logs.go`) still query by `userID`, and `dashboard.go:70` mixes `userID`-scoped providers with `OrgID`-scoped health scores.
  3. **Permissions Endpoint Missing Tenant Validation:** `GET /api/user/permissions` (`main.go:362`) is outside `TenantMiddleware`; `user_permissions.go:38-44` reads `"organizationId"` from Gin context (which is never populated without `TenantMiddleware`) and falls back to `user.OrgID`, ignoring `X-Prism-Org-ID` on org switch.
  4. **Cross-Tenant Provider Interference:** `credential.go:421-428` and `gateway_key.go:130` call unscoped `CountActiveByProviderID` across all tenants on shared system providers.
  5. **Unvalidated Session Workspace/Project Headers & Async Tenant Bypass:** `middleware/tenant.go:104-125` verifies `OrgID` membership for sessions but accepts `X-Prism-Workspace-ID` / `X-Prism-Project-ID` without checking hierarchy ownership inside `OrgID`. `AsyncChatCompletions` (`gateway.go:558-563`) reads `GetTenantContext` without re-validating `ResolveCanonicalTenantContext(c, keyObj)` and falls back to non-UUID `req_<unixnano>` IDs.
  6. **Frontend State Leakage:** `AuthContext.logout` never calls `useTenantStore.getState().resetTenant()`; `setSelectedOrg` never clears `selectedWorkspaceId`; `client.ts` never sends `X-Prism-Project-ID`; `useSandboxMutation.ts` and `sandbox/page.tsx:453-465` use raw `fetch()` without `X-Prism-Org-ID` or `X-Prism-Workspace-ID`.

---

### P1 — Protocol Adapters, Attribution, Governance, Console Contracts & CI Hygiene

#### D-11 (P1): Cross-Provider Payload Corruption & Adapter Protocol Defects
- **Evidence:** `apps/api/internal/proxy/engine.go:515,879`, `apps/api/internal/proxy/google.go:38-40,58,178-214`, `apps/api/internal/proxy/anthropic.go:21-48,206-220`, `apps/api/internal/proxy/openai_responses.go:17-63,153-184`, `apps/api/cmd/server/main.go:578-595`.
- **Mechanism:**
  - `Engine.Proxy` and `ProxyStream` run `SanitizeMessagesForGoogle(req.Messages)` for **every** provider (`engine.go:515,879`), injecting `"thought_signature"` / `"thoughtSignature"` into assistant/tool messages and tool calls sent to OpenAI, Anthropic, and custom OpenAI-compatible endpoints.
  - `GoogleAdapter.BuildRequest` (`google.go:38-40,58`) silently rewrites `gemini-2.0-flash` to `gemini-3.6-flash` and omits `temperature: 0` (`if req.Temperature > 0`).
  - `AnthropicAdapter` (`anthropic.go:43-48,206-220`) omits required `max_tokens` default, drops `temperature: 0`, omits `role: "tool"` -> Anthropic `tool_result` translation, and drops `tool_use` SSE `content_block_delta` chunks; no native `POST /v1/messages` route exists in `main.go` for Claude Code.
  - `OpenAIResponsesAdapter` (`openai_responses.go:17-47,153-184`) drops `tools`, `max_tokens`, and `temperature`; mis-parses `response.output_text.delta` as `{delta: string}` JSON object instead of OpenAI's string delta; and unmarshals `response.completed` usage from `event.Delta` instead of `event.Response.Usage`.

#### D-12 (P1): Telemetry, Attribution, Streaming Keepalive & Audit Durability Bugs
- **Evidence:** `apps/api/internal/proxy/engine.go:72-95,742-746,1280-1302`, `apps/api/internal/handlers/gateway.go:388-420`, `apps/api/internal/service/post_processor.go:195-217`, `apps/api/internal/proxy/audit_recorder.go:29-33,48-75`, `apps/api/internal/proxy/side_effects.go:95-102`, `apps/api/internal/proxy/sse_stream.go:73-107`, `apps/api/internal/proxy/resource_gateway.go:129-135,205-207`.
- **Mechanism:**
  1. `ProxyStream` captures `ttftMs` (`engine.go:1116,1122`) but omits `TTFTMs: ttftMs` in `&models.RequestLog{...}` (`:1280-1296`), leaving DB `ttft_ms`, `/api/analytics/logs` `AvgTTFTMs`, and `ai_audit_trails.ttft_ms` permanently `0`.
  2. `Engine.Proxy/ProxyStream` calls `telemetry.RecordRequestMetrics` (`engine.go:742-746,1298-1302`) before `Orchestrator` populates `log.OrgID` (`orchestrator.go:202`), tagging all Prometheus request metrics with `org_id="unknown"`.
  3. `GatewayHandler.handleProxyError` (`gateway.go:388-420`) records failed gateway requests with empty `OrgID`, `WorkspaceID`, `ProjectID`, `AgentID`, and `LatencyMs: 0`.
  4. `recordMCPInvocationsIfAny` (`engine.go:72-95`) writes synthetic `StatusCode: 200, LatencyMs: 0` rows into `mcp_server_invocations` whenever a model emits a `server__tool` call, inflating real `MCPGateway.ExecuteTool` stats without executing any MCP server.
  5. `post_processor.go:195-206` hashes surrogate metadata strings (`model:requestID`) rather than request/response payloads and hardcodes empty `FailoverChain`/`ToolsInvoked`/`MCPServersCalled` (unlike synchronous `orchestrator.go:371`); `audit_recorder.go:29-33` computes unkeyed `SHA-256` over 5 fields (excluding `org_id`, `agent_id`, `cost_usd`, `compliance_status`, `tools_invoked`).
  6. `SideEffects.Go` (`side_effects.go:95-102`) drops overflow tasks once the 1024-item channel fills; `sse_stream.go:73-107` writes HTTP `200` + `X-Prism-Model` for route 0 on the 15s keepalive tick before pre-first-token failover finishes.
  7. `ResourceGateway.executeRestBackend` hardcodes `LatencyMs: 0` (`resource_gateway.go:134`), and `executePostgresBackend` does not guard `b.TimeoutMs <= 0` (`:205`).

#### D-13 (P1): Governance, Agent Identity & Multi-Replica Budget Enforcement Gaps
- **Evidence:** `apps/api/internal/middleware/agent_policy.go:20,41-43`, `apps/api/internal/models/gateway.go:14-32`, `apps/api/internal/proxy/rbac_engine.go:37`, `apps/api/internal/proxy/budget_manager.go:34-40,57-173`, `apps/api/internal/repository/quota.go`.
- **Mechanism:**
  - `GatewayAPIKey` has no `AgentID` pinning column (`models/gateway.go`), and `AgentPolicyMiddleware` returns `true` when headers are absent (`agent_policy.go:41-43`) while ignoring `X-Paperclip-Agent-ID` (`:20`).
  - `matchPattern` (`rbac_engine.go:37`) falls back to `strings.Contains`, causing substrings like `"gpt-4"` to match `"gpt-4o-mini"`.
  - `BudgetManager` (`budget_manager.go:34-173`) holds in-flight reservations in process memory (`mu` + `inFlight` map keyed by `userID`) and reads `budget_alerts`/budgets scoped only by `userID` — ineffective across >1 API replicas and blind to org/workspace/project budgets. `tenant_quotas.max_concurrent_streams` has UI editing support (`QuotaFormSheet.tsx`) and storage, yet zero gateway runtime enforcement.

#### D-14 (P1): Playground, Sandbox, Resource, Logs, Audit, Turnstile, RBAC, MCP Registry & Billing Mismatches
- **Playground DTO Mismatch:** `apps/app/app/(dashboard)/playground/page.tsx:44-55,173-186` sends only `{ prompt }` (ignoring model select), reads `decision.strategy` (`SimulateResponse` returns `policyName`), `cand.totalScore` (returns `score`), `cand.estimatedCostUSD` (returns `inputPrice1M`/`outputPrice1M`), and `topCand.estimatedLatencyMs` (absent) — displaying `0%`, `$0/1K`, `'Auto Score'`.
- **Sandbox Copy Bug & Missing `?agent=` Wiring:** `apps/app/app/(dashboard)/sandbox/page.tsx:1022-1038` renders `{!executionOutput || (isExecuting && <Button ...>)}` — hiding the Copy Result button whenever output exists and execution finishes; never uses `useSearchParams` even though `agents/[id]/page.tsx:342,618` links to `/sandbox?agent=${agent.name}`.
- **Resource Dialog & Schema Wipe:** `apps/app/app/(dashboard)/resources/_components/ResourceFormDialog.tsx:102` hardcodes `backends: []` (impossible to configure runnable REST/GraphQL/Postgres backends in UI); `handlers/resource.go:247` overwrites `ParametersSchema` to `{}` when omitted on update.
- **Logs Inspector False Positive:** `apps/app/app/(dashboard)/logs/page.tsx:226-230` hardcodes a green dot and `"1. Ingress & Admission: RBAC & Budget OK"` even on `403`/`429` denied requests.
- **Audit Trail Export:** `apps/app/app/(dashboard)/audit-trail/page.tsx:188-203` always exports system admin logs (`/api/audit-trail/export`), never exporting AI cryptographic audit trails on the active tab; both tabs paginate client-side.
- **Models Page & Dashboard Copy Drift:** `models/page.tsx:51,127,130,186` uses hardcoded fallback scores (`85`, `92%`, `95%`) and claims live sync with Ollama/Vertex; `dashboard/page.tsx:69,93,488` maps `"All Time"` to `days: 90` on usage analytics and hardcodes `"via Smart Routing"` under Gateway Spend.
- **Turnstile Disabled Config Ignored:** `handlers/auth.go:59-62` returns `{ enabled, siteKey }`, whereas `lib/api/auth.ts:44` drops `enabled` and `hooks/useTurnstile.ts:35-40` substitutes dummy test site key `'1x00000000000000000000AA'`, showing Turnstile even when disabled backend-side.
- **Permission Slug & Role Collapse:** `apps/app/types/roles.ts:1-19` uses plural slugs (`credentials:read`, `models:write`, `policies:write`, `gateway_keys:write`, `governance:manage`) and `UserRole.ADMIN`, whereas `main.go` enforces singular codes (`credential:read`, `model:update`, `routing:update`, `api_key:create`, `governance:update`) and `PermissionProvider.tsx:19-48` collapses `admin`/`developer` to `UserRole.MEMBER` while defaulting uninitialized context to `OWNER` (`hasPermission: () => true`).
- **MCP Registry Client Mismatch:** `apps/app/lib/api/mcp.ts:67-75` expects unwrapped arrays from `GET /mcp/registry` (Go `mcp_registry.go:40-43` returns `{ object: "list", data: [...] }`) and calls non-existent `POST /mcp/registry/register` (`main.go:540` mounts `POST /api/mcp/registry` requiring `name`, `slug`, `serverUrl`).
- **Billing Client Mismatch (Currently Feature-Flagged Off):** `apps/app/lib/api/billing.ts:9-26` + `types/billing.ts` vs `handlers/billing.go` + `repository/billing.go`: 5 mismatches (`/billing/subscription` vs `/billing/subscription/upgrade`, `planId` vs `planSlug`, `priceMonthlyCents` vs `priceMonthlyUSD`, flat `planSlug` vs nested `plan.slug`, invoice status enum).

#### D-15 (P1): Tooling, CI Gate, Test Infrastructure & Worker Shutdown Defects
- **Missing `eslint` & Unchecked CI Lint:** `apps/app/package.json` defines `"lint": "eslint ."` (`:10`) without `eslint` in `devDependencies`, causing `pnpm --filter prism-dashboard lint` to fail (`ENOENT`); `.github/workflows/app-ci-cd.yml:59-71` does not run `pnpm lint`.
- **Vitest Node 26 Crash & Timeouts:** `apps/app/stores/useTenantStore.ts:15` calls `localStorage.getItem('org_id')` checking only `typeof window !== 'undefined'`, crashing `lib/api/client.test.ts` on Node 26; `DataTable.test.tsx` takes `6022ms` isolated (>5000ms default Vitest timeout).
- **Missing Worker Drain & Tempo Config:** `apps/api/internal/workers/worker.go:32-38` spawns background goroutines without a `sync.WaitGroup` for shutdown draining; `deploy/otel/docker-compose.otel.yml:31` mounts `./tempo.yaml`, which does not exist in `deploy/otel/`.
- **Flaky Benchmark Test:** `TestBaseline_LatencyOverhead` (`apps/api/internal/proxy`) passes in isolation (`task-1808`) and fails under parallel CPU contention (`task-1829`).

---

## 4. Product Workflow Gaps & Codebase Hygiene

### Implemented Backend APIs With Zero Active Frontend UI
1. **Workspaces & Projects:** `GET/POST /api/workspaces` and `GET/POST /api/workspaces/:workspaceId/projects` (`handlers/workspace.go`, `main.go:388-391`) — `0` references in `apps/app`.
2. **Pilot Outcomes Report & Export:** `GET /api/analytics/pilot-outcomes` & `/export` (`handlers/pilot_outcomes.go`, `main.go:434-435`) — `0` references in `apps/app`.
3. **Routing Rules:** `GET/POST/PUT/DELETE /api/routing-rules` (`main.go:469-473`) — `0` references in `apps/app`.
4. **FinOps Anomalies & Budget Alerts:** `GET /api/analytics/finops/anomalies`, `/budget-alerts`, `POST .../acknowledge` (`main.go:437-439`) — `0` references in `apps/app`.
5. **Agent Templates:** `GET/POST /api/agent-templates`, `POST .../instantiate` (`main.go:551-553`) — client wrappers exist in `lib/api/agents.ts`, `0` UI callers.
6. **MCP Registry Catalog:** `GET /api/mcp/registry`, `POST /api/mcp/registry` (`main.go:538-540`) — broken client functions in `lib/api/mcp.ts`, `0` UI callers.
7. **Inbound Governed MCP Endpoint:** `POST /v1/mcp` (`main.go:590`) — `0` UI config generator/inspector or SDK helper.

### Incomplete UI Flows & Stub/Dead Code
- **Incomplete UI:** No Provider Edit/Enable (`providers/page.tsx`), no Budget Edit and no Create Quota entry point when `quotas` list is empty (`budgets/page.tsx`), no Resource Backend editor (`ResourceFormDialog.tsx`), no Sign Out button on `/accept-invite` email mismatch (`accept-invite/page.tsx`).
- **Backend Dead Stub:** `apps/api/internal/service/metering.go:31-64` returns hardcoded `TotalRequests: 1284`, `TotalCostUSD: 14.82` and has `0` runtime callers outside itself.
- **28 Verified Unused `apps/app` Files (`0` External References):** `components/atoms/{DataTable,PageHeader,Icons,ConfirmButton,StatusTag,Separator,index}.tsx`, `context/ThemeContext.tsx`, `lib/http/client.ts`, `stores/useSystemStore.ts`, `theme/prismTheme.ts`, `hooks/queries/{useModelsQuery,useModelsPageQuery}.ts`, `hooks/mutations/index.ts`, plus all `14` `features/*/query-keys.ts` files.
- **Engineering Rule Violations:** `.agents/rules/strict-eslint-and-build-verification.md` (`eslint` missing); `.agents/rules/no-hardcoded-routes.md` (raw `/api/...` URLs/`fetch` in `signin/page.tsx:121-123`, `signup/page.tsx:104-106`, `sandbox/page.tsx:453`, `useSandboxMutation.ts:40`); `.agents/rules/api-sdk-contract-sync.md` (`packages/sdk` lacks `/v1/mcp`, `/api/workspaces`, `/api/analytics/pilot-outcomes`, `/api/budgets`, `/api/quotas`, `/api/logs`).

---

## 5. Security & Tenant Isolation Findings

### Repository Scoping & `sqlrowserr` Matrix (`40` Non-Test Files)
| Category | Count | Repository Files / Notes |
|---|---|---|
| **Domain Repos Lacking `org_id` Filtering** | 14 | `agent.go`, `agent_template.go`, `budget.go`, `budget_alert.go`, `cost_anomaly.go`, `governance_policy.go`, `mcp_invocation.go`, `mcp_server.go`, `model.go`, `provider.go`, `routing_decision.go`, `routing_policy.go`, `routing_rule.go`, `tool.go` |
| **System / Child / Helper Repos Without `org_id`** | 11 | `errors.go`, `gateway_key_cache.go`, `mcp_tool.go`, `model_latency_hourly.go`, `model_pricing.go`, `request_payload.go`, `resource_backend.go`, `retention.go`, `session.go`, `tool_backend.go`, `tool_invocation.go` |
| **Repos Containing `org_id` (Partially Used by Handlers)** | 15 | `account.go`, `audit_trail.go`, `billing.go`, `credential.go`, `gateway_key.go`, `mcp_registry.go`, `pilot_outcomes.go`, `provider_health.go`, `quota.go`, `rbac.go`, `request_log.go`, `resource.go`, `setting.go`, `user.go`, `workspace.go` |
| **PostgreSQL RLS Runtime Activation (`set_config` / `app.current_*`)** | 0 | **Dormant** across 100% of `apps/api/internal/` |
| **`rows.Next()` == `rows.Err()` Compliance** | 40 / 40 | `0` violations across entire repository layer |

### Consolidated Security Summary
1. **Next.js Proxy Path Traversal & Middleware Bypass (`D-09`):** Unvalidated `..` segments allow escaping `/api/` and `/v1/` prefixes to backend root paths (`/metrics`, `/ready`, `/swagger/*`); spoofing `User-Agent: Next.js` or `x-next-data` skips frontend auth/onboarding redirects.
2. **Tenant Header & Async Gaps (`D-10`):** Session requests do not validate `X-Prism-Workspace-ID`/`X-Prism-Project-ID` hierarchy inside `OrgID`; `/api/user/permissions` lacks `TenantMiddleware`; `AsyncChatCompletions` skips `ResolveCanonicalTenantContext`.
3. **Secret Overwrite / Wipe on Edits (`D-05`, `D-06`):** MCP Server edits corrupt encrypted Bearer tokens with `"Bearer fc-b••••1234"`; Tool edits erase `AuthTokenEncrypted`.
4. **Opt-In Agent Binding & Permissive RBAC Matching (`D-13`):** Gateway keys cannot bind to a fixed `AgentID`, and `rbac_engine.go:37` `strings.Contains` fallback widens pattern matching beyond globs.
5. **Audit Signature Forgery Surface (`D-12`):** `AuditRecorder.ComputeSignatureHash` uses unkeyed SHA-256 over 5 fields without HMAC or key separation and without signing tenant, agent, cost, decision, or tools.
6. **HttpOnly Logout Deadlock & Token Exposure (`D-04`):** `POST /api/auth/logout` refuses expired-session browsers from clearing `HttpOnly` cookies; `Login`/`Signup` JSON bodies expose raw session tokens copied into non-HttpOnly cookies.
7. **Cross-Tenant Provider Count & Unvalidated Tool Save URL (`D-06`, `D-10`):** `CountActiveByProviderID` counts globally across shared providers; `ToolHandler.Create/Update` omits save-time `ValidateOutboundURL` check (mitigated at execution time by `SafeHTTPClient`).

---

## 6. App/API Cross-System Integration Traces (10 Prioritized Workflows)

| # | Workflow | UI Entry Point | App API Client / Hook | Next.js Boundary | Go Route & Middleware | Handler -> Service -> Repo | DB Tables / Redis | Automated Tests (Observed) | Integrity Status |
|---|---|---|---|---|---|---|---|---|---|
| **1** | **Org, Workspace & Project Isolation** | `TenantSelector.tsx`, `AuthContext.tsx` | `organizations.ts`, `client.ts`, `useTenantStore.ts` | `middleware.ts`, `api/[[...path]]` | `TenantMiddleware` -> `GET /api/user/organizations`, `GET/POST /api/workspaces` | `user_permissions.go`, `workspace.go` -> `rbac.go`, `workspace.go`, `account.go` | `organizations`, `organization_members`, `workspaces`, `projects` | `client.test.ts` (**FAIL** Node 26 `localStorage`); Go tenant tests (DB skipped) | **BROKEN (`D-10`)**: 0 WS/Project UI; `user/permissions` misses `TenantMiddleware`; session WS/Proj unvalidated; `logout` leaks store; `X-Prism-Project-ID` never sent |
| **2** | **Provider Credential Management** | `providers/`, `credentials/`, `ProviderFormDialog.tsx` | `providers.ts`, `credentials.ts`, `useCredentialMutations.ts` | `api/[[...path]]` | `Auth` + `RestrictGatewayKeys` + `Tenant` + `RequirePermission` | `provider.go`, `credential.go` -> `provider.go`, `credential.go` | `providers`, `credentials`; Redis cooldown/quota/active_streams | Go handler/repo tests (**SKIPPED** no DB/Redis) | **BROKEN (`D-01`, `D-02`, `D-10`)**: Delete & Reset Cooldown 100% 404 (arg swap); Custom provider `enabled=false` + duplicate `slug=""` 409; unscoped `CountActiveByProviderID` |
| **3** | **Gateway Key Creation & Revocation** | `gateway-keys/`, `GatewayKeyFormDialog.tsx` | `gateway-keys.ts`, `useGatewayKeysQuery.ts` | `api/[[...path]]` | `Auth` + `Tenant` + `RequirePermission("api_key:*")` | `gateway_key.go` -> `gateway_key.go`, `GatewayKeyCache` | `gateway_api_keys`; Redis/memory `gateway_key_cache` | `GatewayKeyFormDialog.test.tsx` (pass at 15s timeout, timeout at 5s); Go repo tests (skipped) | **PARTIAL (`D-10`)**: `List`/`Delete` omit `orgID`; `CountActiveByProviderID` cross-tenant; no `agent_id` binding |
| **4** | **Routing Policy Creation & Live Inference** | `policies/`, `PolicyFormDialog.tsx`, `PolicyTuner.tsx`, `playground/`, `sandbox/` | `policies.ts`, `useSandboxMutation.ts` | `api/[[...path]]`, `v1/[[...path]]` | `GatewayAuth` -> `Tenant` -> `RateLimit` -> `AgentPolicy` -> `Idempotency` -> `/v1/chat/completions` | `policy.go`, `simulate.go`, `gateway.go` -> `Orchestrator` -> `Engine` -> `Router` -> `Scorer` -> `routing_policy.go` | `routing_policies`, `routing_rules`, `routing_decisions`; Redis telemetry/cooldown | `scorer_test.go` / `router_test.go` / `orchestrator_test.go` (pass) | **BROKEN (`D-08`, `D-11`, `D-14`)**: `0-100` UI vs `0.0-1.0` scorer breaks thresholds; Playground DTO mismatch; `SanitizeMessagesForGoogle` mutates non-Google payloads; Google/Anthropic/Responses adapter bugs |
| **5** | **Budget Enforcement Under Concurrency** | `budgets/`, `BudgetFormSheet.tsx`, `QuotaFormSheet.tsx` | `budgets.ts`, `quotas.ts` | `api/[[...path]]` | Gateway & Sandbox -> `Orchestrator.ExecuteChatCompletions` -> `AdmissionController` & `BudgetManager` | `budget.go`, `quota.go` -> `BudgetManager.Reserve/Release` -> `budget.go`, `quota.go` | `budgets`, `budget_alerts`, `tenant_quotas` | `budget_manager_test.go` / `admission_test.go` (`-race` pass) | **PARTIAL (`D-10`, `D-13`)**: Single-replica `userID`-only memory lock (no distributed Redis hold, no org/WS budget scope); `MaxConcurrentStreams` un-enforced; no Edit Budget / empty-state Create Quota UI |
| **6** | **Agent Identity & Governance** | `agents/`, `agents/[id]/`, `AgentFormDialog.tsx`, `governance/` | `agents.ts`, `governance.ts` | `api/[[...path]]`, `v1/[[...path]]` | `AgentPolicyMiddleware` -> `Orchestrator` -> `AdmissionController` -> `RBACEngine` | `agent.go`, `governance_policy.go`, `gateway.go:applyAgentBoundary` -> `agent.go`, `governance_policy.go` | `agents`, `agent_templates`, `governance_policies` | `rbac_engine_test.go`, `control_plane_e2e_test.go` (pass) | **BROKEN (`D-07`, `D-10`, `D-13`)**: Agent edit wipes `systemPromptOverride`; backend cannot clear allowlists/`0` budget; header-only opt-in identity; `strings.Contains` RBAC match; Sandbox ignores `?agent=` |
| **7** | **MCP Tool Authorization** | `mcp/`, `mcp/[id]/`, `MCPServerFormDialog.tsx`, `MCPTestModal.tsx` | `mcp.ts` (`useMCPMutations.ts`) | `api/[[...path]]`, `v1/[[...path]]` | `POST /v1/mcp` or `GET/POST/PUT/DELETE /api/mcp/servers*` | `mcp.go`, `mcp_server.go`, `mcp_registry.go` -> `MCPGateway`, `Admission` -> `mcp_server.go`, `mcp_tool.go` | `mcp_servers`, `mcp_tools`, `mcp_registry`, `mcp_server_invocations` | `mcp_server_test.go`, `mcp_gateway_test.go`, `mcp_registry_test.go` (pass) | **BROKEN (`D-05`, `D-12`, `D-14`)**: Toggle switch `400`; Edit overwrites Bearer token with bullets; registry API client wrong path/shape; synthetic invocation logging |
| **8** | **Usage & Cost Attribution** | `dashboard/`, `logs/`, `sandbox/` | `logs.ts`, `dashboard.ts`, `SSEContext.tsx` | `api/[[...path]]`, `v1/[[...path]]` | `/v1/chat/completions`, `/v1/chat/completions/async` -> `Orchestrator` -> `Engine` -> `AsyncPostProcessor` | `gateway.go`, `Orchestrator`, `AsyncPostProcessor`, `Engine.backfillActualCost` -> `request_log.go`, `routing_decision.go` | `request_logs`, `request_payloads`, `routing_decisions`; Prometheus metrics | `logs_dashboard_test.go` (unit), `request_log_test.go` (DB skipped) | **DEGRADED (`D-10`, `D-12`)**: `TTFTMs` always `0`; Prometheus `org_id="unknown"`; failed requests lose tenant/agent metadata; `SideEffects`/AsyncPostProcessor memory-only drop risk; SSE dead for OAuth |
| **9** | **Audit Event Creation & Verification** | `audit-trail/` | `audit.ts` | `api/[[...path]]` | `GET /api/audit-trail{,/logs,/export}`, `POST /api/audit-trail/:id/verify` | `audit_trail.go`, `audit_log.go`, `Orchestrator` -> `AuditRecorder` -> `audit_trail.go` | `ai_audit_trails`, `audit_logs` | Go audit handler unit tests | **DEGRADED (`D-12`, `D-14`)**: Async AI trails hash surrogate metadata & zero tools/failovers; unkeyed 5-field SHA-256 signature; UI Export button never exports AI audit trails; `ai_audit_trails` `user_id`-scoped |
| **10** | **Dashboard Analytics Consistency** | `dashboard/`, `logs/` | `dashboard.ts`, `logs.ts`, `policies.ts` (`finops`) | `api/[[...path]]` | `GET /api/dashboard/{stats,usage,health,active-streams}`, `GET /api/logs`, `GET /api/analytics/*` | `dashboard.go`, `logs.go` -> `request_log.go`, `ProviderHealthStore` | `request_logs`, `credentials`, `providers`; Redis active_streams/telemetry | `logs_dashboard_test.go` | **DEGRADED (`D-10`, `D-12`, `D-14`)**: `user_id`-only queries (misses teammates' org traffic); `dashboard.go:70` mixes user providers + org health scores; `"All Time"` = `90d`; Logs inspector lies on 403/429 |

---

## 7. Verification Matrix, Test Results & Skipped Environment Checks

All verification commands below were executed on **2026-11-20** against the repository at commit `e31a1af`. Docker Desktop was offline (`//./pipe/dockerDesktopLinuxEngine` unavailable), so live PostgreSQL and Redis containers were not running during today's execution.

| Target / Suite | Command Executed | Outcome | Details & Caveats |
|---|---|---|---|
| **Go Vet** | `cd apps/api && go vet ./...` | **PASS** | Exit code `0`, `0` diagnostics |
| **Go Test Suite (Unit + Skipped DB)** | `cd apps/api && go test -count=1 ./...` | **PASS (with 94 skips)** | `17` testable packages passed, `5` packages `[no test files]`; **`94` DB/Redis-dependent tests SKIPPED** across `internal/database`, `internal/handlers`, `internal/proxy`, `internal/queue`, `internal/redis`, `internal/repository`, `internal/workers`. **`TestBaseline_LatencyOverhead`** in `internal/proxy` is CPU-load flaky (passed solo; failed once under parallel load in `task-1829`). |
| **Go Race Detector** | `cd apps/api && go test -race -count=1 ./internal/proxy/... ./internal/middleware/... ./internal/handlers/... ./internal/service/... ./internal/security/... ./internal/config/... ./internal/utils/...` | **PASS** | `7/7` packages passed, `0` data races |
| **Next.js Typecheck** | `pnpm --filter prism-dashboard typecheck` | **PASS** | `tsc --noEmit`, `0` errors |
| **Next.js Production Build** | `pnpm --filter prism-dashboard build` | **PASS** | Next.js 16.3.3 Turbopack, `6/6` static pages generated; warns `The "middleware" file convention is deprecated. Please use "proxy" instead.` |
| **Next.js Lint** | `pnpm --filter prism-dashboard lint` | **FAIL (Exit 1)** | `eslint: command not found` (`ENOENT`; `eslint` absent from `apps/app/package.json`) |
| **Frontend Vitest Suite** | `pnpm --filter prism-dashboard test` | **FAIL (Exit 1)** | `7/10` files passed (`33/35` tests): `lib/api/client.test.ts` fails deterministically at `useTenantStore.ts:15` (`localStorage.getItem` undefined under Node 26); `DataTable.test.tsx` (`6022ms` isolated) & `GatewayKeyFormDialog.test.tsx` (`3294ms` isolated) timed out at default `5000ms` under contention, both **PASS (5/5 tests)** when run isolated with `--testTimeout=15000`. |
| **SDK & CLI Builds** | `pnpm run build:sdk` & `pnpm run build:cli` | **PASS** | Emits `@roozylabs/prism` CJS/ESM/DTS and `@roozylabs/prism-cli` CJS/DTS |

### Missing Automated Verification
1. **Live PostgreSQL + Redis Execution Today:** The `94` DB/Redis tests could not run locally while Docker Engine is stopped (historical live execution was recorded previously in `docs/verification/product-integrity-verification.md`, not re-verified today).
2. **Browser E2E Suite:** Zero Playwright / Cypress browser E2E tests exist in `apps/app` — all cross-system contract bugs (`D-01`–`D-08`, `D-14`) escaped unit tests because Vitest mocks API responses.
3. **Real HTTP Load/Chaos Benchmarks:** `load_chaos_test.go` tests in-memory mocks rather than a running `cmd/server` binary backed by Postgres + Redis.
4. **CI Lint Enforcement:** `.github/workflows/app-ci-cd.yml` runs `pnpm test`, `pnpm typecheck`, and `pnpm build` on Node 20 without `pnpm lint`.

---

## 8. Architectural Recommendations (Distinct From Confirmed Defects)

These structural upgrades are **recommendations**, not current defect claims:
1. **Transactional Outbox + DLQ for Financial & Security Telemetry:** Replace volatile in-memory Go channels (`SideEffects` / `AsyncPostProcessor`) with a durable DB outbox table + retry worker for `request_logs`, `routing_decisions`, and `ai_audit_trails`.
2. **Distributed Multi-Replica Spend Reservation (Redis Lua):** Move `BudgetManager`'s in-process mutex/map into atomic Redis hash/Lua reservations keyed by `(org_id, workspace_id, user_id)` with DB reconciliation so horizontal API scaling cannot breach hard limits.
3. **Active PostgreSQL RLS Via Non-Superuser App Role:** Add a transactional DB helper setting `SET LOCAL app.current_user_id` / `app.current_org_id` and execute queries under a non-`BYPASSRLS` role so database RLS enforces tenant boundaries as true defense-in-depth.
4. **Schema-First TypeScript Contract Generation:** Generate TypeScript types and API clients for `apps/app` and `packages/sdk` from Go handlers/OpenAPI specs to eliminate manual DTO drift (`Playground`, `Billing`, `MCP Registry`).
5. **Cookie/URL Tenant Context & Next 16 `proxy.ts` Migration:** Move active `orgId`/`workspaceId`/`projectId` out of plain `localStorage` into validated server-readable state/URL scope, remove client-writable JWT cookies in `AuthContext`, and migrate `middleware.ts` to Next 16's `proxy.ts`.

---

## 9. Prioritized Remediation Backlog (P0–P3), Acceptance Criteria & Sequence

### Prioritized Backlog
| Priority | IDs | Workstream | Acceptance Criteria |
|---|---|---|---|
| **P0** | `D-01`, `D-02` | Credential & Custom Provider Fixes | `useCredentialMutations` passes `(d.credId, d.providerId)`; `ProviderFormDialog` sends generated slug + `enabled` + closes/toasts on result; `providers/page.tsx` supports Edit/Toggle. |
| **P0** | `D-03` | Invite Auth & Redirect Loop Repair | `/accept-invite` whitelisted from `401` redirect in `client.ts`; mismatched user gets Sign Out action; OAuth/Sign Up preserve `?invite=`; invite flow outranks un-onboarded redirect; `useAcceptInviteMutation` invalidates `['user-organizations']` + `['user-permissions']`. |
| **P0** | `D-04` | HttpOnly SSE, Async Polling & Logout Fix | `SSEContext` connects via Next `/api/sse` without reading `document.cookie` for `HttpOnly` sessions; `sandbox/page.tsx` polls `GET /api/jobs/:jobId` as fallback; `POST /api/auth/logout` clears cookie even on expired sessions; remove client-side non-HttpOnly `Cookies.set('auth_token')`. |
| **P0** | `D-05`, `D-06`, `D-07` | MCP / Tool / Agent Secret & State Preservation | Add dedicated enable-toggle/PATCH support on `MCPHandler`; preserve existing Authorization/Bearer secrets when masked `"••••"` arrives on `req.AuthToken` or `req.Headers`; preserve existing `ToolBackend.AuthTokenEncrypted` when UI omits token + validate tool backend URLs on save; expose `systemPromptOverride`/`allowedModels`/`enabled` in `AgentFormDialog` and allow explicit empty arrays / `0` budget updates in `AgentHandler`. |
| **P0** | `D-08` | Policy Weight Normalization & Scale Sync | Normalize weights in `scorer.go` (or validate `0.0–1.0` sum + convert at UI boundary) and align `PolicyFormDialog` / `PolicyTuner` so seeded and custom policies produce consistent `[0, 1]` scores matching `0.50` downgrade thresholds. |
| **P0** | `D-09` | Next.js Proxy & Middleware Hardening | Reject `..`/`%2e%2e`/backslash segments in `/api` & `/v1` proxies; propagate `request.signal`; bring `/v1` proxy up to parity; stop forcing `text/event-stream` on `/api/sse` JSON errors; remove `User-Agent: Next.js` / `x-next-data` bypass in `middleware.ts`. |
| **P0** | `D-10`, `P0-5` | Multi-Tenant `org_id`/RLS Scoping & Key/Agent Pinning | Scope all 14 domain repos + handlers (`gateway_key`, `credential`, `dashboard`, `logs`, `ai_audit_trails`) by `org_id` (+ workspace/project where applicable); add `TenantMiddleware` on `/api/user/permissions`; validate session workspace/project ownership; enforce `ResolveCanonicalTenantContext` on `AsyncChatCompletions`; scope `CountActiveByProviderID` by tenant; reset `useTenantStore` on logout & pass all tenant headers from Sandbox; add `GatewayAPIKey.AgentID` binding + fix `rbac_engine.go` exact/glob match. |
| **P1** | `D-11` | Adapter Protocol & `POST /v1/messages` Repair | Apply `SanitizeMessagesForGoogle` strictly to `GoogleAdapter`; fix `gemini-2.0-flash` rewrite, `temperature: 0`, Anthropic `max_tokens`/`tool_result`/streaming `tool_use`, `OpenAIResponsesAdapter` deltas/usage/params, and add native `POST /v1/messages`. |
| **P1** | `D-12` | Attribution, Keepalive, Latency & Audit Integrity | Assign `TTFTMs` in `ProxyStream`; populate `OrgID`/tenant context before `RecordRequestMetrics`; preserve tenant/agent IDs on `handleProxyError`; stop synthetic MCP invocations in `Engine`; hash true canonical payloads & compute keyed HMAC signatures in `AuditRecorder`; defer streaming `200`/model headers until pre-token failover completes; record REST resource latency & Postgres default timeout. |
| **P1** | `D-14` | Frontend Contract & UX Repairs | Align Playground request/DTO fields; fix Sandbox Copy button JSX & `?agent=` param; add Resource Backend form + preserve `ParametersSchema`; show real denial status in Logs inspector; export AI Audit Trails when active tab; strip fake Model fallbacks & fix Dashboard `"All Time"`/`Spend` labels; honor `turnstileConfig.enabled`; sync RBAC slugs & roles; align `mcp/registry` & `billing` clients. |
| **P1** | `D-15` | Tooling, CI & Worker Drain | Add `eslint` to `apps/app/package.json` & `.github/workflows/app-ci-cd.yml`; guard `localStorage?.getItem` in `useTenantStore.ts` for Node 26; optimize `DataTable.test.tsx`; add `sync.WaitGroup` to `workers.Manager`; add missing `deploy/otel/tempo.yaml`. |
| **P2–P3** | Gaps | Exposed UI Workflows, Hygiene & Scale | Wire existing `/api/workspaces`, `/api/analytics/pilot-outcomes`, `/api/routing-rules`, `/api/analytics/finops/{anomalies,budget-alerts}`, `/api/agent-templates`, `/api/mcp/registry`, and `/v1/mcp` into Console + SDK; remove dead `MeteringService` stub & 28 unused `apps/app` files; add Playwright E2E suite; implement Redis multi-replica budget Lua & `MaxConcurrentStreams`. |

### Recommended Phased Implementation Sequence
1. **Wave 1 — Surgical P0 Hotfixes (`D-01`, `D-02`, `D-03`, `D-05`, `D-06`, `D-07`, `D-08`, `D-09`, `D-15`):** Restore broken credential delete/cooldown, custom providers, invite links, MCP/Tool/Agent secret preservation, policy scoring, Next.js proxy security, and green `lint`/`vitest` gates first.
2. **Wave 2 — Auth, Real-Time SSE & Multi-Tenant Enforcement (`D-04`, `D-10`, `P0-5`):** Repair HttpOnly SSE + Async Sandbox polling + expired logout, migrate tenant queries/repos to `org_id` + RLS session variables, validate hierarchy headers, and bind optional `GatewayAPIKey.AgentID` + exact RBAC globs.
3. **Wave 3 — Protocol Adapters, Attribution & Audit Integrity (`D-11`, `D-12`):** Scope Google sanitization, repair Google/Anthropic/Responses adapters + add `/v1/messages`, fix `TTFTMs`/Prometheus `OrgID`/error attribution/HMAC audit signatures/stream keepalive deferral.
4. **Wave 4 — Console Contracts, Unexposed Backend Workflows & E2E (`D-14` + Gaps):** Fix Playground/Sandbox/Resource/Logs/Audit/Turnstile/RBAC UIs, expose Workspaces/Projects, Pilot Outcomes exports, Routing Rules, and FinOps Alerts, and lock everything behind live Postgres/Redis + Playwright E2E CI suites.
5. **Wave 5 — Horizontal Scale & Commercial Readiness (`P2–P3`):** Distributed Redis budget reservations, `MaxConcurrentStreams`, worker `WaitGroup` drain, `tempo.yaml` & real load tests, dead-code removal, and real subscription billing (`FEATURE_FLAG_BILLING`).
