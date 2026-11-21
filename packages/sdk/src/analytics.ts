import { HttpClient } from "./client.js";
import { ApprovalRequest, BudgetSummary, PilotOutcomesSummary, TenantQuota } from "./types.js";

export class AnalyticsModule {
  constructor(private client: HttpClient) {}

  public async getPilotOutcomes(params?: { days?: number; workspaceId?: string }): Promise<PilotOutcomesSummary> {
    return this.client.request<PilotOutcomesSummary>("/api/analytics/pilot-outcomes", {
      query: params,
    });
  }

  public async listBudgets(): Promise<BudgetSummary[]> {
    const res = await this.client.request<{ data?: BudgetSummary[] } | BudgetSummary[]>("/api/budgets");
    return Array.isArray(res) ? res : (res.data ?? []);
  }

  public async listQuotas(): Promise<TenantQuota[]> {
    const res = await this.client.request<{ data?: TenantQuota[] } | TenantQuota[]>("/api/quotas");
    return Array.isArray(res) ? res : (res.data ?? []);
  }

  public async updateQuota(targetType: "organization" | "workspace" | "project", targetId: string, data: Partial<TenantQuota>): Promise<TenantQuota> {
    return this.client.request<TenantQuota>(`/api/quotas/${targetType}/${targetId}`, {
      method: "PUT",
      body: data,
    });
  }

  public async listLogs(params?: { page?: number; limit?: number; model?: string; status?: number }): Promise<unknown> {
    return this.client.request("/api/logs", { query: params });
  }

  public async listApprovals(status?: string): Promise<ApprovalRequest[]> {
    const res = await this.client.request<{ data?: ApprovalRequest[] } | ApprovalRequest[]>("/api/approvals", {
      query: status ? { status } : undefined,
    });
    return Array.isArray(res) ? res : (res.data ?? []);
  }
}
