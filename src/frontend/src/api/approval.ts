import { apiGet, apiPost } from './request'
import type {
  ApprovalRequest,
  CreateApprovalPayload,
  HandleApprovalPayload,
  PaginatedData
} from '@/types/api'

export function createApproval(data: CreateApprovalPayload): Promise<ApprovalRequest> {
  return apiPost<ApprovalRequest, CreateApprovalPayload>('/approvals', data)
}

export function getMyRequests(params?: {
  page?: number
  page_size?: number
  status?: string
}): Promise<PaginatedData<ApprovalRequest> | ApprovalRequest[]> {
  return apiGet<PaginatedData<ApprovalRequest> | ApprovalRequest[]>('/approvals/my-requests', {
    params
  })
}

export function getPendingApprovals(params?: {
  page?: number
  page_size?: number
}): Promise<PaginatedData<ApprovalRequest> | ApprovalRequest[]> {
  return apiGet<PaginatedData<ApprovalRequest> | ApprovalRequest[]>('/approvals/pending', {
    params
  })
}

export function getHandledApprovals(params?: {
  page?: number
  page_size?: number
}): Promise<PaginatedData<ApprovalRequest> | ApprovalRequest[]> {
  return apiGet<PaginatedData<ApprovalRequest> | ApprovalRequest[]>('/approvals/handled', {
    params
  })
}

export function handleApproval(
  id: number,
  data: HandleApprovalPayload
): Promise<ApprovalRequest> {
  return apiPost<ApprovalRequest, HandleApprovalPayload>(`/approvals/${id}/handle`, data)
}
