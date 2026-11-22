import request, { apiGet, apiPost } from './request'
import type { SevenOInspectResult, SevenOReleaseItem } from '@/types/api'

export function getSevenOReleases(): Promise<SevenOReleaseItem[]> {
  return apiGet<SevenOReleaseItem[]>('/tools/7o/releases')
}

export async function downloadSevenORelease(filename: string): Promise<Blob> {
  const res = await request.get<Blob>(`/tools/7o/releases/${encodeURIComponent(filename)}`, {
    responseType: 'blob'
  })
  return res.data
}

export async function encryptSevenO(formData: FormData): Promise<Blob> {
  const res = await request.post<Blob>('/tools/7o/encrypt', formData, {
    responseType: 'blob',
    timeout: 120000,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return res.data
}

export async function decryptSevenO(formData: FormData): Promise<Blob> {
  const res = await request.post<Blob>('/tools/7o/decrypt', formData, {
    responseType: 'blob',
    timeout: 120000,
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return res.data
}

export function inspectSevenO(file: File): Promise<SevenOInspectResult> {
  const formData = new FormData()
  formData.append('file', file)
  return apiPost<SevenOInspectResult, FormData>('/tools/7o/inspect', formData, {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
}

export function triggerBlobDownload(blob: Blob, filename: string): void {
  const blobUrl = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = blobUrl
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(blobUrl)
}

export async function parseBlobErrorMessage(error: unknown, fallback: string): Promise<string> {
  const maybeErr = error as {
    response?: { data?: unknown }
    message?: string
  }
  const data = maybeErr?.response?.data
  if (data instanceof Blob) {
    try {
      const text = await data.text()
      const parsed = JSON.parse(text) as { message?: string }
      if (parsed?.message) {
        return parsed.message
      }
    } catch {
      // Ignore parse error and use fallback
    }
  }
  return maybeErr?.message || fallback
}
