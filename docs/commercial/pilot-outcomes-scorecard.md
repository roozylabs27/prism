# Prism — Pilot Outcomes Scorecard & Willingness-to-Pay (WTP) Validation Protocol (Recommendation 3)

> **Validation Status Notice:** This scorecard defines the measurement protocol and newly added API export endpoints (`GET /api/analytics/pilot-outcomes`, `GET /api/analytics/pilot-outcomes/export`) to evaluate upcoming design-partner pilots. **No real pilot metrics or commercial conversions have been recorded yet.**

---

## 1. The Four Required Hard Pilot Signals

Collect all four signals at Day 0 (baseline), Day 7, and Day 14 per design partner. Do **not** rely on qualitative compliments alone.

### Signal 1 — Agency Ops Hours / Month Saved & Spreadsheet Elimination
- **Day-0 Baseline Question:** *"Last full calendar month, how many minutes/hours did your team spend collecting vendor invoices, rotating scattered keys, and assembling client AI spend reports?"*
- **Day-14 Verification Artifact:** Run 1-click export via `GET /api/analytics/pilot-outcomes/export?format=csv&days=14`. Confirm whether the exported CSV replaces manual per-client spreadsheet rollups.
- **Target Signal:** `>= 50%` reduction in manual reporting/key-management hours and zero external client cost reconciliation spreadsheets needed during the pilot window.

### Signal 2 — Client #N+1 Onboarding Speed (`< 10 Minutes` Target)
- **Measurement:** Stopwatch from starting Client #2 setup via [onboard-client-workspace.sh](file:///c:/me/roozylabs/projects/prism/scripts/pilot/onboard-client-workspace.sh) (or `POST /api/workspaces` -> `/projects` -> `/quotas` -> `/gateway-keys`) through first `200 OK` attributed request in `request_logs`.
- **Target Signal:** **< 10 minutes** wall-clock (expected API setup <1 min + client workflow env paste <5 min) without custom engineering code edits.

### Signal 3 — Real Policy & Budget Violations Caught
- **Data Source:** `governance` object inside `GET /api/analytics/pilot-outcomes?days=14`:
  - `budgetQuotaDenials429`: count of hard-limit (`429`) blocks before upstream LLM charges accrued.
  - `policyDenials403`: count of disallowed models, denied tools (`!deny`), or cross-workspace spoofing (`403 TENANT_SECURITY_ERROR`) catches.
  - `mcpToolInvocations` & `delegatedTaskCalls`: auditable MCP boundary coverage.
- **Target Signal:** `100%` of injected safety tests + any unintentional client loops stopped deterministically prior to upstream vendor billing.

### Signal 4 — Measured `actualCostUsd`, Latency & Quality Delta on `prism-auto` / `delegate_ai_task`
- **Data Source:** `totals` & `routingBreakdown` inside `GET /api/analytics/pilot-outcomes/export?format=csv&days=14`.
- **Exact Fields Checked:**
  - `actualCostUsd`: logged cost under routed policy.
  - `baselineFlagshipCostUsd`: modeled cost benchmark (`\$3.00/1M` input, `\$15.00/1M` output).
  - `estimatedSavingsUsd`: modeled delta (`baselineFlagshipCostUsd - actualCostUsd`).
  - `avgLatencyMs` & `p95LatencyMs`: real latency impact per workspace and model.
- **Mandatory Operator Quality Sign-Off Gate:** Lower `actualCostUsd` counts as **FAILED** if accuracy/task completion regresses. For every routed workflow, sample >=20 requests/client and record operator quality grade (`Pass / Minor Regressions / Unacceptable`).

---

## 2. Explicit Willingness-to-Pay (WTP) Pricing Test Protocol

Run at the end of Day-14 readout **using the partner's own pilot numbers from `pilot-outcomes` CSV on screen**:

| Tier Tested | Monthly Price | Proposed Entitlement Anchor | What Actually Triggers Purchase? (Record Verbatim) |
|---|---|---|---|
| **Starter Agency** | **\$49 / mo** | Up to **5 client workspaces**, centralized BYOK isolation, hard budget caps, CSV cost attribution | *(e.g. Hard cap prevents one \$200 runaway loop)* |
| **Growth Agency** | **\$129 / mo** | Up to **15 client workspaces**, MCP Gateway tool governance, `delegate_ai_task`, anomaly tracking | *(e.g. Eliminates 4+ hrs/mo manual client billing)* |
| **Scale Agency** | **\$199 / mo** | Up to **30 client workspaces**, advanced routing policies, priority onboarding support | *(e.g. Team-wide shared org access — blocked until `P0-6`)* |

*(Also ask backup check: *"Would you instead prefer **\$10–\$15/active client/mo** pass-through pricing vs a flat **\$49–\$199/mo** agency tier?"*)*

---

## 3. Commercial Go / Hold Decision Gate After 3–5 Pilots

| Outcome Condition | Next Product Decision |
|---|---|
| `>= 3` of 5 pilots run >=7 days, `<10 min` client #2 onboarding passes, `>=2` partners convert or sign LOI at **\$49–\$199/mo** once P0-5/P0-6 + top requested P1 gap land | **Proceed** cleanly into P0-5/P0-6 + highest-voted P1 item (`P1-2` `/v1/messages` or `P1-4` approvals) and commercial billing (`P3-3`). |
| Pilots love BYOK isolation + budget caps on staging, **but refuse to pay** until white-label client-facing portal or Stripe rebilling ships | Re-score `P3-1`/`P3-2` timing after verifying paid LOI conditional on client portal. |
| `< 2` partners connect real staging traffic after 2 weeks or WTP stays `< \$29/mo` | **Stop building net-new features** and revisit ICP / positioning before investing further engineering cycles. |
