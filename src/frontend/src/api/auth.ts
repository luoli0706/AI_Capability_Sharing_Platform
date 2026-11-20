import { apiGet, apiPost, apiPut } from './request'
import type {
  AssetItem,
  LoginPayload,
  LoginResult,
  PaginatedData,
  RegisterPayload,
  UpdatePasswordPayload,
  UpdateProfilePayload,
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
