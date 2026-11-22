import { create } from 'zustand';
import { createJSONStorage, persist, type StateStorage } from 'zustand/middleware';

interface TenantState {
  selectedOrgId: string | null;
  selectedOrgName: string | null;
  selectedWorkspaceId: string | null;
  selectedProjectId: string | null;
  setSelectedOrg: (orgId: string | null, orgName?: string | null) => void;
  setSelectedWorkspaceId: (workspaceId: string | null) => void;
  setSelectedProjectId: (projectId: string | null) => void;
  resetTenant: () => void;
}

type SyncStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

const memoryBacking = new Map<string, string>();
const memoryStorage: StateStorage = {
  getItem: (name) => memoryBacking.get(name) ?? null,
  setItem: (name, value) => {
    memoryBacking.set(name, value);
  },
  removeItem: (name) => {
    memoryBacking.delete(name);
  },
};

function getSafeStorage(): SyncStorage | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const ls = window.localStorage;
  if (
    ls &&
    typeof ls.getItem === 'function' &&
    typeof ls.setItem === 'function' &&
    typeof ls.removeItem === 'function'
  ) {
    return ls;
  }
  return null;
}

const getInitialItem = (key: string): string | null => {
  const storage = getSafeStorage();
  return storage ? storage.getItem(key) : null;
};

function syncActiveOrgCookie(orgId: string | null) {
  if (typeof document === 'undefined') return;
  if (orgId) {
    document.cookie = `prism_active_org=${encodeURIComponent(orgId)}; Path=/; Max-Age=2592000; SameSite=Lax`;
  } else {
    document.cookie = 'prism_active_org=; Path=/; Max-Age=0; SameSite=Lax';
  }
}

export const useTenantStore = create<TenantState>()(
  persist(
    (set, get) => ({
      selectedOrgId: getInitialItem('org_id'),
      selectedOrgName: null,
      selectedWorkspaceId: getInitialItem('workspace_id'),
      selectedProjectId: getInitialItem('project_id'),
      setSelectedOrg: (orgId: string | null, orgName: string | null = null) => {
        const storage = getSafeStorage();
        const orgChanged = orgId !== get().selectedOrgId;
        syncActiveOrgCookie(orgId);
        if (storage) {
          if (orgId) {
            storage.setItem('org_id', orgId);
          } else {
            storage.removeItem('org_id');
          }
          if (orgChanged) {
            storage.removeItem('workspace_id');
            storage.removeItem('project_id');
          }
        }
        set((state) => ({
          selectedOrgId: orgId,
          selectedOrgName: orgName,
          selectedWorkspaceId: orgChanged ? null : state.selectedWorkspaceId,
          selectedProjectId: orgChanged ? null : state.selectedProjectId,
        }));
      },
      setSelectedWorkspaceId: (workspaceId: string | null) => {
        const storage = getSafeStorage();
        const wsChanged = workspaceId !== get().selectedWorkspaceId;
        if (storage) {
          if (workspaceId) {
            storage.setItem('workspace_id', workspaceId);
          } else {
            storage.removeItem('workspace_id');
          }
          if (wsChanged) {
            storage.removeItem('project_id');
          }
        }
        set((state) => ({
          selectedWorkspaceId: workspaceId,
          selectedProjectId: wsChanged ? null : state.selectedProjectId,
        }));
      },
      setSelectedProjectId: (projectId: string | null) => {
        const storage = getSafeStorage();
        if (storage) {
          if (projectId) {
            storage.setItem('project_id', projectId);
          } else {
            storage.removeItem('project_id');
          }
        }
        set({ selectedProjectId: projectId });
      },
      resetTenant: () => {
        const storage = getSafeStorage();
        syncActiveOrgCookie(null);
        if (storage) {
          storage.removeItem('org_id');
          storage.removeItem('workspace_id');
          storage.removeItem('project_id');
        }
        set({
          selectedOrgId: null,
          selectedOrgName: null,
          selectedWorkspaceId: null,
          selectedProjectId: null,
        });
      },
    }),
    {
      name: 'prism-tenant-storage',
      storage: createJSONStorage(() => getSafeStorage() ?? memoryStorage),
    }
  )
);

export function getTenantHeaders(): Record<string, string> {
  const headers: Record<string, string> = {};
  try {
    const state = useTenantStore.getState();
    const storage = getSafeStorage();
    const orgId = state.selectedOrgId || storage?.getItem('org_id') || null;
    const wsId = state.selectedWorkspaceId || storage?.getItem('workspace_id') || null;
    const projId = state.selectedProjectId || storage?.getItem('project_id') || null;
    if (orgId) headers['X-Prism-Org-ID'] = orgId;
    if (wsId) headers['X-Prism-Workspace-ID'] = wsId;
    if (projId) headers['X-Prism-Project-ID'] = projId;
  } catch {
    // Ignore when storage is unavailable
  }
  return headers;
}
