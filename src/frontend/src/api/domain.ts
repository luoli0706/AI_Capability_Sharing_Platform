import { apiDelete, apiGet, apiPost, apiPut } from './request'
import type {
  AddDomainMemberPayload,
  ApprovalRequest,
  CreateDomainPayload,
  DomainItem,
  DomainMember,
  PaginatedData,
  UpdateDomainJoinSettingsPayload,
  UpdateDomainPayload
} from '@/types/api'

export interface DomainQueryParams {
  tab?: 'mine' | 'square'
  keyword?: string
  domain_uid?: string
  org_id?: number
  page?: number
  page_size?: number
}

export function getDomains(
  params?: DomainQueryParams
): Promise<PaginatedData<DomainItem> | DomainItem[]> {
  return apiGet<PaginatedData<DomainItem> | DomainItem[]>('/domains', { params })
}

export function createDomain(data: CreateDomainPayload): Promise<DomainItem> {
  return apiPost<DomainItem, CreateDomainPayload>('/domains', data)
}

export function updateDomain(id: number, data: UpdateDomainPayload): Promise<DomainItem> {
  return apiPut<DomainItem, UpdateDomainPayload>(`/domains/${id}`, data)
}

export function deleteDomain(id: number): Promise<null> {
  return apiDelete<null>(`/domains/${id}`)
}

export function getDomainMembers(
  domainId: number
): Promise<PaginatedData<DomainMember> | DomainMember[]> {
  return apiGet<PaginatedData<DomainMember> | DomainMember[]>(`/domains/${domainId}/members`)
}

export function addDomainMember(
  domainId: number,
  data: AddDomainMemberPayload
): Promise<null> {
  return apiPost<null, AddDomainMemberPayload>(`/domains/${domainId}/members`, data)
}

export function updateDomainMemberRole(
  domainId: number,
  userId: number,
  domain_role: 'admin' | 'member'
): Promise<null> {
  return apiPut<null, { domain_role: 'admin' | 'member' }>(
    `/domains/${domainId}/members/${userId}`,
    { domain_role }
  )
}

export function removeDomainMember(domainId: number, userId: number): Promise<null> {
  return apiDelete<null>(`/domains/${domainId}/members/${userId}`)
}

export function updateDomainJoinSettings(
  domainId: number,
  payload: UpdateDomainJoinSettingsPayload
): Promise<DomainItem> {
  return apiPut<DomainItem, UpdateDomainJoinSettingsPayload>(
    `/domains/${domainId}/join-settings`,
    payload
  )
}

export function applyJoinDomain(
  domainId: number,
  reason?: string
): Promise<ApprovalRequest> {
  return apiPost<ApprovalRequest, { reason?: string }>(
    `/domains/${domainId}/join-requests`,
    { reason }
  )
}

export function joinDomainByQuiz(
  domainId: number,
  answer: string
): Promise<DomainItem | null> {
  return apiPost<DomainItem | null, { answer: string }>(
    `/domains/${domainId}/join-by-quiz`,
    { answer }
  )
}
