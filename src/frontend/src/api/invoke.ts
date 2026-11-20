import { apiGet, apiPost } from './request'
import type { InvokeLogItem, PaginatedData } from '@/types/api'

export function invokeMCP(
  assetCode: string,
  payload: Record<string, unknown>
): Promise<unknown> {
  return apiPost<unknown, Record<string, unknown>>(`/invoke/mcp/${assetCode}`, payload)
}

export function invokeSkill(
  assetCode: string,
  payload: Record<string, unknown>
): Promise<unknown> {
  return apiPost<unknown, Record<string, unknown>>(`/invoke/skill/${assetCode}`, payload)
}

export function invokeAgent(
  assetCode: string,
  payload: Record<string, unknown>
): Promise<unknown> {
  return apiPost<unknown, Record<string, unknown>>(`/invoke/agent/${assetCode}`, payload)
}

export function invokeKBQuery(
  assetCode: string,
  payload: { query: string; top_k?: number }
): Promise<unknown> {
  return apiPost<unknown, { query: string; top_k?: number }>(
    `/invoke/kb/${assetCode}/query`,
    payload
  )
}

export function getInvokeLogs(params?: {
  page?: number
  page_size?: number
  asset_type?: string
  is_success?: number
}): Promise<PaginatedData<InvokeLogItem> | InvokeLogItem[]> {
  return apiGet<PaginatedData<InvokeLogItem> | InvokeLogItem[]>('/user/invoke-logs', { params })
}
