import request, { apiDelete, apiGet, apiPost, apiPut } from './request'
import { deobfuscateBase64ToText, deobfuscateBytes } from '@/utils/obfuscate'
import type {
  KbChunk,
  KbContentResult,
  KbDocument,
  KbDownloadPreview,
  PaginatedData
} from '@/types/api'

const IMAGE_MIME_MAP: Record<string, string> = {
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.svg': 'image/svg+xml',
  '.bmp': 'image/bmp',
  '.ico': 'image/x-icon',
  '.avif': 'image/avif'
}

function inferMimeType(fileName?: string): string {
  if (!fileName) return 'application/octet-stream'
  const dotIdx = fileName.lastIndexOf('.')
  if (dotIdx === -1) return 'application/octet-stream'
  const ext = fileName.slice(dotIdx).toLowerCase()
  return IMAGE_MIME_MAP[ext] ?? 'application/octet-stream'
}

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

export function createKbFile(
  kbId: number,
  data: { folder_path?: string; file_name: string; content: string }
): Promise<KbDocument> {
  return apiPost<KbDocument, { folder_path?: string; file_name: string; content: string }>(
    `/kb/${kbId}/files`,
    data
  )
}

export function updateKbFileContent(
  kbId: number,
  docId: number,
  data: { file_name?: string; content: string }
): Promise<KbContentResult> {
  return apiPut<KbContentResult, { file_name?: string; content: string }>(
    `/kb/${kbId}/documents/${docId}/content`,
    data
  )
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
  kbId: number,
  params?: { lazy?: boolean; folder_path?: string }
): Promise<PaginatedData<KbDocument> | KbDocument[]> {
  return apiGet<PaginatedData<KbDocument> | KbDocument[]>(`/kb/${kbId}/documents`, {
    params
  })
}

export async function getDocumentContent(kbId: number, docId: number): Promise<KbContentResult> {
  const res = await apiGet<KbContentResult>(`/kb/${kbId}/documents/${docId}/content`, {
    params: { obf: '1' }
  })
  if (res.obfuscated && res.payload) {
    res.content = deobfuscateBase64ToText(res.payload)
  }
  return res
}

export async function getDocumentRawBlob(
  kbId: number,
  docId: number,
  fileName?: string
): Promise<Blob> {
  const res = await request.get<ArrayBuffer>(`/kb/${kbId}/documents/${docId}/raw`, {
    params: { obf: '1' },
    responseType: 'arraybuffer'
  })
  const rawBytes = new Uint8Array(res.data)
  const isObf = res.headers['x-ascp-obf'] === '1'
  const plainBytes = isObf ? deobfuscateBytes(rawBytes) : rawBytes
  return new Blob([plainBytes as Uint8Array<ArrayBuffer>], { type: inferMimeType(fileName) })
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

export function moveDocument(
  kbId: number,
  docId: number,
  target_folder_path: string
): Promise<KbDocument> {
  return apiPost<KbDocument, { target_folder_path: string }>(
    `/kb/${kbId}/documents/${docId}/move`,
    { target_folder_path }
  )
}

export function copyDocument(
  kbId: number,
  docId: number,
  target_folder_path: string,
  new_name?: string
): Promise<KbDocument> {
  return apiPost<KbDocument, { target_folder_path: string; new_name?: string }>(
    `/kb/${kbId}/documents/${docId}/copy`,
    { target_folder_path, new_name }
  )
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
