import { HttpClient } from "./client.js";
import {
  MCPServer,
  MCPTool,
  MCPServerWithTools,
  MCPServerEdit,
  CreateMCPServerRequest,
  MCPServerStats,
  MCPJSONRPCResponse,
  MCPRegistryCatalogItem,
} from "./types.js";

export class MCPModule {
  constructor(private client: HttpClient) {}

  public async listServers(): Promise<MCPServer[]> {
    return this.client.request<MCPServer[]>("/api/mcp/servers");
  }

  public async getServer(serverId: string): Promise<MCPServerEdit> {
    return this.client.request<MCPServerEdit>(`/api/mcp/servers/${serverId}`);
  }

  public async createServer(data: CreateMCPServerRequest): Promise<MCPServerWithTools> {
    return this.client.request<MCPServerWithTools>("/api/mcp/servers", {
      method: "POST",
      body: data,
    });
  }

  public async updateServer(serverId: string, data: Partial<CreateMCPServerRequest>): Promise<MCPServerWithTools> {
    return this.client.request<MCPServerWithTools>(`/api/mcp/servers/${serverId}`, {
      method: "PUT",
      body: data,
    });
  }

  public async deleteServer(serverId: string): Promise<void> {
    await this.client.request(`/api/mcp/servers/${serverId}`, {
      method: "DELETE",
    });
  }

  public async sync(serverId: string): Promise<MCPServerWithTools> {
    return this.client.request<MCPServerWithTools>(`/api/mcp/servers/${serverId}/sync`, {
      method: "POST",
    });
  }

  public async getTools(serverId: string): Promise<MCPTool[]> {
    return this.client.request<MCPTool[]>(`/api/mcp/servers/${serverId}/tools`);
  }

  public async getServerStats(serverId: string, days?: number): Promise<MCPServerStats> {
    const params = days && days > 0 ? `?days=${days}` : "";
    const res = await this.client.request<{ data: MCPServerStats }>(`/api/mcp/servers/${serverId}/stats${params}`);
    return res.data;
  }

  /**
   * Lists governed tools exposed on the inbound MCP JSON-RPC 2.0 gateway (POST /v1/mcp).
   */
  public async listGatewayTools(id: string | number = 1): Promise<MCPJSONRPCResponse<{ tools: Array<{ name: string; description?: string; inputSchema?: Record<string, unknown> }> }>> {
    return this.client.request("/v1/mcp", {
      method: "POST",
      body: { jsonrpc: "2.0", id, method: "tools/list", params: {} },
    });
  }

  /**
   * Executes a governed tool (or delegate_ai_task) through POST /v1/mcp.
   */
  public async callGatewayTool(
    name: string,
    args: Record<string, unknown>,
    options: { approvalId?: string; id?: string | number } = {}
  ): Promise<MCPJSONRPCResponse> {
    return this.client.request("/v1/mcp", {
      method: "POST",
      headers: options.approvalId ? { "X-Prism-Approval-ID": options.approvalId } : undefined,
      body: {
        jsonrpc: "2.0",
        id: options.id ?? 1,
        method: "tools/call",
        params: {
          name,
          arguments: options.approvalId ? { ...args, _approval_id: options.approvalId } : args,
        },
      },
    });
  }

  /**
   * Delegates a subtask through the built-in delegate_ai_task tool on POST /v1/mcp.
   */
  public async delegateTask(params: {
    prompt: string;
    taskType?: string;
    model?: string;
    approvalId?: string;
  }): Promise<MCPJSONRPCResponse> {
    return this.callGatewayTool(
      "delegate_ai_task",
      {
        prompt: params.prompt,
        ...(params.taskType ? { task_type: params.taskType } : {}),
        ...(params.model ? { model: params.model } : {}),
      },
      { approvalId: params.approvalId }
    );
  }

  public async listRegistryCatalog(): Promise<MCPRegistryCatalogItem[]> {
    const res = await this.client.request<{ data?: MCPRegistryCatalogItem[] } | MCPRegistryCatalogItem[]>("/api/mcp/registry");
    return Array.isArray(res) ? res : (res.data ?? []);
  }

  public async registerFromCatalog(data: { slug?: string; name?: string; serverUrl: string; authToken?: string }): Promise<MCPServerWithTools> {
    return this.client.request<MCPServerWithTools>("/api/mcp/registry", {
      method: "POST",
      body: data,
    });
  }
}
