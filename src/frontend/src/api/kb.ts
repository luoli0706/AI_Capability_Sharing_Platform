import request, { apiDelete, apiGet, apiPost } from './request'
import type {
  KbChunk,
  KbContentResult,
  KbDocument,
  KbDownloadPreview,
  PaginatedData
} from '@/types/api'

export interface KbDownloadParams {
  scope: 'file' | 'folder' | 'all'
  doc_id?: number
  folder_path?: string
}

export function createKbFolder(kbId: number, folder_path: string): Promise<KbDocument> {
  return apiPost<KbDocument, { folder_path: string }>(`/kb/${kbId}/folders`, {
    folder_path
  })
}

export function uploadDocument(
  kbId: number,
  file: File,
  folder_path?: string,
  relative_path?: string
): Promise<KbDocument> {
  const formData = new FormData()
  formData.append('file', file)
  if (folder_path) {
    formData.append('folder_path', folder_path)
  }
  if (relative_path) {
    formData.append('relative_path', relative_path)
  }
  return apiPost<KbDocument, FormData>(`/kb/${kbId}/documents`, formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export function getDocuments(
  kbId: number
): Promise<PaginatedData<KbDocument> | KbDocument[]> {
  return apiGet<PaginatedData<KbDocument> | KbDocument[]>(`/kb/${kbId}/documents`)
}

export function getDocumentContent(kbId: number, docId: number): Promise<KbContentResult> {
  return apiGet<KbContentResult>(`/kb/${kbId}/documents/${docId}/content`)
}

export function getDocumentChunks(
  kbId: number,
  docId: number
): Promise<PaginatedData<KbChunk> | KbChunk[]> {
  return apiGet<PaginatedData<KbChunk> | KbChunk[]>(`/kb/${kbId}/documents/${docId}/chunks`)
}

export function deleteDocument(kbId: number, docId: number): Promise<null> {
  return apiDelete<null>(`/kb/${kbId}/documents/${docId}`)
}

export function searchKB(
  kbId: number,
  data: { query: string; top_k?: number }
): Promise<PaginatedData<KbChunk> | KbChunk[]> {
  return apiPost<PaginatedData<KbChunk> | KbChunk[], { query: string; top_k?: number }>(
    `/kb/${kbId}/search`,
    data
  )
}

export function getKbDownloadPreview(
  kbId: number,
  params: KbDownloadParams
): Promise<KbDownloadPreview> {
  return apiGet<KbDownloadPreview>(`/kb/${kbId}/download/preview`, { params })
}

export async function downloadKbFiles(
  kbId: number,
  params: KbDownloadParams,
  filename: string
): Promise<void> {
  const res = await request.get<Blob>(`/kb/${kbId}/download`, {
    params,
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
