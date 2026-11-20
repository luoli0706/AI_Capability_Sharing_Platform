import { apiDelete, apiGet, apiPost, apiPut } from './request'
import type {
  AddOrgMemberPayload,
  CreateOrgPayload,
  OrgDetail,
  OrgItem,
  OrgMember,
  PaginatedData,
  UpdateOrgPayload
} from '@/types/api'

export function getOrgs(params?: {
  page?: number
  page_size?: number
  keyword?: string
}): Promise<PaginatedData<OrgItem> | OrgItem[]> {
  return apiGet<PaginatedData<OrgItem> | OrgItem[]>('/orgs', { params })
}

export function createOrg(data: CreateOrgPayload): Promise<OrgItem> {
  return apiPost<OrgItem, CreateOrgPayload>('/orgs', data)
}

export function updateOrg(id: number, data: UpdateOrgPayload): Promise<OrgItem> {
  return apiPut<OrgItem, UpdateOrgPayload>(`/orgs/${id}`, data)
}

export function deleteOrg(id: number): Promise<null> {
  return apiDelete<null>(`/orgs/${id}`)
}

export function getOrgDetail(id: number): Promise<OrgDetail> {
  return apiGet<OrgDetail>(`/orgs/${id}`)
}

export function getOrgMembers(
  orgId: number
): Promise<PaginatedData<OrgMember> | OrgMember[]> {
  return apiGet<PaginatedData<OrgMember> | OrgMember[]>(`/orgs/${orgId}/members`)
}

export function addOrgMember(
  orgId: number,
  data: AddOrgMemberPayload
): Promise<null> {
  return apiPost<null, AddOrgMemberPayload>(`/orgs/${orgId}/members`, data)
}

export function removeOrgMember(orgId: number, userId: number): Promise<null> {
  return apiDelete<null>(`/orgs/${orgId}/members/${userId}`)
}
