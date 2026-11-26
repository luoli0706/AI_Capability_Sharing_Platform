import { apiDelete, apiGet, apiPost } from './request'

export type DomainEventType = 'folder_vote' | 'lottery' | 'matching'
export type DomainEventStatus = 'upcoming' | 'active' | 'ended'

export interface DomainEventSummary {
  id: number
  domain_id: number
  title: string
  description: string
  event_type: DomainEventType | string
  allow_multi_vote: boolean
  config_json?: string
  creator_id: number
  start_time: string
  end_time: string
  closed_at?: string | null
  created_at: string
  updated_at: string
  status: DomainEventStatus
  creator_name: string
  candidate_count: number
  total_votes: number
  can_manage: boolean
}

export interface DomainEventCandidateView {
  id: number
  event_id: number
  kb_asset_id: number
  kb_name: string
  folder_doc_id: number
  folder_path: string
  folder_name: string
  added_by_user_id: number
  vote_count: number
  created_at: string
  updated_at: string
  added_by_name: string
  has_voted: boolean
}

export interface DomainEventDetail extends DomainEventSummary {
  candidates: DomainEventCandidateView[]
  my_voted_candidate_ids: number[]
}

export interface CreateDomainEventRequest {
  title: string
  description?: string
  event_type?: DomainEventType
  allow_multi_vote?: boolean
  start_time: string
  end_time: string
}

export interface AddDomainEventCandidateRequest {
  kb_asset_id: number
  folder_doc_id: number
}

export function listDomainEvents(
  domainId: number,
  params?: { status?: string; event_type?: string }
): Promise<DomainEventSummary[]> {
  return apiGet<DomainEventSummary[]>(`/domains/${domainId}/events`, { params })
}

export function createDomainEvent(
  domainId: number,
  data: CreateDomainEventRequest
): Promise<DomainEventSummary> {
  return apiPost<DomainEventSummary, CreateDomainEventRequest>(
    `/domains/${domainId}/events`,
    data
  )
}

export function getDomainEventDetail(
  domainId: number,
  eventId: number
): Promise<DomainEventDetail> {
  return apiGet<DomainEventDetail>(`/domains/${domainId}/events/${eventId}`)
}

export function addDomainEventCandidate(
  domainId: number,
  eventId: number,
  data: AddDomainEventCandidateRequest
): Promise<DomainEventDetail> {
  return apiPost<DomainEventDetail, AddDomainEventCandidateRequest>(
    `/domains/${domainId}/events/${eventId}/candidates`,
    data
  )
}

export function voteDomainEventCandidate(
  domainId: number,
  eventId: number,
  candidateId: number
): Promise<DomainEventDetail> {
  return apiPost<DomainEventDetail>(
    `/domains/${domainId}/events/${eventId}/candidates/${candidateId}/vote`
  )
}

export function unvoteDomainEventCandidate(
  domainId: number,
  eventId: number,
  candidateId: number
): Promise<DomainEventDetail> {
  return apiDelete<DomainEventDetail>(
    `/domains/${domainId}/events/${eventId}/candidates/${candidateId}/vote`
  )
}

export function closeDomainEvent(domainId: number, eventId: number): Promise<void> {
  return apiPost<void>(`/domains/${domainId}/events/${eventId}/close`)
}

export function deleteDomainEvent(domainId: number, eventId: number): Promise<void> {
  return apiDelete<void>(`/domains/${domainId}/events/${eventId}`)
}
