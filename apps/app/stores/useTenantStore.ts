import { create } from 'zustand';
import { createJSONStorage, persist, type StateStorage } from 'zustand/middleware';

interface TenantState {
  selectedOrgId: string | null;
  selectedOrgName: string | null;
  selectedWorkspaceId: string | null;
  setSelectedOrg: (orgId: string | null, orgName?: string | null) => void;
  setSelectedWorkspaceId: (workspaceId: string | null) => void;
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

const getInitialOrgId = (): string | null => {
  const storage = getSafeStorage();
  return storage ? storage.getItem('org_id') : null;
};

const getInitialWsId = (): string | null => {
  const storage = getSafeStorage();
  return storage ? storage.getItem('workspace_id') : null;
};

export const useTenantStore = create<TenantState>()(
  persist(
    (set) => ({
      selectedOrgId: getInitialOrgId(),
      selectedOrgName: null,
      selectedWorkspaceId: getInitialWsId(),
      setSelectedOrg: (orgId: string | null, orgName: string | null = null) => {
        const storage = getSafeStorage();
        if (storage) {
          if (orgId) {
            storage.setItem('org_id', orgId);
          } else {
            storage.removeItem('org_id');
          }
        }
        set({
          selectedOrgId: orgId,
          selectedOrgName: orgName,
        });
      },
      setSelectedWorkspaceId: (workspaceId: string | null) => {
        const storage = getSafeStorage();
        if (storage) {
          if (workspaceId) {
            storage.setItem('workspace_id', workspaceId);
          } else {
            storage.removeItem('workspace_id');
          }
        }
        set({ selectedWorkspaceId: workspaceId });
      },
      resetTenant: () => {
        const storage = getSafeStorage();
        if (storage) {
          storage.removeItem('org_id');
          storage.removeItem('workspace_id');
        }
        set({
          selectedOrgId: null,
          selectedOrgName: null,
          selectedWorkspaceId: null,
        });
      },
    }),
    {
      name: 'prism-tenant-storage',
      storage: createJSONStorage(() => getSafeStorage() ?? memoryStorage),
    }
  )
);
