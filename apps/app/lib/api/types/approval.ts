export type ApiApprovalStatus = 'pending' | 'approved' | 'denied' | 'expired' | 'consumed';

export interface ApiApprovalRequest {
  id: string;
  orgId?: string;
  userId: string;
  agentId?: string;
  agentName?: string;
  toolName: string;
  mcpServerName?: string;
  argumentsJson: Record<string, unknown>;
  argumentsHash: string;
  policyId?: string;
  policyName?: string;
  reason?: string;
  status: ApiApprovalStatus;
  decidedBy?: string;
  decisionNote?: string;
  decidedAt?: string;
  expiresAt: string;
  consumedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface ApiApprovalsListRes {
  data: ApiApprovalRequest[];
  total: number;
  page: number;
  pageSize: number;
}

export interface ApiApprovalDecisionReq {
  note?: string;
}
