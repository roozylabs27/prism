import test from "node:test";
import assert from "node:assert/strict";
import { Prism } from "../dist/index.mjs";

test("Prism SDK modules send canonical paths, methods, and tenant headers", async () => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (url, init = {}) => {
    calls.push({ url: String(url), method: init.method ?? "GET", headers: init.headers ?? {}, body: init.body });
    return new Response(JSON.stringify({ data: [{ id: "item_1", name: "Test" }] }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  };

  try {
    const client = new Prism({
      apiKey: "prism_sk_test_key",
      baseURL: "http://localhost:8080",
      orgId: "org_123",
      workspaceId: "ws_456",
      projectId: "proj_789",
    });

    const agents = await client.agents.list();
    assert.equal(agents.length, 1);

    await client.workspaces.list();
    await client.workspaces.listProjects("ws_456");
    await client.analytics.getPilotOutcomes({ days: 30, workspaceId: "ws_456" });
    await client.analytics.listBudgets();
    await client.analytics.listQuotas();
    await client.analytics.listApprovals("pending");

    const paths = calls.map((c) => `${c.method} ${new URL(c.url).pathname}`);
    assert.deepEqual(paths, [
      "GET /api/agents",
      "GET /api/workspaces",
      "GET /api/workspaces/ws_456/projects",
      "GET /api/analytics/pilot-outcomes",
      "GET /api/budgets",
      "GET /api/quotas",
      "GET /api/approvals",
    ]);

    for (const call of calls) {
      const headers = new Headers(call.headers);
      assert.equal(headers.get("authorization"), "Bearer prism_sk_test_key");
      assert.equal(headers.get("x-prism-org-id"), "org_123");
      assert.equal(headers.get("x-prism-workspace-id"), "ws_456");
      assert.equal(headers.get("x-prism-project-id"), "proj_789");
    }
  } finally {
    globalThis.fetch = originalFetch;
  }
});
