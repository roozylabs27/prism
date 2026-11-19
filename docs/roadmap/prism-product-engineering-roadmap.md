# Prism — Prioritized Product & Engineering Roadmap (P0–P3)

**Prioritization Principle:** Security, tenant isolation, enforcement integrity, and reliable agency core workflows come strictly before visual polish, billing expansion, or multi-feature breadth.

---

## P0 — Security, Data Integrity & Enforcement Foundations

| ID | Workstream | Status | Dependencies | Est. Complexity | Acceptance Criteria | Blockers / Notes |
|---|---|---|---|---|---|---|
| **P0-1** | **Gateway Key `WorkspaceID`/`ProjectID` Pinning & Cross-Tenant Header Rejection** | **Completed (Phase 4)** | None | Low | Spoofed `X-Prism-Workspace-ID`/`X-Prism-Project-ID` returns `403 TENANT_SECURITY_ERROR` (`middleware/tenant.go`). | Verified in unit & E2E tests. |
| **P0-2** | **Close `ai_audit_trails` Cross-Tenant Leak & `ModelHandler` Ownership + MCP Secret Masking + Features Bypass** | **Completed (Phase 4)** | None | Medium | Non-admin cannot view `'user_admin'`/`''` trails; foreign/system model writes return `404`/`403`; MCP headers masked on `GET`; `?plan_tier=` ignored. | Verified in unit & handler tests (`ModelHandler`, `MCPHandler`, `FeaturesHandler`); DB tests skipped when Postgres offline. |
| **P0-3** | **Fail-Closed Redis Policy (`REDIS_FAIL_CLOSED`), Fail-Closed Quotas + WS/Project Quotas, Single-Replica Atomic Budget Reserve** | **Completed (Phase 4)** | None | Medium | Redis outage rejects with `503` when failClosed=true; DB quota/budget errors abort safely; concurrency `-race` test caps simultaneous holds at hard limit. | Multi-replica distributed lock deferred to P1-5. |
| **P0-4** | **Enforce Agent Boundaries Across Smart Router (`prism-auto`), Tool Gateway, Resource Gateway & Inbound MCP** | **Completed (Phase 4)** | P0-3 | Medium | `prism-auto` never picks disallowed model; `!deny` & RBAC block tools/resources/MCP calls before upstream; `MaxBudgetCents` enforced. | Opt-in header gap tracked in P0-5. |
| **P0-5** | **Bind `GatewayAPIKey.AgentID` + Fix `X-Paperclip-Agent-ID` Resolution + Exact RBAC Glob Matching** | **Open (Next P0)** | None | Medium | Migration adding nullable `agent_id` on `gateway_api_keys`; key-bound requests cannot omit/override agent; `X-Paperclip-Agent-ID` resolves agent via `AgentPolicyMiddleware`; replace `strings.Contains` in `rbac_engine.go:37` with glob/exact matching. | Schema migration required. |
| **P0-6** | **Systemic `org_id` + `workspace_id` Scoping Across Repositories + Active Postgres RLS Session Context** | **Open** | P0-5 | High | Shared org members view same org agents, budgets, MCP servers, routing rules, & credentials (filtered by `org_id`/`workspace_id`); Go DB wrapper sets `app.current_user_id`/`org_id` via non-superuser DB role; `CountActiveByProviderID` tenant-scoped; `/api/user/permissions` mounts `TenantMiddleware`. | High blast radius across repos/tests; do in small slices. |
| **P0-7** | **Durable Financial & Security Audit Delivery (Transactional Outbox / DLQ + True Payload Hashing)** | **Open** | P0-6 | High | `request_logs` written transactionally or via durable outbox with retry/DLQ on crash; `ai_audit_trails` hashes actual request/response & populates real `ToolsInvoked`/`FailoverChain`; failed requests retain `OrgID`/`WS`/`Proj`/`AgentID`. | Requires outbox table/worker & HMAC key decision. |

---

## P1 — Core Product Reliability & Universal Integration Completeness

| ID | Workstream | Status | Dependencies | Est. Complexity | Acceptance Criteria | Blockers / Notes |
|---|---|---|---|---|---|---|
| **P1-1** | **Inbound Governed MCP Gateway Server (`POST /v1/mcp`) + Built-In `delegate_ai_task`** | **Completed (Phase 4)** | P0-3, P0-4 | High | JSON-RPC 2.0 `initialize`/`tools/list`/`tools/call` authenticates keys, filters agent tools, executes upstream MCP & `delegate_ai_task`, returns model/cost disclosure, logs audit trails. | Verified in unit, handler, & zero-external-dep E2E tests (`Postgres/Redis` live stack E2E test written, skipped while Docker/DB offline). |
| **P1-2** | **Anthropic Messages API Endpoint (`POST /v1/messages`) & Adapter Repair** | **Open (Critical P1)** | P0-4 | High | Real Claude Code (`ANTHROPIC_BASE_URL`) multi-turn chat, tool_use/tool_result loop, streaming SSE `content_block_*` events, token usage & failover work end-to-end; fix `SanitizeMessagesForGoogle` cross-provider corruption, Google `gemini-3.6-flash` rewrite, and `*float64` `temperature: 0`. | Needed for native Claude Code inference positioning. |
| **P1-3** | **Repair `OpenAIResponsesAdapter` + Streaming Interrupt/Keepalive & `TTFTMs` Bugs** | **Open** | None | Medium | `streamInterrupted` blocks broken stream replay; no premature `X-Prism-Model` header flush on keepalive failover; `TTFTMs` written to `request_logs`; fix Responses API SSE string-delta & usage extraction; strip auth headers on cross-host HTTP redirects. | Required for OpenCode/Agents SDK reliability. |
| **P1-4** | **Human-in-the-Loop Tool/Agent Approval Workflow** | **Open** | P0-4, P1-1 | Medium-High | Add `"require_approval"` policy/tool flag + `approval_requests` table + `/api/approvals/{list,approve,deny}` + pending hold/webhook notification on `/v1/mcp` and `/v1/tools/*`. | New schema + dashboard inbox UI. |
| **P1-5** | **Distributed Multi-Replica Spend Reservation (Redis Lua) & `MaxConcurrentStreams`** | **Open** | P0-3 | Medium | Atomic Redis budget hold/reconcile across N API replicas + enforcement of `tenant_quotas.max_concurrent_streams`. | Enables horizontal API scaling. |
| **P1-6** | **Routing Evaluation, Historical Replay Engine & Simulator/Frontend DTO Fix** | **Open** | P1-2 | Medium | Persist outcome status/latency on `routing_decisions`; add `/api/routing/replay` comparing policies over historical logs; align `/api/routing/simulate` with live `ResolveSemantic` (rules, cooldowns, agent allowlists) and fix `/playground` DTO mismatch. | Blocked only by outcome logging schema addition. |
| **P1-7** | **Workspace/Project CRUD API + SDK Contract Alignment + Frontend Critical Fixes & E2E Playwright Suite** | **Open** | P0-6 | High | Add workspace/project creation & `TenantSelector` switching; fix `ResourceFormDialog` backends, `AgentFormDialog` fields, `Sandbox` `?agent=` & Copy button, `Logs` server pagination; install `eslint` & fix Node 26 test setup; add Playwright E2E covering auth -> key -> inference -> budget -> agent/MCP -> logs. | Largest UI workstream; slice per workflow. |

---

## P2 — Production Operations, Observability & Scale

| ID | Workstream | Status | Dependencies | Est. Complexity | Acceptance Criteria | Blockers / Notes |
|---|---|---|---|---|---|---|
| **P2-1** | **Fix Blue-Green Compose Aliases, Frontend Healthcheck & Rollback Policy** | **Open** | None | Low-Medium | Separate compose profiles/networks so blue & green never share DNS alias accidentally; healthcheck probes `/api/health` or `/signin`; document forward-fix migration policy. | Easy operational win. |
| **P2-2** | **Backup/Restore Automated CI Drill & Non-Superuser DB Role Verification** | **Open** | P0-6 | Medium | GitHub Actions weekly/PR job runs `pg-backup.sh` -> `pg-restore-test.sh` verifying schema, RLS roles, and sample row checksums; documented RTO/RPO drill results. | Depends on non-superuser migration strategy. |
| **P2-3** | **Real Gateway Load/Chaos Suite & Full Observability Stack Completion** | **Open** | P1-3 | Medium | Rewrite `load_chaos_test.go` to target real `cmd/server` binary + Postgres + Redis under 50–200 RPS; add missing `deploy/otel/tempo.yaml`, Alertmanager config, fix `PrismApiDown` PromQL for blue/green, add OTel spans & `Engine` `org_id` fix. | Needed before SLA guarantees. |
| **P2-4** | **Worker Graceful `WaitGroup` & Documentation Alignment (`README.md`, `ARCHITECTURE.md`)** | **Open** | P1-2, P1-7 | Low | `workers.Manager` drains in-flight background ticks cleanly on SIGTERM; align `README.md` route list & integration support matrix with verified capabilities. | Low risk cleanup. |

---

## P3 — Commercial Agency Capabilities (Only After P0–P1 Reliability + Agency PMF Validation)

| ID | Workstream | Status | Dependencies | Est. Complexity | Acceptance Criteria | Blockers / Notes |
|---|---|---|---|---|---|---|
| **P3-1** | **Client-Level Cost Attribution, CSV/PDF Executive Exports & Agency Onboarding Flow** | **Open** | P0-6, P1-7 | Medium | Per-client workspace cost/reliability/savings export ready in 1 click without manual spreadsheet work. | Gate on 3–5 agency design partner feedback. |
| **P3-2** | **White-Label Client Portal & Agency Branding (Read-Only Client Viewer Role)** | **Open** | P3-1 | High | Agency custom logo/domain & scoped client-viewer login showing only assigned client workspace usage, budgets & reports. | Build ONLY if validated in WTP interviews. |
| **P3-3** | **Real Subscription Billing / Stripe Metering (`FEATURE_FLAG_BILLING`)** | **Open** | P0-7, P3-1 | High | Replace stub `UpgradeSubscription` & hardcoded `MeteringService` with real plan persistence, webhooks, and Stripe/invoice sync. | Explicitly deferred behind feature flag. |

---

## Explicitly Out of Scope (Near Term)
- Building a custom Prism web chat / coding IDE replacement for Claude Code, Claude Desktop, or Cursor.
- Unrequested SSO/SAML/SCIM enterprise compliance suites prior to validating 5–30 client agency PMF.
- Deep local endpoint DLP / OS kernel sandboxing for arbitrary subprocesses running outside Prism's network boundary.
- Marketplace app-store monetization or unrequested multi-agent social network features.
