'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import {
  LayoutDashboard,
  Box,
  Server,
  Key,
  Layers,
  Workflow,
  KeyRound,
  Wrench,
  Database,
  Globe,
  Bot,
  Users,
  ShieldCheck,
  ScrollText,
  Activity,
  Wallet,
  Play,
  Settings,
  CreditCard,
  ChevronLeft,
  ChevronRight,
  Search,
  X,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { DEFAULT_FEATURE_FLAGS, type FeatureFlagKey } from '@/lib/features/features';
import { useFeatureFlagsQuery } from '@/hooks/queries/useFeatureFlags';
import { Button } from '@/components/atoms/Button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/molecules/Dialog';
import { ModelActivityWidget } from '@/components/molecules/ModelActivityWidget';
import { useSidebarStore } from '@/stores/useSidebarStore';
import { AppRoutes } from '@/constants/routes';

interface NavItem {
  key: string;
  label: string;
  href: AppRoutes;
  icon: ReactNode;
  group: string;
  /** Shown only while this feature flag is on. */
  flag?: FeatureFlagKey;
}

interface NavGroup {
  title: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    title: 'OVERVIEW',
    items: [
      { key: AppRoutes.HOME, label: 'Overview', href: AppRoutes.HOME, group: 'Overview', icon: <LayoutDashboard className="h-4 w-4" /> },
      { key: AppRoutes.SANDBOX, label: 'AI Sandbox', href: AppRoutes.SANDBOX, group: 'Overview', icon: <Box className="h-4 w-4" /> },
    ],
  },
  {
    title: 'OBSERVE',
    items: [
      { key: AppRoutes.LOGS, label: 'Request Logs', href: AppRoutes.LOGS, group: 'Observe', icon: <Activity className="h-4 w-4" /> },
      { key: AppRoutes.AUDIT_TRAIL, label: 'Audit Trail', href: AppRoutes.AUDIT_TRAIL, group: 'Observe', icon: <ScrollText className="h-4 w-4" /> },
    ],
  },
  {
    title: 'AI INFRASTRUCTURE',
    items: [
      { key: AppRoutes.PROVIDERS, label: 'Providers', href: AppRoutes.PROVIDERS, group: 'Infrastructure', icon: <Server className="h-4 w-4" /> },
      { key: AppRoutes.CREDENTIALS, label: 'Credentials', href: AppRoutes.CREDENTIALS, group: 'Infrastructure', icon: <Key className="h-4 w-4" /> },
      { key: AppRoutes.MODELS, label: 'Models', href: AppRoutes.MODELS, group: 'Infrastructure', icon: <Layers className="h-4 w-4" /> },
      { key: AppRoutes.POLICIES, label: 'Routing Policies', href: AppRoutes.POLICIES, group: 'Infrastructure', icon: <Workflow className="h-4 w-4" /> },
      { key: AppRoutes.GATEWAY_KEYS, label: 'Gateway Keys', href: AppRoutes.GATEWAY_KEYS, group: 'Infrastructure', icon: <KeyRound className="h-4 w-4" /> },
    ],
  },
  {
    title: 'AI RUNTIME',
    items: [
      { key: AppRoutes.AGENTS, label: 'Agent Gateway', href: AppRoutes.AGENTS, group: 'Runtime', icon: <Bot className="h-4 w-4" /> },
      { key: AppRoutes.TOOLS, label: 'Tool Gateway', href: AppRoutes.TOOLS, group: 'Runtime', icon: <Wrench className="h-4 w-4" /> },
      { key: AppRoutes.RESOURCES, label: 'Resource Gateway', href: AppRoutes.RESOURCES, group: 'Runtime', icon: <Database className="h-4 w-4" /> },
      { key: AppRoutes.MCP, label: 'MCP Gateway', href: AppRoutes.MCP, group: 'Runtime', icon: <Globe className="h-4 w-4" /> },
      { key: AppRoutes.PLAYGROUND, label: 'Routing Simulator', href: AppRoutes.PLAYGROUND, group: 'Runtime', icon: <Play className="h-4 w-4" /> },
    ],
  },
  {
    title: 'CONTROL',
    items: [
      { key: AppRoutes.GOVERNANCE, label: 'Governance & RBAC', href: AppRoutes.GOVERNANCE, group: 'Control', icon: <Users className="h-4 w-4" /> },
      { key: AppRoutes.APPROVALS, label: 'Approvals', href: AppRoutes.APPROVALS, group: 'Control', icon: <ShieldCheck className="h-4 w-4" /> },
      { key: AppRoutes.BUDGETS, label: 'Budgets & Quotas', href: AppRoutes.BUDGETS, group: 'Control', icon: <Wallet className="h-4 w-4" /> },
    ],
  },
  {
    title: 'ORGANIZATION',
    items: [
      { key: AppRoutes.SETTINGS_MEMBERS, label: 'Team Members', href: AppRoutes.SETTINGS_MEMBERS, group: 'Organization', icon: <Users className="h-4 w-4" /> },
      { key: AppRoutes.BILLING, label: 'Billing & Plans', href: AppRoutes.BILLING, group: 'Organization', icon: <CreditCard className="h-4 w-4" />, flag: 'billing' },
      { key: AppRoutes.SETTINGS_ORGANIZATION, label: 'Settings', href: AppRoutes.SETTINGS_ORGANIZATION, group: 'Organization', icon: <Settings className="h-4 w-4" /> },
    ],
  },
];

function PrismBrandMark() {
  return (
    <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-lg border border-violet-500/35 bg-violet-500/15 text-prism-violet">
      <svg className="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <polygon points="12 3 21 19 3 19" />
        <line x1="12" y1="3" x2="12" y2="19" className="text-prism-cyan" stroke="currentColor" />
      </svg>
    </div>
  );
}

export function DashboardSidebar() {
  const pathname = usePathname();
  const router = useRouter();
  const { collapsed, toggleCollapsed, mobileOpen, setMobileOpen } = useSidebarStore();
  const { data: features } = useFeatureFlagsQuery();
  const [commandOpen, setCommandOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const isOn = useCallback(
    (flag: FeatureFlagKey) => features?.flags?.[flag] ?? DEFAULT_FEATURE_FLAGS[flag] ?? false,
    [features]
  );

  const visibleItems = useMemo(
    () =>
      navGroups.flatMap((group) =>
        group.items.filter((item) => !item.flag || isOn(item.flag))
      ),
    [isOn]
  );

  const filteredItems = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return visibleItems;
    return visibleItems.filter(
      (item) =>
        item.label.toLowerCase().includes(q) ||
        item.group.toLowerCase().includes(q) ||
        item.href.toLowerCase().includes(q)
    );
  }, [visibleItems, searchQuery]);

  // Global Cmd/Ctrl+K shortcut for Quick Jump palette
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setCommandOpen((prev) => !prev);
      } else if (event.key === 'Escape' && mobileOpen) {
        setMobileOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [mobileOpen, setMobileOpen]);

  // Close mobile drawer on route change
  useEffect(() => {
    setMobileOpen(false);
  }, [pathname, setMobileOpen]);

  const handleQuickNavigate = (href: AppRoutes) => {
    setCommandOpen(false);
    setSearchQuery('');
    router.push(href);
  };

  return (
    <>
      {/* Mobile Backdrop */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/60 backdrop-blur-xs md:hidden"
          onClick={() => setMobileOpen(false)}
          aria-hidden="true"
        />
      )}

      <aside
        className={cn(
          'fixed bottom-0 top-0 left-0 z-50 flex flex-col border-r border-border bg-card transition-all duration-200',
          // Mobile: slide-over drawer
          mobileOpen ? 'translate-x-0 w-[264px]' : '-translate-x-full w-[264px]',
          // Desktop: persistent collapsible sidebar
          'md:translate-x-0 md:z-40',
          collapsed ? 'md:w-[72px]' : 'md:w-[248px]'
        )}
        aria-label="Primary sidebar navigation"
      >
        {/* Sidebar Header Brand */}
        <div className="flex h-14 items-center justify-between border-b border-border px-3.5">
          <Link href={AppRoutes.HOME} className="flex items-center gap-2.5 overflow-hidden">
            <PrismBrandMark />
            {(!collapsed || mobileOpen) && (
              <div className="min-w-0">
                <span className="block font-mono text-sm font-bold tracking-tight text-foreground truncate">
                  PRISM
                </span>
                <span className="block text-[10px] text-muted-foreground truncate">
                  Control Plane v0.4.0
                </span>
              </div>
            )}
          </Link>

          <Button
            variant="ghost"
            size="icon"
            onClick={toggleCollapsed}
            className="hidden md:inline-flex h-7 w-7 rounded-md text-muted-foreground hover:text-foreground"
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
          </Button>

          <Button
            variant="ghost"
            size="icon"
            onClick={() => setMobileOpen(false)}
            className="md:hidden h-8 w-8 rounded-md text-muted-foreground hover:text-foreground"
            aria-label="Close navigation menu"
          >
            <X className="h-4 w-4" />
          </Button>
        </div>

        {/* Quick Jump Search Trigger */}
        <div className="px-2.5 pt-3 pb-1">
          <button
            type="button"
            onClick={() => setCommandOpen(true)}
            className={cn(
              'flex w-full items-center rounded-lg border border-border/80 bg-background/60 text-xs text-muted-foreground transition-colors hover:border-border hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer',
              collapsed && !mobileOpen ? 'justify-center h-9 px-0' : 'justify-between h-9 px-3'
            )}
            aria-label="Quick search navigation"
          >
            <span className="flex items-center gap-2">
              <Search className="h-3.5 w-3.5 shrink-0" />
              {(!collapsed || mobileOpen) && <span>Search routes...</span>}
            </span>
            {(!collapsed || mobileOpen) && (
              <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
                ⌘K
              </kbd>
            )}
          </button>
        </div>

        {/* Navigation Groups */}
        <nav className="flex-1 overflow-y-auto px-2.5 py-2 space-y-4 custom-scrollbar">
          {navGroups.map((group) => {
            const items = group.items.filter((item) => !item.flag || isOn(item.flag));
            if (items.length === 0) return null;

            return (
              <div key={group.title} className="space-y-0.5">
                {(!collapsed || mobileOpen) && (
                  <h4 className="px-2.5 pb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/80">
                    {group.title}
                  </h4>
                )}
                {items.map((item) => {
                  const isActive =
                    pathname === item.href ||
                    (item.href !== AppRoutes.HOME && pathname.startsWith(item.href));
                  return (
                    <Link
                      key={item.key}
                      href={item.href}
                      title={collapsed && !mobileOpen ? item.label : undefined}
                      className={cn(
                        'group relative flex items-center gap-2.5 rounded-lg px-2.5 py-2 text-xs font-medium transition-colors',
                        collapsed && !mobileOpen && 'justify-center px-0',
                        isActive
                          ? 'border border-border bg-accent text-foreground font-semibold'
                          : 'border border-transparent text-muted-foreground hover:bg-accent/60 hover:text-foreground'
                      )}
                    >
                      {isActive && (
                        <span className="absolute left-0 top-1/2 h-4 w-0.5 -translate-y-1/2 rounded-full bg-gradient-to-b from-violet-500 to-cyan-400" />
                      )}
                      <span
                        className={cn(
                          'shrink-0',
                          isActive ? 'text-prism-violet' : 'text-muted-foreground group-hover:text-foreground'
                        )}
                      >
                        {item.icon}
                      </span>
                      {(!collapsed || mobileOpen) && <span className="truncate">{item.label}</span>}
                    </Link>
                  );
                })}
              </div>
            );
          })}
        </nav>

        {/* Active Model Activity / Smart Router Widget */}
        <div className="border-t border-border/70 px-2.5 py-2.5">
          <ModelActivityWidget collapsed={collapsed && !mobileOpen} />
        </div>
      </aside>

      {/* Command Search Palette Dialog */}
      <Dialog open={commandOpen} onOpenChange={setCommandOpen}>
        <DialogContent className="p-0 gap-0 max-w-md overflow-hidden">
          <DialogHeader className="sr-only">
            <DialogTitle>Quick Jump Navigation</DialogTitle>
          </DialogHeader>
          <div className="flex items-center gap-2.5 border-b border-border px-4 py-3">
            <Search className="h-4 w-4 text-muted-foreground shrink-0" />
            <input
              type="text"
              autoFocus
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && filteredItems.length > 0) {
                  e.preventDefault();
                  handleQuickNavigate(filteredItems[0].href);
                }
              }}
              placeholder="Jump to overview, logs, providers, policies..."
              className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none"
            />
          </div>
          <div className="max-h-72 overflow-y-auto p-2 space-y-1">
            {filteredItems.length === 0 ? (
              <p className="px-3 py-6 text-center text-xs text-muted-foreground">
                No matching console routes found.
              </p>
            ) : (
              filteredItems.map((item) => (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => handleQuickNavigate(item.href)}
                  className="flex w-full items-center justify-between rounded-lg px-3 py-2 text-left text-xs transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
                >
                  <span className="flex items-center gap-2.5 font-medium text-foreground">
                    <span className="text-prism-violet">{item.icon}</span>
                    <span>{item.label}</span>
                  </span>
                  <span className="font-mono text-[10px] text-muted-foreground">{item.href}</span>
                </button>
              ))
            )}
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
