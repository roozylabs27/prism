# Prism — AI Automation Agency Discovery Playbook (Recommendation 1)

> **Validation Status Notice:** This is an **operational execution toolkit** prepared for the founder/commercial team. **Zero customer interviews, pilot runs, or traction metrics have been executed or fabricated here.** Every claim made to prospective agencies must stay strictly within Prism's currently verified capabilities ([unified-ai-control-plane.md](file:///c:/me/roozylabs/projects/prism/docs/product/unified-ai-control-plane.md)).

**Target Cohort:** **15–20 structured discovery interviews** with technical founders, CTOs, or Heads of AI Engineering at **AI Automation Agencies** managing **5–30 active client accounts**.

---

## 1. ICP Screening Criteria

Prioritize interviewees who satisfy at least 3 of the following:
- **Account Scale:** Agency manages **5–30 active production/staging AI workflows, chatbots, or custom agents** across distinct client companies.
- **Stack Profile:** Uses **OpenAI-compatible APIs / SDKs** (OpenAI SDK, LangChain, LangGraph, CrewAI, n8n, custom Node/Python backends), and/or **MCP-connected tooling** (Claude Desktop, Cursor MCP, Claude Code MCP, custom MCP servers).
- **Cost/Credential Pain Ownership:** Personally deals with client OpenAI/Anthropic/Google API keys, client billing pass-throughs, token cost spikes, or agent guardrail incidents.
- **Exclude for Initial Cohort:** Pure single-prompt no-code Zapier marketers (<3 clients) or Fortune 500 enterprises demanding full SOC2/SAML/SCIM before any pilot.

---

## 2. Honest Outreach Templates

### Template A — Cold Founder DM (LinkedIn / Slack / Discord / X)
> Hey {{Name}} — saw your team builds {{n8n / LangGraph / custom AI agents}} for multiple clients. Quick non-sales question: how are you currently isolating client API keys and preventing one runaway workflow from burning a client's monthly OpenAI/Anthropic budget?
>
> We're interviewing 15–20 AI automation agency builders (5–30 clients) as we harden Prism (an open-architecture AI control plane we're testing in staging). No pitch deck — would you spare 20 mins to share how you handle multi-client costs & agent guardrails today? Happy to share anonymized benchmark findings afterward.

### Template B — Short Cold Email
> **Subject:** Quick question on managing AI keys & spend across {{AgencyName}}’s clients
>
> Hi {{Name}},
>
> Running AI workflows across 5–30 client accounts usually hits two headaches: vendor keys (`sk-*`) scattered across environments, and manual CSV reconciliation to tell each client what their bots actually spent.
>
> We're building **Prism** (an AI gateway + MCP governance layer — early staging, not a turnkey white-label suite yet) and speaking with 15–20 technical agency leads to pressure-test whether per-client budget caps, BYOK isolation, and tool-level agent allowlists actually solve a top-3 operational problem for you.
>
> Open to a candid 25-minute chat this week? Zero sales pitch; we'll send back our anonymized 20-agency summary once finished.

### Template C — Warm Intro Blurb (Forwardable)
> Intro'ing you to {{YourName}}, who is building Prism (an infrastructure/governance layer behind OpenAI-compatible agent stacks and MCP tools). Before finishing their commercial roadmap for 5–30 client AI agencies, they're doing 20 reality-check calls on multi-client credential isolation, runaway spend prevention, and client cost reporting. Worth 20 minutes if you've dealt with multi-client LLM ops pain.

### What NOT to Claim During Discovery
- **Do NOT claim** unattended multi-tenant SaaS compliance (P0-5/P0-6/P0-7 hardening remains on the pre-production roadmap).
- **Do NOT claim** white-label branded client login portals or automated Stripe rebilling (P3, intentionally gated until validation passes).
- **Do NOT claim** native Claude Code `ANTHROPIC_BASE_URL` inference routing yet (`POST /v1/messages` is P1-2 open).

---

## 3. 30-Minute Structured Mom-Test Interview Script

Keep 80% of the call in the past tense (*"Tell me about the last time..."*) rather than pitching Prism features.

### Block 1 — Context & Stack (Minutes 0–4)
1. How many active client accounts currently run live or staging AI workflows at {{AgencyName}}?
2. Which frameworks/apps do you ship most? *(OpenAI/Anthropic SDKs, LangGraph/CrewAI, n8n/Make, Cursor/Claude Code, MCP servers?)*

### Block 2 — Fragmented Spend & Client Cost Attribution (Minutes 4–10)
3. Walk me through how you currently pay for LLM tokens across clients: do clients pay you a flat retainer, pass-through BYOK, or marked-up usage?
4. Last month, how did you calculate exactly how much AI spend Client X generated vs Client Y?
   - *Follow-up:* Roughly how many hours/month does your team spend pulling invoices/spreadsheets?
   - *Rate Pain 0–3:* (`0 = none`, `1 = mild annoyance`, `2 = recurring ops tax`, `3 = acute margin/churn risk`)

### Block 3 — Credential Custody & BYOK Isolation (Minutes 10–15)
5. Where do your clients' OpenAI, Anthropic, Google, or MCP API keys live right now? *(separate orgs, `.env` files, n8n credentials, contractor repos?)*
6. Tell me about the last time a developer left, a contractor rotated off, or a key was exposed/shared across two client projects. How long did revocation take?
   - *Rate Pain 0–3.*

### Block 4 — Budget Overruns & Agent Permission Drift (Minutes 15–21)
7. Have you ever had a retry loop, prompt bug, or bot script blow past an expected spend limit or touch the wrong DB/API endpoint? What happened financially and with the client?
8. Today, if you want an agent to only use specific cheap models or forbid destructive MCP/API tools (e.g., `!delete_customer`), where is that enforced?
   - *Rate Pain 0–3.*

### Block 5 — Model Routing & Cost Optimization (Minutes 21–25)
9. When switching a workflow from a flagship model (GPT-4o / Claude Sonnet / Opus) to a smaller model, how do you test whether quality/latency stayed acceptable? Or do you mostly stick to flagship models to stay safe?

### Block 6 — Commercial & Willingness-to-Pay (WTP) Reality Check (Minutes 25–30)
*(Introduce Prism's concrete scope ONLY in the final 5 minutes.)*
10. **"If you could put a single staging gateway in front of your OpenAI-compatible client bots + MCP tools that gives each client its own workspace key, hard monthly/daily budget caps, isolated BYOK credentials, and one-click client CSV spend/savings export:"**
    - Would that remove a top-3 bottleneck today, or is it a nice-to-have? Why?
    - Which tier makes sense for your current {{N}} active clients: **\$49/mo (up to 5 client workspaces)**, **\$129/mo (up to 15)**, or **\$199/mo (up to 30)** — or would you expect per-client pass-through pricing?
    - What single missing capability would stop you from testing this with 2 client workflows in staging next week? *(Watch closely if `/v1/messages` for Claude Code, approval workflows, or white-label reports shows up unprompted.)*

---

## 4. Quantitative Go / Pivot / Drop Decision Matrix

Score all 15–20 completed interviews inside [agency-discovery-tracker.csv](file:///c:/me/roozylabs/projects/prism/docs/commercial/templates/agency-discovery-tracker.csv).

| Criterion | Pass (Proceed to Design Partner Pilots) | Pivot Signal | Drop / Pause Signal |
|---|---|---|---|
| **Multi-Pain Validation Rate** | **>= 60%** (e.g., >=10 of 16) rate **>= 2** of the 4 core pains (`byok_keys`, `budget_caps`, `agent_permissions`, `client_reporting`) at **>= 2/3** | >=60% validate only 1 narrow pain (e.g. only MCP tools or only reporting -> narrow product surface) | < 40% experience >=2 pains at severity >=2 |
| **Manual Tracking Baseline** | Median **>= 3 hours/month** spent reconciling client AI costs/keys or >=1 painful incident in past 6 months | < 1 hour/month on reporting, but acute credential/agent safety pain | Clients all put their own single-card keys directly into native SaaS tools and never ask agency to operate |
| **Staging Pilot Pull** | **>= 3–5 agencies** commit to test **>= 2 client workspaces** on staging within 14 days | Agencies want self-hosted Docker only with specific missing connector (e.g. Claude Code `/v1/messages`) before pilot | Everyone says *"looks cool, ping me in 6 months"* |
| **Willingness to Pay (WTP)** | **>= 40%** verbally commit that **\$49–\$199/mo** fits current agency opex once proven in pilot | Willing to pay only per-client pass-through fee (`< \$15/client/mo`) or demanding white-label client portal first | Expect free open-source only with zero paid control-plane budget |
