import request, { apiDelete, apiGet, apiPost, apiPut } from './request'
import { deobfuscateBase64ToText, deobfuscateBytes } from '@/utils/obfuscate'
import type {
  CopyKbDocumentRequest,
  EditableKbTarget,
  KbBilibiliPreview,
  KbChunk,
  KbCommentItem,
  KbContentResult,
  KbDocument,
  KbDocumentInteractions,
  KbDocumentListResponse,
  KbDownloadPreview,
  KbReadingProgress,
  PaginatedData,
  UserBrief
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

function withRefDomainId<T extends Record<string, unknown>>(
  params?: T,
  refDomainId?: number
): (T & { ref_domain_id?: number }) | { ref_domain_id: number } | undefined {
  if (!refDomainId) return params
  return { ...(params ?? ({} as T)), ref_domain_id: refDomainId }
}

export interface GetDocumentsParams {
  lazy?: boolean
  folder_path?: string
  offset?: number
  limit?: number
  ext_group?: 'image' | 'txt'
  fields?: 'minimal'
}

export function getDocuments(
  kbId: number,
  params?: GetDocumentsParams,
  refDomainId?: number
): Promise<KbDocumentListResponse | KbDocument[]> {
  const normalizedParams: Record<string, unknown> | undefined = params
    ? {
        ...params,
        ...(params.lazy !== undefined ? { lazy: params.lazy ? 1 : 0 } : {})
      }
    : undefined
  return apiGet<KbDocumentListResponse | KbDocument[]>(`/kb/${kbId}/documents`, {
    params: withRefDomainId(normalizedParams, refDomainId)
  })
}

export function getDocumentMeta(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<KbDocument> {
  return apiGet<KbDocument>(`/kb/${kbAssetId}/documents/${docId}/meta`, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export async function getDocumentContent(
  kbId: number,
  docId: number,
  refDomainId?: number
): Promise<KbContentResult> {
  const res = await apiGet<KbContentResult>(`/kb/${kbId}/documents/${docId}/content`, {
    params: withRefDomainId({ obf: '1' }, refDomainId)
  })
  if (res.obfuscated && res.payload) {
    res.content = deobfuscateBase64ToText(res.payload)
  }
  return res
}

export async function getDocumentRawBlob(
  kbId: number,
  docId: number,
  fileName?: string,
  refDomainId?: number
): Promise<Blob> {
  const res = await request.get<ArrayBuffer>(`/kb/${kbId}/documents/${docId}/raw`, {
    params: withRefDomainId({ obf: '1' }, refDomainId),
    responseType: 'arraybuffer'
  })
  const rawBytes = new Uint8Array(res.data)
  const isObf = res.headers['x-ascp-obf'] === '1'
  const plainBytes = isObf ? deobfuscateBytes(rawBytes) : rawBytes
  return new Blob([plainBytes as Uint8Array<ArrayBuffer>], { type: inferMimeType(fileName) })
}

export function getDocumentChunks(
  kbId: number,
  docId: number,
  refDomainId?: number
): Promise<PaginatedData<KbChunk> | KbChunk[]> {
  return apiGet<PaginatedData<KbChunk> | KbChunk[]>(`/kb/${kbId}/documents/${docId}/chunks`, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export function deleteDocument(kbId: number, docId: number): Promise<null> {
  return apiDelete<null>(`/kb/${kbId}/documents/${docId}`)
}

export function restoreDocument(kbAssetId: number, docId: number): Promise<KbDocument> {
  return apiPost<KbDocument>(`/kb/${kbAssetId}/documents/${docId}/restore`)
}

export function getBilibiliPreview(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<KbBilibiliPreview> {
  return apiGet<KbBilibiliPreview>(`/kb/${kbAssetId}/documents/${docId}/bilibili`, {
    params: withRefDomainId(undefined, refDomainId)
  })
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
  new_name?: string,
  target_kb_id?: number,
  ref_domain_id?: number
): Promise<KbDocument> {
  const payload: CopyKbDocumentRequest = { target_folder_path }
  if (new_name !== undefined && new_name !== '') {
    payload.new_name = new_name
  }
  if (target_kb_id !== undefined) {
    payload.target_kb_id = target_kb_id
  }
  if (ref_domain_id !== undefined) {
    payload.ref_domain_id = ref_domain_id
  }
  return apiPost<KbDocument, CopyKbDocumentRequest>(
    `/kb/${kbId}/documents/${docId}/copy`,
    payload
  )
}

export function listEditableKbTargets(): Promise<EditableKbTarget[]> {
  return apiGet<EditableKbTarget[]>('/kb/editable-targets')
}

export function searchKB(
  kbId: number,
  data: { query: string; top_k?: number },
  refDomainId?: number
): Promise<PaginatedData<KbChunk> | KbChunk[]> {
  return apiPost<PaginatedData<KbChunk> | KbChunk[], { query: string; top_k?: number }>(
    `/kb/${kbId}/search`,
    data,
    { params: withRefDomainId(undefined, refDomainId) }
  )
}

export function getKbDownloadPreview(
  kbId: number,
  params: KbDownloadParams,
  refDomainId?: number
): Promise<KbDownloadPreview> {
  return apiGet<KbDownloadPreview>(`/kb/${kbId}/download/preview`, {
    params: withRefDomainId({ ...params }, refDomainId)
  })
}

export async function downloadKbFiles(
  kbId: number,
  params: KbDownloadParams,
  filename: string,
  refDomainId?: number
): Promise<void> {
  const res = await request.get<Blob>(`/kb/${kbId}/download`, {
    params: withRefDomainId({ ...params }, refDomainId),
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

export function getDocumentInteractions(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<KbDocumentInteractions> {
  return apiGet<KbDocumentInteractions>(`/kb/${kbAssetId}/documents/${docId}/interactions`, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export function toggleDocumentLike(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<{ liked_by_me?: boolean; liked?: boolean; like_count: number; like_users?: UserBrief[] }> {
  return apiPost<{
    liked_by_me?: boolean
    liked?: boolean
    like_count: number
    like_users?: UserBrief[]
  }>(`/kb/${kbAssetId}/documents/${docId}/like`, undefined, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export function toggleDocumentFavorite(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<{ favorited_by_me?: boolean; favorited?: boolean }> {
  return apiPost<{ favorited_by_me?: boolean; favorited?: boolean }>(
    `/kb/${kbAssetId}/documents/${docId}/favorite`,
    undefined,
    { params: withRefDomainId(undefined, refDomainId) }
  )
}

export function listDocumentComments(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<{ list: KbCommentItem[] } | KbCommentItem[]> {
  return apiGet<{ list: KbCommentItem[] } | KbCommentItem[]>(
    `/kb/${kbAssetId}/documents/${docId}/comments`,
    { params: withRefDomainId(undefined, refDomainId) }
  )
}

export function createDocumentComment(
  kbAssetId: number,
  docId: number,
  data: { content: string; quote_text?: string; parent_id?: number },
  refDomainId?: number
): Promise<KbCommentItem> {
  return apiPost<KbCommentItem, { content: string; quote_text?: string; parent_id?: number }>(
    `/kb/${kbAssetId}/documents/${docId}/comments`,
    data,
    { params: withRefDomainId(undefined, refDomainId) }
  )
}

export function deleteDocumentComment(
  kbAssetId: number,
  docId: number,
  commentId: number,
  refDomainId?: number
): Promise<null> {
  return apiDelete<null>(`/kb/${kbAssetId}/documents/${docId}/comments/${commentId}`, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export function renameDocument(
  kbAssetId: number,
  docId: number,
  newName: string
): Promise<KbDocument> {
  return apiPut<KbDocument, { new_name: string }>(
    `/kb/${kbAssetId}/documents/${docId}/rename`,
    { new_name: newName }
  )
}

export function getReadingProgress(
  kbAssetId: number,
  docId: number,
  refDomainId?: number
): Promise<KbReadingProgress> {
  return apiGet<KbReadingProgress>(`/kb/${kbAssetId}/documents/${docId}/reading-progress`, {
    params: withRefDomainId(undefined, refDomainId)
  })
}

export function saveReadingProgress(
  kbAssetId: number,
  docId: number,
  data: { current_page: number; total_pages: number },
  refDomainId?: number
): Promise<KbReadingProgress> {
  return apiPut<KbReadingProgress, { current_page: number; total_pages: number }>(
    `/kb/${kbAssetId}/documents/${docId}/reading-progress`,
    data,
    { params: withRefDomainId(undefined, refDomainId) }
  )
}

export function resolveVideoShortUrl(url: string): Promise<{ resolved_url: string }> {
  return apiGet<{ resolved_url: string }>('/tools/video-link/resolve', {
    params: { url }
  })
}
