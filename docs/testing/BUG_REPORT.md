# Prism — Smoke Test Bug Report

**Test Phase:** Remediation Applied (`DEF-001`–`DEF-014`) — Pending Stack Rebuild, Migration `122`, & Live Re-Verification  
**Environment:** Local Development (`http://localhost:3000` / `http://localhost:8080`)  
**Last Updated:** 2026-11-22 (After Bug Fix Remediation)

---

## Defect Summary

| ID | Severity | Status | Feature | Affected Role(s) | Summary | Fix Files / Migration |
|---|---|---|---|---|---|---|
| **DEF-001** | **P1** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Budgets (`/budgets`) | Owner, FinOps Manager | Budget creation completely broken in UI due to percentage (`80`/`95`) vs fraction (`0..1`) mismatch + HTML5 `step="1"` | `BudgetFormSheet.tsx`, `budget.schema.ts`, `budgets/page.tsx`, `BudgetFormSheet.test.tsx` |
| **DEF-009** | **P1** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Member Invitations (`/accept-invite`) | Invited Users (All Roles) | Opening a freshly generated invitation link immediately fails with HTTP `404 INVITE_NOT_FOUND` (`PreviewInvite`/`AcceptInvite` missing RLS bypass) | `apps/api/internal/repository/rbac.go`, `rbac_invite_rls_test.go` |
| **DEF-002** | **P2** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Providers (`/providers`) | Owner | Creating a custom provider (`type: "custom"`) returns HTTP `500 PROVIDER_CREATE_FAILED` | `122_allow_custom_provider_type.{up,down}.sql`, `provider.go`, `provider_helpers.go`, `provider_custom_test.go` |
| **DEF-003** | **P2** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Organization Settings (`/settings/organization`) | Owner (all roles) | React Hydration Error `#418` thrown in browser console on `/settings/organization` | `TenantSelector.tsx`, `WorkspacesProjectsSection.tsx`, `TenantSelector.test.tsx` |
| **DEF-010** | **P2** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Workspace Switcher (Topbar / `GET /api/workspaces`) | Developer, Agent Manager, FinOps Manager (all non-Owner roles) | Selecting an unauthorized workspace traps the user in `403` across all pages and hides the Workspace Selector (`GET /api/workspaces` & `/user/permissions` fail with `403`) until sign-out | `workspace.go` (repo & handler), `main.go`, `workspace_test.go`, `client.ts`, `TenantSelector.tsx`, `client.test.ts` |
| **DEF-011** | **P2** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Dashboard Health & Global Status Indicator (`/api/dashboard/health`) | Agent Manager, FinOps Manager (and likely Auditor) | `GET /api/dashboard/health` requires `model:read`, returning `403` on every route and falsely showing `"System degraded"` in the global topbar | `useDashboardQuery.ts`, `DashboardTopbar.tsx`, `app/(dashboard)/page.tsx`, `DashboardOperationalBreakdown.tsx` |
| **DEF-012** | **P2** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Providers UI Permission Guard (`/providers`) | Developer | `Connect Custom Provider` button is enabled for Developer because `types/roles.ts` aliases `model:create` to `model:update`; submitting fails with backend `403` | `apps/console/types/roles.ts`, `roles.test.ts` |
| **DEF-004** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Onboarding / SSE (`/onboarding`) | New Owner | `GET /api/sse` loops with HTTP `403` (×7) and console warnings before organization creation completes | `SSEContext.tsx`, `lib/api/types/auth.ts`, `SSEContext.test.tsx` |
| **DEF-005** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Auth Middleware (`/favicon.svg`) | Unauthenticated | `GET /favicon.svg` returns `307` redirect to `/signin` (middleware only excludes `favicon.ico`) | `apps/console/middleware.ts`, `middleware.test.ts` |
| **DEF-006** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Member Invitations (`/settings/members`) | Owner | One-time invite link modal (`SecretReveal`) displays `"Copy Key"` and `"Please copy your API key now..."` copy | `SecretReveal.tsx`, `InviteMemberSheet.tsx`, `SecretReveal.test.tsx` |
| **DEF-007** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Routing Simulator (`/playground`) | All roles | Toast displays `"Dry-run simulation completed! Selected candidate:"` with blank candidate when 0 candidates exist | `apps/console/app/(dashboard)/playground/page.tsx` |
| **DEF-008** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | RBAC / Role Inventory | All roles | `admin` and `member` roles exist in frontend `UserRole` enum & `/governance` subtitle, but are not seeded in PostgreSQL `roles` table | `roles.ts`, `roles.test.ts`, `governance/page.tsx`, `DashboardTopbar.tsx` |
| **DEF-013** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Audit Trail Export UI (`/audit-trail`) | Developer (and roles without `audit:export`) | `Export` / `Export JSON` button is not guarded by `PermissionGuard`, failing with `403` and generic `"An unexpected API error occurred"` toast | `audit-trail/page.tsx`, `client.ts`, `client.test.ts` |
| **DEF-014** | **P3** | `FIXED (Unit-Tested — Pending Live Re-Verification)` | Budgets FinOps Tab (`/budgets`) | Developer (and roles without `finops:read`) | `403` on `GET /api/analytics/finops/*` silently renders normal empty-state copy (`"No Active Budget Alerts"`) instead of an access-denied message | `useBudgetsQuery.ts`, `budgets/page.tsx` |

> [!IMPORTANT]
> **Deployment Note Before Live Re-Verification & Stage 4:** Rebuild/restart the running API & Console services and ensure migration `122_allow_custom_provider_type.up.sql` is applied before re-running failed cases (`DEF-001`, `DEF-002`, `DEF-009`, `DEF-012`) in the browser.

---

## Detailed Defect Reports

### DEF-001 (P1) — Budget Creation Impossible via `/budgets` UI (Threshold Unit Mismatch)
- **Feature:** Budgets & Quotas (`/budgets` -> `POST /api/budgets`)
- **Affected Role:** Owner, FinOps Manager
- **Reproduction Steps:**
  1. Sign in as Owner and navigate to `/budgets`.
  2. Click `Create Budget Limit`.
  3. Fill in Name (`Smoke Monthly Budget`), Limit (`50`), keep default `Warning Threshold (%) = 80` and `Critical Threshold (%) = 95`, and submit.
  4. Observe HTTP 400 error.
  5. Attempt to enter `0.8` and `0.95` instead and submit.
- **Expected:** Budget is created (`201 Created`) and persisted. Frontend either converts `%` to fraction (`0.8`/`0.95`) before sending or accepts decimal fractions.
- **Actual:**
  - Submitting `80`/`95` returns HTTP `400`: `"Failed to create budget: criticalThreshold must be a fraction between 0 and 1, got 95"`.
  - Entering `0.8`/`0.95` is blocked by browser HTML5 validation (`step="1"` on `<input type="number" min="0" max="100">`: `"The two nearest valid values are 0 and 1"`).
- **Evidence:** `steps/453/media_0.png` (Stage 2 Pass 2)
- **Likely Component/Endpoint:** Budget creation form in `apps/console` (`/budgets`) & `POST /api/budgets` (`apps/api/internal/handlers/budget.go`).

### DEF-002 (P2) — Custom Provider Creation Returns HTTP 500
- **Feature:** Providers (`/providers` -> `POST /api/providers`)
- **Affected Role:** Owner
- **Reproduction Steps:**
  1. Sign in as Owner and navigate to `/providers`.
  2. Click `Add Provider`.
  3. Enter Name `Smoke Custom Provider`, Type `custom`, Base URL `https://example.com/v1`, Enabled `true`, and submit.
- **Expected:** Provider is saved (`201 Created`) and appears in the providers list.
- **Actual:** Returns HTTP `500`: `{"error":{"message":"Failed to save provider","type":"server_error","code":"PROVIDER_CREATE_FAILED"}}`.
- **Evidence:** `steps/245/media_0.png` (Stage 2 Pass 1)
- **Likely Component/Endpoint:** `apps/api/internal/handlers/provider.go` / `provider_custom_test.go` / provider repository SQL insert.

### DEF-003 (P2) — React Hydration Error #418 on `/settings/organization`
- **Feature:** Organization Settings & Workspaces (`/settings/organization`)
- **Affected Role:** Owner (likely all roles)
- **Reproduction Steps:**
  1. Navigate to `/settings/organization`, create/rename workspaces/projects, and reload the page.
  2. Inspect browser DevTools console.
- **Expected:** Zero uncaught React hydration errors.
- **Actual:** Browser console logs `Uncaught Error: Minified React error #418`.
- **Evidence:** Console capture during `SMK-WS-01..05` (`steps/170/media_0.png`).
- **Likely Component/Endpoint:** `apps/console/app/(dashboard)/settings/organization/page.tsx` or `TenantSelector` / `WorkspacesProjectsSection`.

### DEF-004 (P3) — SSE Connection `403` Retry Loop During `/onboarding`
- **Feature:** Onboarding & Real-Time SSE (`/onboarding` -> `GET /api/sse`)
- **Affected Role:** Newly signed-up Owner (pre-onboarding)
- **Reproduction Steps:**
  1. Sign up a new user and remain on `/onboarding` before completing org creation.
  2. Inspect Network & Console tabs.
- **Expected:** SSE client does not connect until an active organization exists.
- **Actual:** `GET /api/sse` fires repeatedly (7×) returning HTTP `403` and logging `[SSE Connection Error]` console warnings.
- **Evidence:** Stage 2 Pass 1 (`SMK-AUTH-01 / 02`).
- **Likely Component/Endpoint:** SSE hook/provider mounted globally in `apps/console`.

### DEF-005 (P3) — Unauthenticated `/favicon.svg` Redirects (`307`) to `/signin`
- **Feature:** Auth Middleware (`apps/console/middleware.ts`)
- **Affected Role:** Unauthenticated
- **Reproduction Steps:**
  1. Open `/signin` in a fresh unauthenticated browser context.
  2. Inspect Network tab for `/favicon.svg`.
- **Expected:** `200 OK` static asset response.
- **Actual:** `307 Temporary Redirect` to `/signin` because `middleware.ts` matcher/check only excludes `favicon.ico`, not `favicon.svg`.
- **Evidence:** Stage 2 Pass 1 (`SMK-AUTH-05`).

### DEF-006 (P3) — Invite Link Modal Reuses API Key Copy
- **Feature:** Member Invitations (`/settings/members` -> `InviteMemberSheet.tsx`)
- **Affected Role:** Owner
- **Reproduction Steps:**
  1. Invite a new member when SMTP is unset so the one-time link modal opens.
- **Expected:** Copy refers to the invitation link (`"Copy Link"`, `"Copy this invitation link now..."`).
- **Actual:** `SecretReveal` renders `"Copy Key"` and `"Please copy your API key now..."`.
- **Evidence:** Stage 2 Pass 1 (`SMK-ORG-05`).

### DEF-007 (P3) — Blank Candidate Name in Routing Simulator Toast
- **Feature:** Routing Simulator (`/playground`)
- **Affected Role:** All roles with `routing:read`
- **Reproduction Steps:**
  1. In an org with 0 provider credentials, run a dry-run simulation on `/playground`.
- **Expected:** Toast indicates no eligible candidates were found.
- **Actual:** Toast reads `"Dry-run simulation completed! Selected candidate:"` with nothing after the colon.
- **Evidence:** `steps/418/media_0.png` (Stage 2 Pass 2).

### DEF-008 (P3) — Unseeded `admin` and `member` Roles Referenced in Frontend
- **Feature:** RBAC & Governance (`/governance`, `apps/console/types/roles.ts`)
- **Affected Role:** All roles
- **Reproduction Steps:**
  1. Inspect `/settings/members` role dropdown vs `/governance` subtitle and `apps/console/types/roles.ts`.
- **Expected:** Frontend role enums and UI copy match backend seeded system roles.
- **Actual:** Backend only seeds 6 roles (`owner`, `developer`, `agent_manager`, `finops_manager`, `auditor`, `viewer`), while frontend enum defines `admin` (with a hardcoded `checkHasPermission` bypass) and `member`, and `/governance` subtitle lists `Admin`.
- **Evidence:** Stage 1 Discovery & Stage 2 (`SMK-ADM-01`, `SMK-GOV-01`).

### DEF-009 (P1) — Member Invitation Links Immediately Fail with `404 INVITE_NOT_FOUND`
- **Feature:** Member Invitations (`/accept-invite` -> `GET /api/auth/invites/:token`)
- **Affected Role:** Invited Users (all roles)
- **Reproduction Steps:**
  1. Sign in as Owner, navigate to `/settings/members`, and invite `smoke.dev.20261122@roozylabs.test` as `Developer` (`POST /api/organizations/invites [201]`).
  2. Copy the generated one-time invitation URL, sign out, and open `/accept-invite?token=<redacted>`.
- **Expected:** Invitation preview loads (`200 OK`) showing organization name (`Smoke Test Org A`), invited email, and role (`developer`), allowing signup/acceptance.
- **Actual:** `GET /api/auth/invites/:token` returns HTTP `404` `{"error":{"message":"This invitation is not valid any more","type":"not_found_error","code":"INVITE_NOT_FOUND"}}`, blocking team member onboarding completely.
- **Evidence:** `a6dae5ed-1675-412e-9366-f4366c2bc7d0/.system_generated/steps/717/media_0.png` (Stage 3 Pass 1)
- **Likely Component/Endpoint:** `RBACRepository.PreviewInvite` and `RBACRepository.AcceptInvite` in `apps/api/internal/repository/rbac.go` (missing `database.WithSystemBypassRLS(ctx)` when querying RLS-protected `member_invites` on public/user-scoped routes).

### DEF-010 (P2) — Workspace Selector Deadlock Trap When Selecting an Unauthorized Workspace
- **Feature:** Multi-Workspace Selector & Tenant Context (`GET /api/workspaces`, `GET /api/user/permissions`)
- **Affected Role:** Developer, Agent Manager, FinOps Manager (all non-Owner roles without `workspace:admin`)
- **Reproduction Steps:**
  1. Sign in as a non-Owner who belongs to one workspace (e.g., `dev.team@prism.local` in `Team Engineering Core`).
  2. Open the topbar Workspace Selector and pick a workspace the user is not a member of (`Team Finance & Ops`).
  3. Navigate to `/gateway-keys` or `/agents`.
- **Expected:** Either `GET /api/workspaces` only lists workspaces the user can access, OR selecting a forbidden workspace shows a clear error without breaking `GET /api/workspaces` and `GET /api/user/permissions` so the user can switch back.
- **Actual:** Backend enforces `403` on all workspace-scoped endpoints (security boundary holds), **but** the frontend also sends the forbidden `X-Prism-Workspace-ID` header on `GET /api/workspaces` and `GET /api/user/permissions`, causing both to return `403` and removing the Workspace Selector from the topbar — trapping the user in an unrecoverable `403` state until sign-out or localStorage reset.
- **Evidence:** `7e17387e-683e-4f30-99b3-52640c75ed95/.system_generated/steps/159/media_0.png` (Developer), `steps/357/media_0.png` (Agent Manager), `steps/495/media_0.png` (FinOps Manager).
- **Likely Component/Endpoint:** `TenantSelector` / HTTP client workspace header injection in `apps/console`, and `GET /api/workspaces` scoping in `apps/api/internal/handlers/workspace.go`.

### DEF-011 (P2) — Global `"System degraded"` False Alarm Caused by `GET /api/dashboard/health` Requiring `model:read`
- **Feature:** Dashboard Overview & Global Topbar Status (`/api/dashboard/health`)
- **Affected Role:** Agent Manager, FinOps Manager (and any role without `model:read`)
- **Reproduction Steps:**
  1. Sign in as `agent.team@prism.local` or `finops.team@prism.local` and land on `/`.
  2. Inspect Network tab and topbar health badge.
- **Expected:** Global health indicator works without `model:read` or hides gracefully when unauthorized, without firing `403` on every page navigation.
- **Actual:** `GET /api/dashboard/health` returns `403` on every route, causing the global topbar indicator to display `"System degraded"` and Dashboard Provider Availability to display `"No providers configured"`.
- **Evidence:** `7e17387e-683e-4f30-99b3-52640c75ed95/.system_generated/steps/195/media_0.png` (Agent Manager).
- **Likely Component/Endpoint:** `apps/api/cmd/server/main.go` line 486 (`GET /dashboard/health` guarded by `model:read`) & global status indicator in `apps/console`.

### DEF-012 (P2) — `/providers` `Connect Custom Provider` Button Enabled for Developer Due to Permission Alias Bug
- **Feature:** Providers (`/providers`)
- **Affected Role:** Developer
- **Reproduction Steps:**
  1. Sign in as `dev.team@prism.local` (has `model:update` but NOT `model:create`).
  2. Navigate to `/providers`, click `Connect Custom Provider`, fill in provider fields, and submit.
- **Expected:** `Connect Custom Provider` button is disabled (`PermissionGuard` checks `model:create`).
- **Actual:** Button is enabled because `apps/console/types/roles.ts` line 32 defines `'model:create': ['models:write', 'model:update']`, falsely granting UI write access to holders of `model:update`. Backend correctly blocks `POST /api/providers` with `403 Forbidden: insufficient permissions for action model:create`.
- **Evidence:** `7e17387e-683e-4f30-99b3-52640c75ed95/.system_generated/steps/132/media_0.png`.
- **Likely Component/Endpoint:** `PERMISSION_ALIASES` in `apps/console/types/roles.ts`.

### DEF-013 (P3) — `/audit-trail` Export Button Unguarded in UI for Roles Without `audit:export`
- **Feature:** Audit Trail (`/audit-trail`)
- **Affected Role:** Developer (and other roles lacking `audit:export`)
- **Reproduction Steps:**
  1. Sign in as `dev.team@prism.local`, open `/audit-trail`, click `Export` -> `Export JSON`.
- **Expected:** Export action is disabled with permission tooltip (or displays a clear permission-denied message).
- **Actual:** Button is enabled; clicking triggers `GET /api/audit-trail/export [403]` and displays generic toast `"Failed to export audit logs: An unexpected API error occurred"`.
- **Evidence:** Stage 3 (`SMK-DEV-02b`).

### DEF-014 (P3) — `/budgets` FinOps Alerts & Anomalies Silently Shows Empty State on `403`
- **Feature:** Budgets & FinOps (`/budgets`)
- **Affected Role:** Developer (and roles without `finops:read`)
- **Reproduction Steps:**
  1. Sign in as `dev.team@prism.local`, open `/budgets`, click `FinOps Alerts & Anomalies`.
- **Expected:** Displays permission-denied / error state when `GET /api/analytics/finops/*` returns `403`.
- **Actual:** Silently displays `"No Active Budget Alerts"` / `"No Cost Anomalies Detected"`, masking the `403` failure.
- **Evidence:** Stage 3 (`SMK-DEV-03d`).
