'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/molecules/Dialog';
import { Button } from '@/components/atoms/Button';
import { Input } from '@/components/atoms/Input';
import { Label } from '@/components/atoms/Label';
import { Badge } from '@/components/atoms/Badge';
import {
  apiCreateGatewayKey,
  apiCreateProject,
  apiCreateWorkspace,
  apiUpdateQuota,
} from '@/lib/api';
import { getErrorMessage } from '@/types/ui';
import { Check, Copy, Rocket } from 'lucide-react';
import { toast } from 'sonner';

interface ClientOnboardingModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCompleted?: (result: { workspaceId: string; projectId: string; keyPrefix: string }) => void;
}

interface ProvisionedResult {
  workspaceId: string;
  workspaceName: string;
  projectId: string;
  projectName: string;
  keyPrefix: string;
  rawKey: string;
}

export function ClientOnboardingModal({
  open,
  onOpenChange,
  onCompleted,
}: ClientOnboardingModalProps) {
  const queryClient = useQueryClient();
  const [clientName, setClientName] = useState('');
  const [projectName, setProjectName] = useState('Default Workflow');
  const [monthlyLimitUsd, setMonthlyLimitUsd] = useState('200');
  const [dailyLimitUsd, setDailyLimitUsd] = useState('10');
  const [dailyRequestLimit, setDailyRequestLimit] = useState('2000');
  const [rateLimitRpm, setRateLimitRpm] = useState('120');
  const [submitting, setSubmitting] = useState(false);
  const [provisioned, setProvisioned] = useState<ProvisionedResult | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedSnippet, setCopiedSnippet] = useState(false);

  const resetForm = () => {
    setClientName('');
    setProjectName('Default Workflow');
    setMonthlyLimitUsd('200');
    setDailyLimitUsd('10');
    setDailyRequestLimit('2000');
    setRateLimitRpm('120');
    setProvisioned(null);
    setCopiedKey(false);
    setCopiedSnippet(false);
  };

  const handleClose = (nextOpen: boolean) => {
    if (!nextOpen) {
      resetForm();
    }
    onOpenChange(nextOpen);
  };

  const handleProvision = async () => {
    if (!clientName.trim()) {
      toast.error('Client workspace name is required');
      return;
    }

    const monthly = Math.max(0, Number(monthlyLimitUsd) || 0);
    const daily = Math.max(0, Number(dailyLimitUsd) || 0);
    const reqs = Math.max(0, Number(dailyRequestLimit) || 0);
    const rpm = Math.max(1, Number(rateLimitRpm) || 120);

    setSubmitting(true);
    try {
      const ws = await apiCreateWorkspace({ name: clientName.trim() });
      const proj = await apiCreateProject(ws.id, {
        name: projectName.trim() || 'Default Workflow',
      });
      await apiUpdateQuota('workspace', ws.id, {
        monthlySpendLimitUsd: monthly,
        dailySpendLimitUsd: daily,
        dailyRequestLimit: reqs,
      });
      const createdKey = await apiCreateGatewayKey({
        name: `${ws.name} - ${proj.name}`,
        workspaceId: ws.id,
        projectId: proj.id,
        rateLimit: rpm,
      });

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
        queryClient.invalidateQueries({ queryKey: ['projects'] }),
        queryClient.invalidateQueries({ queryKey: ['quotas'] }),
        queryClient.invalidateQueries({ queryKey: ['gateway-keys'] }),
        queryClient.invalidateQueries({ queryKey: ['pilot-outcomes'] }),
      ]);

      setProvisioned({
        workspaceId: ws.id,
        workspaceName: ws.name,
        projectId: proj.id,
        projectName: proj.name,
        keyPrefix: createdKey.keyPrefix,
        rawKey: createdKey.key,
      });
      onCompleted?.({
        workspaceId: ws.id,
        projectId: proj.id,
        keyPrefix: createdKey.keyPrefix,
      });
      toast.success(`Client "${ws.name}" provisioned with pinned key ${createdKey.keyPrefix}`);
    } catch (err) {
      toast.error(`Onboarding failed: ${getErrorMessage(err)}`);
    } finally {
      setSubmitting(false);
    }
  };

  const snippet = provisioned
    ? JSON.stringify(
        {
          provider: 'prism',
          api_url: 'https://api.prism.roozylabs.com/v1',
          mcp_url: 'https://api.prism.roozylabs.com/v1/mcp',
          api_key: provisioned.rawKey,
          model: 'prism-auto',
          headers: {
            'X-Prism-Workspace-ID': provisioned.workspaceId,
            'X-Prism-Project-ID': provisioned.projectId,
          },
        },
        null,
        2
      )
    : '';

  const copyValue = async (text: string, which: 'key' | 'snippet') => {
    try {
      await navigator.clipboard.writeText(text);
      if (which === 'key') {
        setCopiedKey(true);
        setTimeout(() => setCopiedKey(false), 2000);
      } else {
        setCopiedSnippet(true);
        setTimeout(() => setCopiedSnippet(false), 2000);
      }
    } catch {
      toast.error('Clipboard copy failed');
    }
  };

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2 text-base">
            <Rocket className="h-4 w-4 text-prism-violet" />
            <span>Onboard Agency Client Workspace</span>
          </DialogTitle>
          <DialogDescription className="text-xs">
            Provision an isolated client workspace, initial project, hard quota guardrails, and a pinned gateway key in one flow.
          </DialogDescription>
        </DialogHeader>

        {!provisioned ? (
          <div className="space-y-3 py-1">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-client-name">Client Workspace Name</Label>
                <Input
                  id="onboard-client-name"
                  placeholder="Acme Corp"
                  value={clientName}
                  onChange={(e) => setClientName(e.target.value)}
                  className="h-8 text-xs"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-project-name">Initial Project Name</Label>
                <Input
                  id="onboard-project-name"
                  placeholder="Default Workflow"
                  value={projectName}
                  onChange={(e) => setProjectName(e.target.value)}
                  className="h-8 text-xs"
                />
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-monthly-usd">Monthly Spend Cap (USD)</Label>
                <Input
                  id="onboard-monthly-usd"
                  type="number"
                  min={0}
                  step="1"
                  value={monthlyLimitUsd}
                  onChange={(e) => setMonthlyLimitUsd(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-daily-usd">Daily Spend Cap (USD)</Label>
                <Input
                  id="onboard-daily-usd"
                  type="number"
                  min={0}
                  step="0.5"
                  value={dailyLimitUsd}
                  onChange={(e) => setDailyLimitUsd(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-daily-reqs">Daily Request Cap</Label>
                <Input
                  id="onboard-daily-reqs"
                  type="number"
                  min={0}
                  value={dailyRequestLimit}
                  onChange={(e) => setDailyRequestLimit(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-rpm">Key Rate Limit (RPM)</Label>
                <Input
                  id="onboard-rpm"
                  type="number"
                  min={1}
                  value={rateLimitRpm}
                  onChange={(e) => setRateLimitRpm(e.target.value)}
                  className="h-8 text-xs font-mono"
                />
              </div>
            </div>
          </div>
        ) : (
          <div className="space-y-3 py-1 text-xs">
            <div className="rounded-md border border-emerald-500/30 bg-emerald-500/5 p-3 space-y-1.5">
              <div className="flex items-center justify-between">
                <span className="font-semibold text-foreground">{provisioned.workspaceName}</span>
                <Badge variant="success" className="text-[10px]">PROVISIONED</Badge>
              </div>
              <div className="font-mono text-[11px] text-muted-foreground">
                Workspace: {provisioned.workspaceId} · Project: {provisioned.projectId}
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="text-xs">One-Time Pinned Gateway Key (shown once)</Label>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded border border-border bg-muted/40 px-2.5 py-1.5 font-mono text-[11px] break-all">
                  {provisioned.rawKey}
                </code>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  className="h-8 gap-1 text-xs shrink-0"
                  onClick={() => copyValue(provisioned.rawKey, 'key')}
                >
                  {copiedKey ? <Check className="h-3.5 w-3.5 text-emerald-500" /> : <Copy className="h-3.5 w-3.5" />}
                  {copiedKey ? 'Copied' : 'Copy Key'}
                </Button>
              </div>
            </div>

            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <Label className="text-xs">Client Integration Snippet (/v1 &amp; /v1/mcp)</Label>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 gap-1 text-[11px]"
                  onClick={() => copyValue(snippet, 'snippet')}
                >
                  {copiedSnippet ? <Check className="h-3 w-3 text-emerald-500" /> : <Copy className="h-3 w-3" />}
                  {copiedSnippet ? 'Copied' : 'Copy JSON'}
                </Button>
              </div>
              <pre className="max-h-44 overflow-auto rounded border border-border bg-muted/40 p-2.5 font-mono text-[11px] text-foreground">
                {snippet}
              </pre>
            </div>
          </div>
        )}

        <DialogFooter>
          {!provisioned ? (
            <>
              <Button type="button" variant="outline" size="sm" onClick={() => handleClose(false)}>
                Cancel
              </Button>
              <Button
                type="button"
                variant="prismViolet"
                size="sm"
                disabled={submitting || !clientName.trim()}
                onClick={handleProvision}
              >
                {submitting ? 'Provisioning...' : 'Provision Client'}
              </Button>
            </>
          ) : (
            <Button type="button" size="sm" onClick={() => handleClose(false)}>
              Done
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
