import { apiGet, apiPost } from './request'
import type {
  AddDomainMemberPayload,
  CreateDomainPayload,
  DomainItem,
  DomainMember,
  PaginatedData
} from '@/types/api'

export function getDomains(params?: {
  page?: number
  page_size?: number
  keyword?: string
}): Promise<PaginatedData<DomainItem> | DomainItem[]> {
  return apiGet<PaginatedData<DomainItem> | DomainItem[]>('/domains', { params })
}

export function createDomain(data: CreateDomainPayload): Promise<DomainItem> {
  return apiPost<DomainItem, CreateDomainPayload>('/domains', data)
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
