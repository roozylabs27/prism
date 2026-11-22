'use client';

import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';
import { DashboardSidebar } from '@/components/layouts/DashboardSidebar';
import { DashboardTopbar } from '@/components/layouts/DashboardTopbar';
import { useSidebarStore } from '@/stores/useSidebarStore';

export function AppLayout({ children }: { children: ReactNode }) {
  const { collapsed } = useSidebarStore();

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background text-foreground">
      <DashboardSidebar />

      {/* Main Content Area: responsive left padding on md+ only, zero overlap on mobile */}
      <div
        className={cn(
          'flex flex-1 flex-col h-screen min-w-0 overflow-hidden transition-all duration-200 pl-0',
          collapsed ? 'md:pl-[72px]' : 'md:pl-[248px]'
        )}
      >
        <DashboardTopbar />

        {/* Page Content - scroll y & x isolated exclusively inside main */}
        <main className="flex-1 overflow-auto min-w-0 p-4 sm:p-6">{children}</main>
      </div>
    </div>
  );
}
