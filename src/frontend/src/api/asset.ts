import { apiDelete, apiGet, apiPost, apiPut } from './request'
import type {
  AssetItem,
  AssetQueryParams,
  CreateAssetPayload,
  PaginatedData,
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

export function publishAsset(id: number): Promise<AssetItem> {
  return apiPost<AssetItem>(`/assets/${id}/publish`)
}

export function offlineAsset(id: number): Promise<AssetItem> {
  return apiPost<AssetItem>(`/assets/${id}/offline`)
}
