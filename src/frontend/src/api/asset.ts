import request, { apiDelete, apiGet, apiPost, apiPut } from './request'
import type {
  AgentImportResult,
  AssetItem,
  AssetQueryParams,
  AssetVersion,
  CreateAssetPayload,
  McpImportItem,
  PaginatedData,
  ParsedSkillMeta,
  PublishAssetPayload,
  UpdateAssetPayload
} from '@/types/api'

export function getAssets(
  params?: AssetQueryParams
): Promise<PaginatedData<AssetItem> | AssetItem[]> {
  return apiGet<PaginatedData<AssetItem> | AssetItem[]>('/assets', { params })
}

export function getAssetDetail(id: number): Promise<AssetItem> {
  return apiGet<AssetItem>(`/assets/${id}`)
}

export function createAsset(data: CreateAssetPayload): Promise<AssetItem> {
  return apiPost<AssetItem, CreateAssetPayload>('/assets', data)
}

export function updateAsset(id: number, data: UpdateAssetPayload): Promise<AssetItem> {
  return apiPut<AssetItem, UpdateAssetPayload>(`/assets/${id}`, data)
}

export function deleteAsset(id: number): Promise<null> {
  return apiDelete<null>(`/assets/${id}`)
}

export function publishAsset(id: number, data?: PublishAssetPayload): Promise<AssetItem> {
  return apiPost<AssetItem, PublishAssetPayload | undefined>(`/assets/${id}/publish`, data)
}

export function offlineAsset(id: number): Promise<AssetItem> {
  return apiPost<AssetItem>(`/assets/${id}/offline`)
}

export function getAssetVersions(
  id: number
): Promise<PaginatedData<AssetVersion> | AssetVersion[]> {
  return apiGet<PaginatedData<AssetVersion> | AssetVersion[]>(`/assets/${id}/versions`)
}

export function rollbackAsset(id: number, versionId: number): Promise<AssetItem> {
  return apiPost<AssetItem, { version_id: number }>(`/assets/${id}/rollback`, {
    version_id: versionId
  })
}

export function mcpHealthCheck(id: number): Promise<{ health_status: string; message?: string }> {
  return apiPost<{ health_status: string; message?: string }>(`/assets/${id}/mcp/health-check`)
}

export function mcpSyncTools(id: number): Promise<{ tools_schema: unknown[]; count?: number }> {
  return apiPost<{ tools_schema: unknown[]; count?: number }>(`/assets/${id}/mcp/sync-tools`)
}

export function subscribeAsset(id: number): Promise<null> {
  return apiPost<null>(`/assets/${id}/subscribe`)
}

export function parseSkillFile(file: File): Promise<ParsedSkillMeta> {
  const formData = new FormData()
  formData.append('file', file)
  return apiPost<ParsedSkillMeta, FormData>('/skills/parse-file', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export function importSkillFromUrl(url: string): Promise<ParsedSkillMeta> {
  return apiPost<ParsedSkillMeta, { url: string }>('/skills/import-url', { url })
}

export function uploadSkillPackage(assetId: number, file: File): Promise<AssetItem> {
  const formData = new FormData()
  formData.append('file', file)
  return apiPost<AssetItem, FormData>(`/assets/${assetId}/skill/package`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export async function downloadSkillPackage(
  assetId: number,
  format: 'zip' | 'md',
  filename: string
): Promise<void> {
  const res = await request.get<Blob>(`/assets/${assetId}/skill/download`, {
    params: { format },
    responseType: 'blob'
  })
  const blobUrl = URL.createObjectURL(res.data)
  const link = document.createElement('a')
  link.href = blobUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(blobUrl)
}

export function searchMcpRegistry(
  q: string,
  limit = 10
): Promise<PaginatedData<McpImportItem> | McpImportItem[]> {
  return apiGet<PaginatedData<McpImportItem> | McpImportItem[]>('/mcp/registry/search', {
    params: { q, limit }
  })
}

export function parseMcpJson(
  content: string
): Promise<McpImportItem | McpImportItem[]> {
  return apiPost<McpImportItem | McpImportItem[], { content: string }>('/mcp/parse-json', {
    content
  })
}

export function parseAgentFile(file: File): Promise<AgentImportResult> {
  const formData = new FormData()
  formData.append('file', file)
  return apiPost<AgentImportResult, FormData>('/agents/parse-file', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export function parseAgentText(
  content: string,
  filename?: string
): Promise<AgentImportResult> {
  return apiPost<AgentImportResult, { content: string; filename?: string }>(
    '/agents/parse-file',
    { content, filename }
  )
}

export function importAgentFromUrl(url: string): Promise<AgentImportResult> {
  return apiPost<AgentImportResult, { url: string }>('/agents/import-url', { url })
}

export async function downloadAgentConfig(
  assetId: number,
  format: 'md' | 'json',
  filename: string
): Promise<void> {
  const res = await request.get<Blob>(`/assets/${assetId}/agent/download`, {
    params: { format },
    responseType: 'blob'
  })
  const blobUrl = URL.createObjectURL(res.data)
  const link = document.createElement('a')
  link.href = blobUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(blobUrl)
}
