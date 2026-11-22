import { apiDelete, apiGet, apiPost } from './request'
import type {
  CreateCrossRefPayload,
  CrossRefSourceItemsResult,
  DomainCrossRefItem,
  DomainItem,
  PaginatedData
} from '@/types/api'

export function getDomainCrossRefs(
  domainId: number,
  includePending = true
): Promise<PaginatedData<DomainCrossRefItem> | DomainCrossRefItem[]> {
  return apiGet<PaginatedData<DomainCrossRefItem> | DomainCrossRefItem[]>(
    `/domains/${domainId}/cross-refs`,
    {
      params: includePending ? { include_pending: 1 } : undefined
    }
  )
}

export function createCrossRef(
  domainId: number,
  data: CreateCrossRefPayload
): Promise<DomainCrossRefItem> {
  return apiPost<DomainCrossRefItem, CreateCrossRefPayload>(
    `/domains/${domainId}/cross-refs`,
    data
  )
}

export function deleteCrossRef(domainId: number, refId: number): Promise<null> {
  return apiDelete<null>(`/domains/${domainId}/cross-refs/${refId}`)
}

export function getCrossRefSourceDomains(
  excludeDomainId: number
): Promise<PaginatedData<DomainItem> | DomainItem[]> {
  return apiGet<PaginatedData<DomainItem> | DomainItem[]>('/cross-refs/source-domains', {
    params: { exclude_domain_id: excludeDomainId }
  })
}

export function getCrossRefSourceItems(
  sourceDomainId: number
): Promise<CrossRefSourceItemsResult> {
  return apiGet<CrossRefSourceItemsResult>('/cross-refs/source-items', {
    params: { source_domain_id: sourceDomainId }
  })
}
