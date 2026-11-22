'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { Input } from '@/components/atoms/Input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/molecules/Select';
import { EmptyState, ErrorState } from '@/components/molecules/StateAlerts';
import {
  useApprovalsQuery,
  useApproveRequestMutation,
  useDenyRequestMutation,
} from '@/hooks/queries/useApprovalsQuery';
import type { ApiApprovalRequest, ApiApprovalStatus } from '@/lib/api';
import { getErrorMessage } from '@/types/ui';
import { Check, ShieldCheck, X } from 'lucide-react';
import { PermissionGuard } from '@/components/PermissionProvider';
import { toast } from 'sonner';

function statusBadgeVariant(status: ApiApprovalStatus) {
  switch (status) {
    case 'approved':
    case 'consumed':
      return 'success';
    case 'denied':
    case 'expired':
      return 'destructive';
    case 'pending':
    default:
      return 'violet';
  }
}

export default function ApprovalsPage() {
  const [statusFilter, setStatusFilter] = useState<string>('pending');
  const [notes, setNotes] = useState<Record<string, string>>({});

  const { data, isLoading, isError, refetch } = useApprovalsQuery(statusFilter);
  const approveMutation = useApproveRequestMutation();
  const denyMutation = useDenyRequestMutation();

  const items: ApiApprovalRequest[] = data?.data ?? [];

  const handleDecision = (item: ApiApprovalRequest, action: 'approve' | 'deny') => {
    const note = notes[item.id]?.trim() || '';
    const mutation = action === 'approve' ? approveMutation : denyMutation;
    mutation.mutate(
      { id: item.id, note },
      {
        onSuccess: () => {
          toast.success(
            action === 'approve'
              ? `Approved "${item.toolName}"`
              : `Denied "${item.toolName}"`
          );
        },
        onError: (err) => {
          toast.error(`Decision failed: ${getErrorMessage(err)}`);
        },
      }
    );
  };

  return (
    <AppLayout>
      <PageHeader
        title="Human Approval Inbox"
        description="Review, approve, or deny pending tool and MCP execution holds triggered by governance policies."
        extra={
          <div className="w-[180px]">
            <Select value={statusFilter} onValueChange={setStatusFilter}>
              <SelectTrigger className="h-8 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="pending">Pending</SelectItem>
                <SelectItem value="approved">Approved</SelectItem>
                <SelectItem value="consumed">Consumed</SelectItem>
                <SelectItem value="denied">Denied</SelectItem>
                <SelectItem value="expired">Expired</SelectItem>
                <SelectItem value="all">All</SelectItem>
              </SelectContent>
            </Select>
          </div>
        }
      />

      {isError ? (
        <ErrorState
          title="Failed to load approval requests"
          description="Could not retrieve pending governance approvals from the Prism API."
          onRetry={refetch}
        />
      ) : isLoading ? (
        <Card>
          <CardContent className="py-10 text-center text-xs text-muted-foreground">
            Loading approval requests...
          </CardContent>
        </Card>
      ) : items.length === 0 ? (
        <EmptyState
          title="No Approval Requests"
          description={
            statusFilter === 'pending'
              ? 'No tool or MCP calls are awaiting human review right now.'
              : `No ${statusFilter} approval requests found.`
          }
          icon={<ShieldCheck className="h-6 w-6" />}
        />
      ) : (
        <div className="space-y-4">
          {items.map((item) => {
            const busy = approveMutation.isPending || denyMutation.isPending;
            return (
              <Card key={item.id}>
                <CardHeader className="pb-2">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <CardTitle className="text-sm font-mono flex items-center gap-2">
                      <span>{item.toolName}</span>
                      {item.mcpServerName && (
                        <Badge variant="outline" className="text-[10px]">
                          MCP: {item.mcpServerName}
                        </Badge>
                      )}
                      {item.agentName && (
                        <Badge variant="outline" className="text-[10px]">
                          Agent: {item.agentName}
                        </Badge>
                      )}
                    </CardTitle>
                    <Badge variant={statusBadgeVariant(item.status)}>{item.status}</Badge>
                  </div>
                </CardHeader>
                <CardContent className="space-y-3 text-xs">
                  {item.reason && (
                    <p className="text-muted-foreground">
                      <span className="font-semibold text-foreground">Reason:</span> {item.reason}
                      {item.policyName ? ` (${item.policyName})` : ''}
                    </p>
                  )}

                  <div className="rounded border border-border bg-muted/30 p-2.5 font-mono text-[11px] overflow-x-auto">
                    <div className="text-[10px] text-muted-foreground mb-1">
                      ARGUMENTS (SHA-256: {item.argumentsHash.slice(0, 12)}...)
                    </div>
                    <pre className="whitespace-pre-wrap break-all">
                      {JSON.stringify(item.argumentsJson ?? {}, null, 2)}
                    </pre>
                  </div>

                  {item.status === 'pending' ? (
                    <div className="flex flex-wrap items-center gap-2 pt-1">
                      <Input
                        placeholder="Optional decision note..."
                        value={notes[item.id] ?? ''}
                        onChange={(e) =>
                          setNotes((prev) => ({ ...prev, [item.id]: e.target.value }))
                        }
                        className="h-8 text-xs max-w-sm"
                        disabled={busy}
                      />
                      <PermissionGuard permission="governance:update" disabledTooltip="You do not have permission to approve requests">
                        <Button
                          size="sm"
                          variant="prismViolet"
                          className="gap-1.5"
                          disabled={busy}
                          onClick={() => handleDecision(item, 'approve')}
                        >
                          <Check className="h-3.5 w-3.5" /> Approve
                        </Button>
                      </PermissionGuard>
                      <PermissionGuard permission="governance:update" disabledTooltip="You do not have permission to deny requests">
                        <Button
                          size="sm"
                          variant="outline"
                          className="gap-1.5 text-destructive hover:text-destructive"
                          disabled={busy}
                          onClick={() => handleDecision(item, 'deny')}
                        >
                          <X className="h-3.5 w-3.5" /> Deny
                        </Button>
                      </PermissionGuard>
                    </div>
                  ) : (
                    <div className="text-[11px] text-muted-foreground font-mono">
                      Requested: {new Date(item.createdAt).toLocaleString()}
                      {item.decidedAt ? ` · Decided: ${new Date(item.decidedAt).toLocaleString()}` : ''}
                      {item.decisionNote ? ` · Note: ${item.decisionNote}` : ''}
                    </div>
                  )}
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}
    </AppLayout>
  );
}
