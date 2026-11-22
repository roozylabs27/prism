'use client';

import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/molecules/Select';
import { PermissionGuard } from '@/components/PermissionProvider';
import { apiExportPilotOutcomes, type ApiPilotOutcomesReport, type ApiWorkspace } from '@/lib/api';
import { DollarSign, Download, FileText, Rocket } from 'lucide-react';
import { toast } from 'sonner';

const escapeHtml = (value: string) =>
  value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');

interface DashboardPilotOutcomesCardProps {
  pilotOutcomes: ApiPilotOutcomesReport;
  pilotWorkspaceId: string;
  onPilotWorkspaceChange: (workspaceId: string) => void;
  workspaces: ApiWorkspace[];
  selectedDays: number;
  activePilotWorkspace?: string;
  onOpenOnboarding: () => void;
}

export function DashboardPilotOutcomesCard({
  pilotOutcomes,
  pilotWorkspaceId,
  onPilotWorkspaceChange,
  workspaces,
  selectedDays,
  activePilotWorkspace,
  onOpenOnboarding,
}: DashboardPilotOutcomesCardProps) {
  const handleDownloadPilotOutcomes = async (format: 'csv' | 'json') => {
    try {
      const blob = await apiExportPilotOutcomes(format, selectedDays, activePilotWorkspace);
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `prism-pilot-outcomes-${selectedDays || 30}d.${format}`;
      link.click();
      URL.revokeObjectURL(url);
      toast.success(`Exported pilot outcomes (${format.toUpperCase()})`);
    } catch {
      toast.error('Failed to export pilot outcomes');
    }
  };

  const handleExportPilotPdf = () => {
    const printWin = window.open('', '_blank', 'width=960,height=720');
    if (!printWin) {
      toast.error('Allow popups in your browser to generate the Executive PDF report');
      return;
    }

    const wsRows = (pilotOutcomes.workspaces ?? [])
      .map(
        (ws) => `
          <tr>
            <td>${escapeHtml(ws.workspaceName || ws.workspaceId)}</td>
            <td>${ws.totalRequests.toLocaleString()}</td>
            <td>${(ws.errorRatePct ?? 0).toFixed(2)}%</td>
            <td>$${ws.actualCostUsd.toFixed(2)}</td>
            <td>$${ws.estimatedSavingsUsd.toFixed(2)}</td>
            <td>${Math.round(ws.p95LatencyMs ?? ws.avgLatencyMs)} ms</td>
          </tr>`
      )
      .join('');

    const html = `<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <title>RoozyLabs Prism Executive Pilot Outcomes (${pilotOutcomes.windowDays}d)</title>
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; color: #111827; padding: 32px; }
    h1 { font-size: 20px; margin: 0 0 4px; }
    p.meta { font-size: 12px; color: #6b7280; margin: 0 0 20px; }
    .kpis { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12px; margin-bottom: 24px; }
    .kpi { border: 1px solid #e5e7eb; border-radius: 6px; padding: 12px; }
    .kpi span { display: block; font-size: 10px; text-transform: uppercase; color: #6b7280; }
    .kpi strong { font-size: 18px; }
    table { width: 100%; border-collapse: collapse; font-size: 12px; margin-top: 12px; }
    th, td { border-bottom: 1px solid #e5e7eb; text-align: left; padding: 8px; }
    th { background: #f9fafb; font-weight: 600; }
    @media print { body { padding: 12px; } }
  </style>
</head>
<body>
  <h1>RoozyLabs Prism — Executive Value Attribution Report</h1>
  <p class="meta">Org: ${escapeHtml(pilotOutcomes.orgId)} · Window: ${pilotOutcomes.windowDays} days · Generated: ${escapeHtml(new Date().toISOString())}</p>
  <p class="meta">${escapeHtml(pilotOutcomes.methodologyNote)}</p>
  <div class="kpis">
    <div class="kpi"><span>Actual Spend</span><strong>$${pilotOutcomes.totals.actualCostUsd.toFixed(2)}</strong></div>
    <div class="kpi"><span>Smart Routed Savings</span><strong>$${pilotOutcomes.totals.estimatedSavingsUsd.toFixed(2)}</strong></div>
    <div class="kpi"><span>P95 Latency</span><strong>${Math.round(pilotOutcomes.totals.p95LatencyMs)} ms</strong></div>
    <div class="kpi"><span>Policy &amp; Quota Blocks</span><strong>${pilotOutcomes.governance.policyDenials403 + pilotOutcomes.governance.budgetQuotaDenials429}</strong></div>
  </div>
  <h2>Client Workspace Attribution</h2>
  <table>
    <thead><tr><th>Workspace</th><th>Requests</th><th>Error Rate</th><th>Actual Spend</th><th>Savings</th><th>P95 Latency</th></tr></thead>
    <tbody>${wsRows || '<tr><td colspan="6">No workspace requests recorded in window.</td></tr>'}</tbody>
  </table>
</body>
</html>`;

    printWin.document.open();
    printWin.document.write(html);
    printWin.document.close();
    printWin.focus();
    printWin.print();
  };

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <CardTitle className="flex items-center gap-2 text-sm font-semibold">
            <DollarSign className="h-4 w-4 text-emerald-500" />
            <span>Client Value Attribution &amp; Pilot Outcomes ({pilotOutcomes.windowDays}d)</span>
          </CardTitle>
          <p className="text-[11px] text-muted-foreground mt-0.5">{pilotOutcomes.methodologyNote}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Select value={pilotWorkspaceId} onValueChange={onPilotWorkspaceChange}>
            <SelectTrigger className="h-8 w-[180px] text-xs bg-card border-border">
              <SelectValue placeholder="All Client Workspaces" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all" className="text-xs">
                All Client Workspaces
              </SelectItem>
              {workspaces.map((ws) => (
                <SelectItem key={ws.id} value={ws.id} className="text-xs">
                  {ws.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <PermissionGuard
            permission="organization:update"
            disabledTooltip="Organization update permission required to onboard client workspaces"
          >
            <Button variant="outline" size="sm" className="h-8 gap-1 text-xs" onClick={onOpenOnboarding}>
              <Rocket className="h-3.5 w-3.5 text-prism-violet" /> Onboard Client
            </Button>
          </PermissionGuard>
          <Button
            variant="outline"
            size="sm"
            className="h-8 gap-1 text-xs"
            onClick={() => handleDownloadPilotOutcomes('csv')}
          >
            <Download className="h-3.5 w-3.5" /> CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="h-8 gap-1 text-xs"
            onClick={() => handleDownloadPilotOutcomes('json')}
          >
            <Download className="h-3.5 w-3.5" /> JSON
          </Button>
          <Button variant="prismViolet" size="sm" className="h-8 gap-1 text-xs" onClick={handleExportPilotPdf}>
            <FileText className="h-3.5 w-3.5" /> Executive PDF
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 grid-cols-2 lg:grid-cols-4 font-mono text-xs">
          <div className="rounded border border-border bg-muted/20 p-3">
            <span className="text-[10px] text-muted-foreground block">SMART ROUTED SAVINGS</span>
            <span className="text-base font-bold text-emerald-600 dark:text-emerald-400">
              ${pilotOutcomes.totals.estimatedSavingsUsd.toFixed(2)}
            </span>
          </div>
          <div className="rounded border border-border bg-muted/20 p-3">
            <span className="text-[10px] text-muted-foreground block">P95 LATENCY</span>
            <span className="text-base font-bold text-foreground">
              {Math.round(pilotOutcomes.totals.p95LatencyMs)}ms
            </span>
          </div>
          <div className="rounded border border-border bg-muted/20 p-3">
            <span className="text-[10px] text-muted-foreground block">POLICY / QUOTA BLOCKS</span>
            <span className="text-base font-bold text-foreground">
              {pilotOutcomes.governance.policyDenials403 + pilotOutcomes.governance.budgetQuotaDenials429}
            </span>
          </div>
          <div className="rounded border border-border bg-muted/20 p-3">
            <span className="text-[10px] text-muted-foreground block">WORKSPACES TRACKED</span>
            <span className="text-base font-bold text-foreground">
              {pilotOutcomes.workspaces?.length ?? 0}
            </span>
          </div>
        </div>

        {pilotOutcomes.workspaces && pilotOutcomes.workspaces.length > 0 && (
          <div className="overflow-x-auto rounded border border-border">
            <table className="w-full text-xs">
              <thead className="bg-muted/30 text-[10px] uppercase tracking-wider text-muted-foreground">
                <tr>
                  <th className="px-3 py-2 text-left">Client Workspace</th>
                  <th className="px-3 py-2 text-right">Requests</th>
                  <th className="px-3 py-2 text-right">Error %</th>
                  <th className="px-3 py-2 text-right">Actual Spend</th>
                  <th className="px-3 py-2 text-right">Estimated Savings</th>
                  <th className="px-3 py-2 text-right">P95 Latency</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border font-mono">
                {pilotOutcomes.workspaces.map((ws) => (
                  <tr key={ws.workspaceId} className="hover:bg-muted/10">
                    <td className="px-3 py-2 font-sans font-medium text-foreground">
                      {ws.workspaceName || ws.workspaceId}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">{ws.totalRequests.toLocaleString()}</td>
                    <td className="px-3 py-2 text-right tabular-nums">{(ws.errorRatePct ?? 0).toFixed(2)}%</td>
                    <td className="px-3 py-2 text-right tabular-nums">${ws.actualCostUsd.toFixed(2)}</td>
                    <td className="px-3 py-2 text-right tabular-nums text-emerald-600 dark:text-emerald-400">
                      ${ws.estimatedSavingsUsd.toFixed(2)}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {Math.round(ws.p95LatencyMs ?? ws.avgLatencyMs)} ms
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
