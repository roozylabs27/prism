import { create } from 'zustand';
import { persist } from 'zustand/middleware';

interface SidebarState {
  collapsed: boolean;
  mobileOpen: boolean;
  toggleCollapsed: () => void;
  setCollapsed: (collapsed: boolean) => void;
  setMobileOpen: (open: boolean) => void;
  toggleMobileOpen: () => void;
}

export const useSidebarStore = create<SidebarState>()(
  persist(
    (set) => ({
      collapsed: false,
      mobileOpen: false,
      toggleCollapsed: () => set((state) => ({ collapsed: !state.collapsed })),
      setCollapsed: (collapsed: boolean) => set({ collapsed }),
      setMobileOpen: (mobileOpen: boolean) => set({ mobileOpen }),
      toggleMobileOpen: () => set((state) => ({ mobileOpen: !state.mobileOpen })),
    }),
    {
      name: 'prism-sidebar-storage',
      partialize: (state) => ({ collapsed: state.collapsed }),
    }
  )
);
