import { api, resolveSignal, ApiSignal } from './client';
import type {
  ApiApprovalDecisionReq,
  ApiApprovalRequest,
  ApiApprovalsListRes,
} from './types/approval';

export async function apiGetApprovals(
  status?: string,
  page = 1,
  limit = 50,
  signal?: ApiSignal
): Promise<ApiApprovalsListRes> {
  const response = await api.get<ApiApprovalsListRes>('/approvals', {
    params: {
      ...(status ? { status } : {}),
      page,
      limit,
    },
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiGetApproval(id: string, signal?: ApiSignal): Promise<ApiApprovalRequest> {
  const response = await api.get<ApiApprovalRequest>(`/approvals/${id}`, {
    signal: resolveSignal(signal),
  });
  return response.data;
}

export async function apiApproveRequest(
  id: string,
  data: ApiApprovalDecisionReq = {}
): Promise<ApiApprovalRequest> {
  const response = await api.post<ApiApprovalRequest>(`/approvals/${id}/approve`, data);
  return response.data;
}

export async function apiDenyRequest(
  id: string,
  data: ApiApprovalDecisionReq = {}
): Promise<ApiApprovalRequest> {
  const response = await api.post<ApiApprovalRequest>(`/approvals/${id}/deny`, data);
  return response.data;
}
