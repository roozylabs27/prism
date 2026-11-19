# Prism — AI Automation Agency Use Cases & Commercial Validation Plan

**Primary ICP:** AI Automation Agencies managing **5–30 active client accounts**  
**Commercial Hypothesis:** Agencies experience sub-linear operational leverage when adding clients because vendor API keys, spend limits, prompt/routing experiments, agent permissions, and client usage reporting remain scattered across workflows and vendor consoles.

---

## 1. Customer Pain Points (Hypotheses to Validate)

1. **Fragmented AI Spend Across Clients:** Vendor invoices mix usage across 10+ client chatbots/workflows with zero automatic client attribution.
2. **Credential Sprawl & Cross-Client Risk:** Raw `sk-*` vendor keys are copy-pasted into `.env` files, n8n credentials, LangGraph deployments, and contractor scripts without rotation or instant per-client revocation.
3. **Per-Client Permission Drift:** Legal, healthcare, and e-commerce clients require strictly different allowed tools, data sources, MCP servers, and approved model lists.
4. **Runaway Agent Actions:** Unconstrained automation agents can invoke destructive tools, exceed budgets in tight retry loops, or access internal resources outside client scope.
5. **Unmeasured Model Cost & Latency Trade-offs:** Agencies use expensive flagship models everywhere out of fear, or swap to cheaper models blindly without measuring latency, failure rate, and cost impact.
6. **Manual Client Usage Reporting:** Compiling monthly token/cost/reliability CSVs or executive summaries per client consumes billable engineering/ops time.
7. **Non-Linear Ops Overhead Per New Client:** Onboarding client #25 duplicates hours of key setup, guardrail scripting, and billing spreadsheet work.

---

## 2. Five Priority Agency Use Cases

### Use Case 1 — Multi-Client AI Cost & Budget Control (**Priority: P0**)
- **User Journey:** Agency operator creates an agency organization, creates a client workspace + project (`WorkspaceID`, `ProjectID`), issues a client-pinned `prism_sk_*` gateway key, sets monthly/daily quotas & budgets (`warningThreshold`, `criticalThreshold`, `hardLimit`), and monitors real-time spend. When spending hits warning/critical thresholds, Prism warns or prioritizes cheaper models under `prism-auto`; at hard limit, requests deny with `429` before upstream spend occurs.
- **Required Backend Capabilities:** Org/Workspace/Project quotas, daily request limits, atomic spend reservation, fail-closed quota/budget errors, client-scoped budgets (`org_id`/`workspace_id`/`project_id`), accurate token/cost calculation (sync + streaming), durable request logs.
- **Required Frontend Workflows:** Workspace & project CRUD selector in `TenantSelector`, per-client budget editor (`budget` edit currently missing in UI!), client-level quota configuration, live FinOps cost breakdown & alerts UI.
- **Current Readiness & Honest Gaps:**
  - *Done (Phase 4):* Gateway key `WorkspaceID` & `ProjectID` pinning prevents cross-client header spoofing (`tenant.go`); `AdmissionController` enforces Org + Workspace + Project quotas + daily request limits and fails closed; `BudgetManager.Reserve` provides single-replica atomic concurrency hold.
  - *Remaining P0/P1 Gaps:* `budgets` table/repo is still `user_id`-scoped (not `workspace_id`/`project_id`); zero API/UI CRUD for creating workspaces & projects after onboarding; distributed multi-replica spend reservation; UI lacks budget edit / FinOps alerts pages (`finops/anomalies`, `finops/budget-alerts`).
- **Success Metrics:** Zero accidental budget overruns beyond reservation buffer; <5 minutes to configure client budget caps; >99% attribution accuracy on routed requests.

### Use Case 2 — Client-Isolated Credential Governance (**Priority: P0**)
- **User Journey:** Agency stores each client's BYOK OpenAI/Anthropic/Google/MCP keys centrally in Prism encrypted with AES-256-GCM, scopes gateway keys and routing exclusively to that client's organization/workspace, rotates keys without redeploying workflows, and inspects credential health & access logs.
- **Required Backend Capabilities:** AES-256-GCM key storage + rotation CLI (`cmd/reencrypt`), org/workspace credential scoping, SSRF-guarded egress, masked reads on secrets, audit logging on credential/MCP mutations.
- **Required Frontend Workflows:** Scoped provider/credential views per client workspace, clear cooldown/health indicators, secret rotation/reveal controls gated by RBAC.
- **Current Readiness & Honest Gaps:**
  - *Done:* AES-256-GCM encryption + `cmd/reencrypt` key migration tool; `GET /api/mcp/servers/:id` secret headers now masked in Phase 4 (`handlers/mcp.go`); `ModelHandler` cross-tenant access closed (`handlers/model.go`).
  - *Remaining P0/P1 Gaps:* `credentials.org_id` mismatch with `router.go` passing `userID`; `CountActiveByProviderID` checks globally across tenants (`gateway_key.go`, `credential.go`); `POST /providers/:id/credentials/:credId/reveal` UX & audit policy need strict scoping.

### Use Case 3 — Agent Permission & Execution Governance (**Priority: P0**)
- **User Journey:** Agency registers an agent per client workflow (`support-triage`, `invoice-extractor`), binds its allowed models, allowed/denied MCP servers & tools (`!delete_customer`), allowed resources, and monthly `MaxBudgetCents`. Client runtime passes gateway key + agent identity (or calls `POST /v1/mcp`), and unauthorized model/tool calls are blocked with `403`/`-32001` and written to immutable audit trails.
- **Required Backend Capabilities:** Registered agent identities (ideally bound to dedicated `GatewayAPIKey.AgentID`), consistent enforcement across `/v1/chat/completions`, `prism-auto`, `/v1/tools/*`, `/v1/resources/*`, and `/v1/mcp`, plus approval queues for sensitive actions.
- **Required Frontend Workflows:** Agent form exposing `AllowedModels`, `SystemPromptOverride`, `AllowedMCPServers`, `AllowedTools`/`!Deny`, `AllowedResources`, `MaxBudgetCents`, and `Enabled` (several fields currently missing from `AgentFormDialog.tsx`!); Resource backend form (currently hardcodes `backends: []`!); human approval inbox (not built).
- **Current Readiness & Honest Gaps:**
  - *Done (Phase 4):* `AdmissionController` enforces `!deny`, `AllowedResources`, `AllowedMCPServers`, `MaxBudgetCents`; `prism-auto` filters candidates via `CheckAgentResolvedModel`; `/v1/mcp` exposes governed tools + `delegate_ai_task`; `ai_audit_trails` cross-tenant `user_admin` leak closed.
  - *Remaining P0/P1 Gaps:* Agent identity is still opt-in header (`GatewayAPIKey` needs mandatory/optional `AgentID` binding + `X-Paperclip-Agent-ID` middleware fix); **0% approval workflow** implemented; `AgentFormDialog` & `ResourceFormDialog` frontend parity incomplete.

### Use Case 4 — Intelligent Model Routing & Cost Optimization (**Priority: P1**)
- **User Journey:** Agency configures a routing policy (`cost-saver` vs `quality-first`), routes classification/extraction subtasks via `model: "prism-auto"` or `delegate_ai_task`, replays historical traffic against candidate policies before rollout, and inspects per-decision explanations & actual cost savings.
- **Required Backend Capabilities:** Deterministic candidate filtering + explainable scoring, live `X-Routing-Policy`, offline evaluation & historical replay engine, accurate TTFT/latency/cost tracking.
- **Required Frontend Workflows:** Routing rules UI (missing!), routing decision trace explorer (backend endpoint exists, UI missing!), accurate `/playground` simulator matching backend DTOs.
- **Current Readiness & Honest Gaps:**
  - *Done:* Smart Router classifier/scorer/health integration exists and Phase 4 unlocked `X-Routing-Policy` on `/v1/chat/completions` + agent model filtering + `delegate_ai_task`.
  - *Remaining P1 Gaps:* **No routing evaluation/replay framework**; `/api/routing/simulate` diverges from live routing on 7 dimensions & its JSON keys mismatch `/playground` UI (producing 0s in UI); heuristic classifier misclassifies short complex prompts as low complexity; `TTFTMs` not persisted on stream logs.

### Use Case 5 — White-Label AI Operations & Client Reporting (**Priority: P1/P3**)
- **User Journey:** Agency exports monthly per-client usage, cost, error rate, and agent execution summaries (CSV/JSON/PDF or branded read-only client portal) with verified cost attribution.
- **Required Backend Capabilities:** Org/Workspace/Project-aggregated analytics, export endpoints with formulas/injection hardening, optional custom branding & read-only client role.
- **Required Frontend Workflows:** Date-range/workspace-filtered reports, CSV/JSON download buttons for inference + MCP + audit logs, client-facing read-only view.
- **Current Readiness & Honest Gaps:**
  - *Done:* `RequestLogRepository.GetLogAnalytics`, admin audit CSV/JSON export with formula neutralization (`handlers/audit_trail.go`), `dashboard` overview KPI cards & charts wired to real queries.
  - *Remaining P1/P3 Gaps:* `MeteringService` is dead hardcoded code; `Logs` UI hardcodes 10-row fetch; no per-workspace usage export or agency white-label branding exists yet.

---

## 3. Product Prioritization

Do **not** build white-label portals, billing/Stripe checkout, or complex multi-agent marketplaces first. Execute strictly in order:
1. **P0:** Security & Tenant/Client Isolation + Fail-Closed Controls + Budget/Quota Enforcement + Mandatory Agent Execution Boundaries.
2. **P1:** Anthropic `/v1/messages` + Adapter Correctness + Inbound MCP Hardening + Agent/Approval Workflows + Routing Replay & Accurate Simulator + Frontend Parity + E2E Playwright Suite.
3. **P2:** Operational Maturity (Backup/Restore CI drills, Blue-Green alias isolation, Real Load/Chaos suite against live Gateway, Tempo/Alertmanager completion).
4. **P3:** Commercial Agency Packaging (Workspace/Project/Client UX polish, Client Export Reports, White-Label Branding, Billing Activation).

---

## 4. Lightweight Agency PMF Validation Plan (Validation Targets — Not Existing Traction)

Engineering completion proves technical feasibility, **not commercial demand**. Run this disciplined validation loop before expanding into P3 commercial surface area:

| Validation Target | Metric / Artifact | Decision Gate (Pass / Pivot / Drop) |
|---|---|---|
| **15–20 Customer Discovery Interviews** | Structured notes with technical founders/CTOs of 5–30-client AI agencies | Confirm >=60% cite at least 2 of: scattered client keys, surprise client bill/runaway loop, manual client cost attribution |
| **3–5 Design Partners** | Signed pilot agreements (even unpaid/discounted early access) | Confirm agencies are willing to route real staging traffic through a self-hosted or hosted Prism instance |
| **>=1 Real Agency Production/Staging Workflow** | Live workflow (e.g., n8n/LangGraph support triage or Claude Code/Cursor/MCP coding workflow) running >=7 days through `/v1` or `/v1/mcp` | Verify latency overhead <50ms p95, zero dropped billing logs, 100% policy enforcement fidelity |
| **>=2 Client Environments in Pilot** | 2 isolated workspaces/keys in one agency account with distinct budgets & BYOK keys | Validate zero cross-client leakage and UX clarity without engineering handholding |
| **Explicit Willingness-to-Pay (WTP) Test** | Pricing commitment check (\$49–\$199/mo or per-active-client tier) for P0+P1 capabilities | Identify which exact feature unlocks budget approval (Cost Cap vs BYOK Isolation vs Agent Governance vs Client Reports) |

### Measurable Pilot Success Signals
1. **Ops Time Saved:** Hours/month saved managing client keys and monthly client cost rollups.
2. **Manual Tracking Reduction:** Elimination of external spreadsheet cost reconciliation for pilot clients.
3. **Clients Managed per Operator:** Agency operator onboarding time for client #N+1 (<10 minutes target).
4. **Budget/Policy Violations Prevented:** Count of real hard-cap denials or blocked unauthorized tool/model attempts caught in pilot.
5. **Measured Model Cost Variance:** Verified `actual_cost_usd` difference on `prism-auto` / `delegate_ai_task` vs baseline flagship pinning, with operator quality sign-off.
