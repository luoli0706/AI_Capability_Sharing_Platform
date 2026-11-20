import { apiDelete, apiGet, apiPost, apiPut } from './request'
import type {
  AssetItem,
  CreateApiKeyPayload,
  CreateApiKeyResult,
  LoginPayload,
  LoginResult,
  PaginatedData,
  RegisterPayload,
  UpdatePasswordPayload,
  UpdateProfilePayload,
  UserApiKey,
  UserInfo
} from '@/types/api'

export function register(data: RegisterPayload): Promise<{ user: UserInfo }> {
  return apiPost<{ user: UserInfo }, RegisterPayload>('/auth/register', data)
}

export function login(data: LoginPayload): Promise<LoginResult> {
  return apiPost<LoginResult, LoginPayload>('/auth/login', data)
}

export function logout(): Promise<null> {
  return apiPost<null>('/auth/logout')
}

export function getProfile(): Promise<UserInfo> {
  return apiGet<UserInfo>('/user/profile')
}

export function updateProfile(data: UpdateProfilePayload): Promise<UserInfo> {
  return apiPut<UserInfo, UpdateProfilePayload>('/user/profile', data)
}

export function updatePassword(data: UpdatePasswordPayload): Promise<null> {
  return apiPut<null, UpdatePasswordPayload>('/user/password', data)
}

export function getMyAssets(params?: {
  page?: number
  page_size?: number
}): Promise<PaginatedData<AssetItem> | AssetItem[]> {
  return apiGet<PaginatedData<AssetItem> | AssetItem[]>('/user/assets', { params })
}

export function getApiKeys(): Promise<PaginatedData<UserApiKey> | UserApiKey[]> {
  return apiGet<PaginatedData<UserApiKey> | UserApiKey[]>('/user/api-keys')
}

export function createApiKey(data: CreateApiKeyPayload): Promise<CreateApiKeyResult> {
  return apiPost<CreateApiKeyResult, CreateApiKeyPayload>('/user/api-keys', data)
}

export function revokeApiKey(id: number): Promise<null> {
  return apiDelete<null>(`/user/api-keys/${id}`)
}
