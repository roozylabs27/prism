'use client';

import { useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
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
import {
  clientOnboardingSchema,
  type ClientOnboardingFormValues,
} from '@/features/onboarding/schemas/client-onboarding.schema';
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

interface PartialStepProgress {
  clientName: string;
  workspace?: { id: string; name: string };
  project?: { id: string; name: string };
}

const DEFAULT_VALUES: ClientOnboardingFormValues = {
  clientName: '',
  projectName: 'Default Workflow',
  monthlyLimitUsd: 200,
  dailyLimitUsd: 10,
  dailyRequestLimit: 2000,
  rateLimitRpm: 120,
};

export function ClientOnboardingModal({
  open,
  onOpenChange,
  onCompleted,
}: ClientOnboardingModalProps) {
  const queryClient = useQueryClient();
  const [provisioned, setProvisioned] = useState<ProvisionedResult | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);
  const [copiedSnippet, setCopiedSnippet] = useState(false);
  // Tracks partially created resources so retrying after a mid-flow error reuses them instead of duplicating
  const progressRef = useRef<PartialStepProgress | null>(null);

  const {
    register,
    handleSubmit,
    reset,
    watch,
    formState: { errors, isSubmitting },
  } = useForm<ClientOnboardingFormValues>({
    resolver: zodResolver(clientOnboardingSchema),
    defaultValues: DEFAULT_VALUES,
  });

  const clientNameVal = watch('clientName');

  const resetForm = () => {
    reset(DEFAULT_VALUES);
    progressRef.current = null;
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

  const onSubmit = async (values: ClientOnboardingFormValues) => {
    try {
      if (!progressRef.current || progressRef.current.clientName !== values.clientName) {
        progressRef.current = { clientName: values.clientName };
      }
      const progress = progressRef.current;

      const ws =
        progress.workspace ??
        (await apiCreateWorkspace({ name: values.clientName }));
      progress.workspace = { id: ws.id, name: ws.name };

      const proj =
        progress.project ??
        (await apiCreateProject(ws.id, {
          name: values.projectName || 'Default Workflow',
        }));
      progress.project = { id: proj.id, name: proj.name };

      await apiUpdateQuota('workspace', ws.id, {
        monthlySpendLimitUsd: values.monthlyLimitUsd,
        dailySpendLimitUsd: values.dailyLimitUsd,
        dailyRequestLimit: values.dailyRequestLimit,
      });

      const createdKey = await apiCreateGatewayKey({
        name: `${ws.name} - ${proj.name}`,
        workspaceId: ws.id,
        projectId: proj.id,
        rateLimit: values.rateLimitRpm,
      });

      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['workspaces'] }),
        queryClient.invalidateQueries({ queryKey: ['projects'] }),
        queryClient.invalidateQueries({ queryKey: ['quotas'] }),
        queryClient.invalidateQueries({ queryKey: ['gateway-keys'] }),
        queryClient.invalidateQueries({ queryKey: ['pilot-outcomes'] }),
      ]);

      progressRef.current = null;
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
          <form id="client-onboarding-form" onSubmit={handleSubmit(onSubmit)} className="space-y-3 py-1">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-client-name">Client Workspace Name</Label>
                <Input
                  id="onboard-client-name"
                  placeholder="Acme Corp"
                  {...register('clientName')}
                  className="h-8 text-xs"
                />
                {errors.clientName && (
                  <p className="text-[11px] text-destructive">{errors.clientName.message}</p>
                )}
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-project-name">Initial Project Name</Label>
                <Input
                  id="onboard-project-name"
                  placeholder="Default Workflow"
                  {...register('projectName')}
                  className="h-8 text-xs"
                />
                {errors.projectName && (
                  <p className="text-[11px] text-destructive">{errors.projectName.message}</p>
                )}
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
                  {...register('monthlyLimitUsd')}
                  className="h-8 text-xs font-mono"
                />
                {errors.monthlyLimitUsd && (
                  <p className="text-[11px] text-destructive">{errors.monthlyLimitUsd.message}</p>
                )}
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-daily-usd">Daily Spend Cap (USD)</Label>
                <Input
                  id="onboard-daily-usd"
                  type="number"
                  min={0}
                  step="0.5"
                  {...register('dailyLimitUsd')}
                  className="h-8 text-xs font-mono"
                />
                {errors.dailyLimitUsd && (
                  <p className="text-[11px] text-destructive">{errors.dailyLimitUsd.message}</p>
                )}
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-daily-reqs">Daily Request Cap</Label>
                <Input
                  id="onboard-daily-reqs"
                  type="number"
                  min={0}
                  {...register('dailyRequestLimit')}
                  className="h-8 text-xs font-mono"
                />
                {errors.dailyRequestLimit && (
                  <p className="text-[11px] text-destructive">{errors.dailyRequestLimit.message}</p>
                )}
              </div>
              <div className="space-y-1">
                <Label className="text-xs" htmlFor="onboard-rpm">Key Rate Limit (RPM)</Label>
                <Input
                  id="onboard-rpm"
                  type="number"
                  min={1}
                  {...register('rateLimitRpm')}
                  className="h-8 text-xs font-mono"
                />
                {errors.rateLimitRpm && (
                  <p className="text-[11px] text-destructive">{errors.rateLimitRpm.message}</p>
                )}
              </div>
            </div>
          </form>
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
                type="submit"
                form="client-onboarding-form"
                variant="prismViolet"
                size="sm"
                disabled={isSubmitting || !clientNameVal?.trim()}
              >
                {isSubmitting ? 'Provisioning...' : 'Provision Client'}
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
