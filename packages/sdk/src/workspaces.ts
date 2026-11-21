import { HttpClient } from "./client.js";
import { Workspace, Project } from "./types.js";

export class WorkspacesModule {
  constructor(private client: HttpClient) {}

  public async list(): Promise<Workspace[]> {
    const res = await this.client.request<{ data?: Workspace[] } | Workspace[]>("/api/workspaces");
    return Array.isArray(res) ? res : (res.data ?? []);
  }

  public async create(data: { name: string; slug?: string }): Promise<Workspace> {
    return this.client.request<Workspace>("/api/workspaces", {
      method: "POST",
      body: data,
    });
  }

  public async update(workspaceId: string, data: { name: string; slug?: string }): Promise<Workspace> {
    return this.client.request<Workspace>(`/api/workspaces/${workspaceId}`, {
      method: "PUT",
      body: data,
    });
  }

  public async delete(workspaceId: string): Promise<void> {
    await this.client.request(`/api/workspaces/${workspaceId}`, {
      method: "DELETE",
    });
  }

  public async listProjects(workspaceId: string): Promise<Project[]> {
    const res = await this.client.request<{ data?: Project[] } | Project[]>(`/api/workspaces/${workspaceId}/projects`);
    return Array.isArray(res) ? res : (res.data ?? []);
  }

  public async createProject(workspaceId: string, data: { name: string; slug?: string }): Promise<Project> {
    return this.client.request<Project>(`/api/workspaces/${workspaceId}/projects`, {
      method: "POST",
      body: data,
    });
  }

  public async updateProject(workspaceId: string, projectId: string, data: { name: string; slug?: string }): Promise<Project> {
    return this.client.request<Project>(`/api/workspaces/${workspaceId}/projects/${projectId}`, {
      method: "PUT",
      body: data,
    });
  }

  public async deleteProject(workspaceId: string, projectId: string): Promise<void> {
    await this.client.request(`/api/workspaces/${workspaceId}/projects/${projectId}`, {
      method: "DELETE",
    });
  }
}
