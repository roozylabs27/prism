'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { useMembersQuery } from '@/hooks/queries/useMembersQuery';
import { usePendingInvitesQuery } from '@/hooks/queries/useInvitesQuery';
import {
  useRevokeInviteMutation,
  useRemoveMemberMutation,
  useUpdateMemberRoleMutation,
} from '@/hooks/mutations/useMemberMutations';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/molecules/Select';
import { Users, Shield, UserPlus, Mail, Calendar, Clock, X, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { parseApiError } from '@/lib/http/errors';
import { InviteMemberSheet } from './_components/InviteMemberSheet';

export default function MembersPage() {
  const { data: members, isLoading, isError, refetch } = useMembersQuery();
  const { data: invites, isLoading: invitesLoading, refetch: refetchInvites } = usePendingInvitesQuery();
  const { mutateAsync: revokeInvite, isPending: revoking } = useRevokeInviteMutation();
  const { mutateAsync: updateMemberRole, isPending: updatingRole } = useUpdateMemberRoleMutation();
  const { mutateAsync: removeMember, isPending: removingMember } = useRemoveMemberMutation();
  const [drawerOpen, setDrawerOpen] = useState(false);

  const handleRoleChange = async (userId: string, role: string) => {
    try {
      await updateMemberRole({ memberId: userId, role });
      refetch();
    } catch (err: unknown) {
      toast.error(parseApiError(err, 'Failed to update member role').message);
    }
  };

  const handleRemoveMember = async (userId: string) => {
    try {
      await removeMember(userId);
      refetch();
    } catch (err: unknown) {
      toast.error(parseApiError(err, 'Failed to remove member').message);
    }
  };

  const handleRevoke = async (inviteId: string) => {
    try {
      await revokeInvite(inviteId);
    } catch (err: unknown) {
      toast.error(parseApiError(err, 'Failed to revoke invitation').message);
    }
  };

  return (
    <AppLayout>
      <PageHeader
        title="Organization Team Members"
        description="Manage user access roles and team permissions."
        extra={
          <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={() => setDrawerOpen(true)}>
            <UserPlus className="h-4 w-4" /> Invite Member
          </Button>
        }
      />

      {isError && <ErrorState onRetry={refetch} />}

      {!isError && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Users className="h-4 w-4 text-primary" />
              <span>Active Team Members ({members?.length || 0})</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <p className="text-xs text-muted-foreground">Loading members...</p>
            ) : !members || members.length === 0 ? (
              <EmptyState description="No active team members found." />
            ) : (
              <div className="divide-y divide-border">
                {members.map((member) => (
                  <div key={member.id || member.userId} className="py-3.5 first:pt-0 last:pb-0 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="h-8 w-8 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                        <Shield className="h-4 w-4" />
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-xs text-foreground">
                            {member.userName || member.userEmail || member.userId}
                          </span>
                          <Badge variant={member.role === 'owner' ? 'violet' : member.role === 'admin' ? 'default' : 'outline'} className="text-[10px] capitalize">
                            {member.role}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-3 text-[11px] text-muted-foreground mt-0.5">
                          {member.userEmail && (
                            <span className="flex items-center gap-1">
                              <Mail className="h-3 w-3" /> {member.userEmail}
                            </span>
                          )}
                          {member.createdAt && (
                            <span className="flex items-center gap-1">
                              <Calendar className="h-3 w-3" /> Joined {new Date(member.createdAt).toLocaleDateString()}
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Select
                        value={member.role}
                        onValueChange={(nextRole) => handleRoleChange(member.userId, nextRole)}
                        disabled={updatingRole}
                      >
                        <SelectTrigger className="h-8 w-[155px] text-xs capitalize">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="owner">Owner</SelectItem>
                          <SelectItem value="developer">Developer</SelectItem>
                          <SelectItem value="agent_manager">Agent Manager</SelectItem>
                          <SelectItem value="finops_manager">FinOps Manager</SelectItem>
                          <SelectItem value="auditor">Security Auditor</SelectItem>
                          <SelectItem value="viewer">Read-Only Viewer</SelectItem>
                        </SelectContent>
                      </Select>
                      <ConfirmDialog
                        title="Remove Organization Member"
                        description={`Remove ${member.userName || member.userEmail || member.userId} from this organization?`}
                        confirmText="Remove"
                        onConfirm={() => handleRemoveMember(member.userId)}
                        trigger={
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-8 w-8 text-destructive hover:text-destructive"
                            disabled={removingMember}
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                          </Button>
                        }
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <Card className="mt-4">
        <CardHeader>
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <Clock className="h-4 w-4 text-primary" />
            <span>Pending Invitations ({invites?.length || 0})</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {invitesLoading ? (
            <p className="text-xs text-muted-foreground">Loading invitations...</p>
          ) : !invites || invites.length === 0 ? (
            <EmptyState description="No pending invitations. Invite a member to get started." />
          ) : (
            <div className="divide-y divide-border">
              {invites.map((invite) => (
                <div key={invite.id} className="py-3.5 first:pt-0 last:pb-0 flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-xs text-foreground truncate">{invite.email}</span>
                      <Badge variant="outline" className="text-[10px] capitalize">
                        {invite.role || invite.roleId}
                      </Badge>
                    </div>
                    <div className="flex items-center gap-3 text-[11px] text-muted-foreground mt-0.5">
                      <span className="flex items-center gap-1">
                        <Calendar className="h-3 w-3" /> Expires {new Date(invite.expiresAt).toLocaleDateString()}
                      </span>
                    </div>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    className="gap-1 shrink-0"
                    disabled={revoking}
                    onClick={() => handleRevoke(invite.id)}
                  >
                    <X className="h-3.5 w-3.5" /> Revoke
                  </Button>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <InviteMemberSheet
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        onInviteSuccess={() => {
          refetch();
          refetchInvites();
        }}
      />
    </AppLayout>
  );
}
