'use client';

import { FeatureGate } from '@/components/molecules/FeatureGate';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { ThemeToggle } from '@/components/molecules/ThemeToggle';
import { StatusDot } from '@/components/atoms/Badge';
import { TenantSelector } from '@/components/TenantSelector';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/atoms/Avatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/molecules/DropdownMenu';
import { useAuth } from '@/context/AuthContext';
import { usePermission } from '@/components/PermissionProvider';
import { useDashboardHealthQuery } from '@/hooks/queries/useDashboardQuery';
import { useSidebarStore } from '@/stores/useSidebarStore';
import { AppRoutes } from '@/constants/routes';
import { Building, CreditCard, LogOut, Menu, Users } from 'lucide-react';

const ROUTE_TITLES: Array<{ prefix: string; label: string }> = [
  { prefix: AppRoutes.SANDBOX, label: 'AI Sandbox' },
  { prefix: AppRoutes.LOGS, label: 'Request Logs' },
  { prefix: AppRoutes.AUDIT_TRAIL, label: 'Audit Trail' },
  { prefix: AppRoutes.PROVIDERS, label: 'Providers' },
  { prefix: AppRoutes.CREDENTIALS, label: 'Credentials' },
  { prefix: AppRoutes.MODELS, label: 'Models' },
  { prefix: AppRoutes.POLICIES, label: 'Routing Policies' },
  { prefix: AppRoutes.GATEWAY_KEYS, label: 'Gateway Keys' },
  { prefix: AppRoutes.AGENTS, label: 'Agent Gateway' },
  { prefix: AppRoutes.TOOLS, label: 'Tool Gateway' },
  { prefix: AppRoutes.RESOURCES, label: 'Resource Gateway' },
  { prefix: AppRoutes.MCP, label: 'MCP Gateway' },
  { prefix: AppRoutes.PLAYGROUND, label: 'Routing Simulator' },
  { prefix: AppRoutes.GOVERNANCE, label: 'Governance & RBAC' },
  { prefix: AppRoutes.APPROVALS, label: 'Approvals' },
  { prefix: AppRoutes.BUDGETS, label: 'Budgets & Quotas' },
  { prefix: AppRoutes.SETTINGS_MEMBERS, label: 'Team Members' },
  { prefix: AppRoutes.BILLING, label: 'Billing & Plans' },
  { prefix: AppRoutes.SETTINGS_ORGANIZATION, label: 'Settings' },
];

function getRouteLabel(pathname: string): string {
  if (pathname === AppRoutes.HOME) return 'Overview';
  const match = ROUTE_TITLES.find((r) => pathname.startsWith(r.prefix));
  return match?.label || 'Overview';
}

function getUserInitials(name?: string, email?: string): string {
  if (name && name.trim()) {
    const parts = name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase();
    }
    return parts[0].slice(0, 2).toUpperCase();
  }
  if (email && email.trim()) {
    return email.slice(0, 2).toUpperCase();
  }
  return 'PA';
}

export function DashboardTopbar() {
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const { role: activeRole, hasPermission, isLoading: isPermsLoading } = usePermission();
  const canReadModels = !isPermsLoading && hasPermission('model:read');
  const { data: healthData, isError: isHealthError } = useDashboardHealthQuery();
  const { toggleMobileOpen } = useSidebarStore();

  const isAnyProviderDegraded =
    canReadModels &&
    (isHealthError ||
      (Array.isArray(healthData) &&
        healthData.length > 0 &&
        healthData.some((p) => p.status === 'down')));

  const systemStatus = isAnyProviderDegraded ? 'degraded' : 'operational';
  const systemStatusLabel = isAnyProviderDegraded ? 'System degraded' : 'System operational';

  const displayName = user?.name || user?.email?.split('@')[0] || 'Prism User';
  const email = user?.email || 'user@prism.local';
  const role = !isPermsLoading && activeRole ? activeRole : user?.role || 'viewer';
  const initials = getUserInitials(user?.name, user?.email);
  const avatarSrc = user?.avatarUrl || user?.image;
  const routeLabel = getRouteLabel(pathname);

  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between gap-3 border-b border-border bg-background/95 px-4 sm:px-6 backdrop-blur">
      <div className="flex items-center gap-2.5 sm:gap-3 min-w-0">
        <button
          type="button"
          onClick={toggleMobileOpen}
          className="inline-flex md:hidden min-h-[40px] items-center gap-1.5 rounded-lg border border-border bg-card px-2.5 text-xs font-medium text-foreground hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
          aria-label="Open navigation menu"
        >
          <Menu className="h-4 w-4" />
          <span>Menu</span>
        </button>

        <TenantSelector />

        <span className="hidden sm:inline text-muted-foreground/60" aria-hidden="true">
          /
        </span>
        <span className="hidden sm:inline truncate text-xs font-medium text-foreground">
          {routeLabel}
        </span>
      </div>

      <div className="flex items-center gap-2.5 sm:gap-3 shrink-0">
        {canReadModels && (
          <div className="hidden md:flex items-center rounded-lg border border-border bg-card px-2.5 py-1.5">
            <StatusDot status={systemStatus} label={systemStatusLabel} />
          </div>
        )}

        <ThemeToggle />

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="flex items-center rounded-full p-0.5 transition-all hover:ring-2 hover:ring-violet-500/40 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2 focus:ring-offset-background cursor-pointer"
              aria-label="User account menu"
            >
              <Avatar className="h-8 w-8">
                {avatarSrc && <AvatarImage src={avatarSrc} alt={displayName} />}
                <AvatarFallback className="bg-primary text-primary-foreground text-xs font-bold font-mono">
                  {initials}
                </AvatarFallback>
              </Avatar>
            </button>
          </DropdownMenuTrigger>

          <DropdownMenuContent className="w-56" align="end" sideOffset={6}>
            <DropdownMenuLabel className="font-normal p-2">
              <div className="flex flex-col space-y-1">
                <p className="text-xs font-semibold leading-none text-foreground truncate">{displayName}</p>
                <p className="text-[11px] leading-none text-muted-foreground truncate">{email}</p>
                <div className="pt-1">
                  <span className="inline-flex items-center rounded-md px-1.5 py-0.5 text-[10px] font-medium font-mono bg-secondary text-muted-foreground border border-border">
                    {role}
                  </span>
                </div>
              </div>
            </DropdownMenuLabel>

            <DropdownMenuSeparator />

            <DropdownMenuItem asChild>
              <Link href={AppRoutes.SETTINGS_ORGANIZATION} className="flex items-center gap-2 cursor-pointer">
                <Building className="h-3.5 w-3.5 text-muted-foreground" />
                <span>Organization Settings</span>
              </Link>
            </DropdownMenuItem>

            <DropdownMenuItem asChild>
              <Link href={AppRoutes.SETTINGS_MEMBERS} className="flex items-center gap-2 cursor-pointer">
                <Users className="h-3.5 w-3.5 text-muted-foreground" />
                <span>Team Members</span>
              </Link>
            </DropdownMenuItem>

            <FeatureGate flag="billing">
              <DropdownMenuItem asChild>
                <Link href={AppRoutes.BILLING} className="flex items-center gap-2 cursor-pointer">
                  <CreditCard className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>Billing & Plans</span>
                </Link>
              </DropdownMenuItem>
            </FeatureGate>

            <DropdownMenuSeparator />

            <DropdownMenuItem
              onSelect={async (e) => {
                e.preventDefault();
                await logout();
              }}
              onClick={async () => {
                await logout();
              }}
              className="flex items-center gap-2 text-destructive focus:text-destructive cursor-pointer"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>Sign out</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  );
}
