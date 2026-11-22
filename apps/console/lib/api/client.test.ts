import { beforeEach, describe, expect, it } from 'vitest';
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios';
import { AxiosError } from 'axios';
import { api } from './client';
import { ApiError } from '@/lib/http/errors';
import { useTenantStore } from '@/stores/useTenantStore';

// Captures the config the interceptors produce and answers with a canned
// response (or a canned failure).
function stubAdapter(status = 200) {
  const seen: InternalAxiosRequestConfig[] = [];
  const adapter: AxiosAdapter = async (config) => {
    seen.push(config);
    const response = { data: { ok: true }, status, statusText: 'x', headers: {}, config };
    if (status >= 400) {
      throw new AxiosError('failed', String(status), config, null, response);
    }
    return response;
  };
  api.defaults.adapter = adapter;
  return seen;
}

describe('api client', () => {
  beforeEach(() => {
    localStorage.clear();
    useTenantStore.getState().resetTenant();
    window.history.replaceState({}, '', '/');
  });

  it('uses HttpOnly credentials and attaches selected tenant headers to every request', async () => {
    const seen = stubAdapter();
    useTenantStore.getState().setSelectedOrg('org_1', 'Org One');
    useTenantStore.getState().setSelectedWorkspaceId('ws_1');
    useTenantStore.getState().setSelectedProjectId('proj_1');

    await api.get('/gateway-keys');

    expect(seen).toHaveLength(1);
    expect(seen[0].withCredentials).toBe(true);
    expect(seen[0].headers.Authorization).toBeUndefined();
    expect(seen[0].headers['X-Prism-Org-ID']).toBe('org_1');
    expect(seen[0].headers['X-Prism-Workspace-ID']).toBe('ws_1');
    expect(seen[0].headers['X-Prism-Project-ID']).toBe('proj_1');
    expect(seen[0].baseURL).toBe('/api');
  });

  it('clears workspace and project scope when switching organizations', () => {
    useTenantStore.getState().setSelectedOrg('org_1', 'Org One');
    useTenantStore.getState().setSelectedWorkspaceId('ws_1');
    useTenantStore.getState().setSelectedProjectId('proj_1');

    useTenantStore.getState().setSelectedOrg('org_2', 'Org Two');

    expect(useTenantStore.getState().selectedOrgId).toBe('org_2');
    expect(useTenantStore.getState().selectedWorkspaceId).toBeNull();
    expect(useTenantStore.getState().selectedProjectId).toBeNull();
    expect(localStorage.getItem('workspace_id')).toBeNull();
    expect(localStorage.getItem('project_id')).toBeNull();
  });

  it('sends no auth or tenant headers when nothing is selected', async () => {
    const seen = stubAdapter();

    await api.get('/health');

    expect(seen[0].headers.Authorization).toBeUndefined();
    expect(seen[0].headers['X-Prism-Org-ID']).toBeUndefined();
  });

  it('turns 401 failures into ApiError and stays on auth pages', async () => {
    stubAdapter(401);
    // On the sign-in page a 401 is a wrong password, so no redirect happens.
    window.history.replaceState({}, '', '/signin');

    await expect(api.post('/auth/login', {})).rejects.toMatchObject({ code: 'AUTH_REQUIRED', status: 401 });
    await expect(api.post('/auth/login', {})).rejects.toBeInstanceOf(ApiError);
    expect(window.location.pathname).toBe('/signin');
  });

  it('keeps other errors as ApiError with their status', async () => {
    stubAdapter(429);
    await expect(api.get('/logs')).rejects.toMatchObject({ code: 'RATE_LIMITED', status: 429, retryable: true });
  });

  it('omits workspace and project headers on org discovery endpoints (DEF-010)', async () => {
    const seen = stubAdapter();
    useTenantStore.getState().setSelectedOrg('org_1', 'Org One');
    useTenantStore.getState().setSelectedWorkspaceId('ws_forbidden');
    useTenantStore.getState().setSelectedProjectId('proj_forbidden');

    await api.get('/workspaces');
    await api.get('/user/permissions');
    await api.get('/gateway-keys');

    expect(seen).toHaveLength(3);
    expect(seen[0].headers['X-Prism-Org-ID']).toBe('org_1');
    expect(seen[0].headers['X-Prism-Workspace-ID']).toBeUndefined();
    expect(seen[0].headers['X-Prism-Project-ID']).toBeUndefined();

    expect(seen[1].headers['X-Prism-Org-ID']).toBe('org_1');
    expect(seen[1].headers['X-Prism-Workspace-ID']).toBeUndefined();
    expect(seen[1].headers['X-Prism-Project-ID']).toBeUndefined();

    expect(seen[2].headers['X-Prism-Workspace-ID']).toBe('ws_forbidden');
    expect(seen[2].headers['X-Prism-Project-ID']).toBe('proj_forbidden');
  });

  it('extracts JSON error messages from Blob error responses (DEF-013)', async () => {
    api.defaults.adapter = async (config) => {
      const blob = new Blob(
        [JSON.stringify({ error: { message: 'Forbidden: insufficient permissions for action audit:export' } })],
        { type: 'application/json' }
      );
      throw new AxiosError('forbidden', '403', config, null, {
        data: blob,
        status: 403,
        statusText: 'Forbidden',
        headers: {},
        config,
      });
    };

    await expect(api.get('/audit-trail/export', { responseType: 'blob' })).rejects.toMatchObject({
      code: 'FORBIDDEN',
      status: 403,
      message: 'Forbidden: insufficient permissions for action audit:export',
    });
  });
});
