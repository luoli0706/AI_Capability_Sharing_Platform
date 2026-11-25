import { apiGet, apiPost, apiPut } from './request'
import type {
  BindEmailPayload,
  PublicUserProfile,
  UserBrief,
  UserFavoriteItem,
  UserInfo
} from '@/types/api'

export function sendBindEmailCode(email: string): Promise<null> {
  return apiPost<null, { email: string }>('/user/email/send-code', { email })
}

export function bindEmail(payload: BindEmailPayload): Promise<UserInfo> {
  return apiPut<UserInfo, BindEmailPayload>('/user/email', payload)
}

export function searchUsers(
  keyword: string
): Promise<{ list: UserBrief[] } | UserBrief[]> {
  return apiGet<{ list: UserBrief[] } | UserBrief[]>('/users/search', {
    params: { keyword }
  })
}

export function getPublicUserProfile(userId: number): Promise<PublicUserProfile> {
  return apiGet<PublicUserProfile>(`/users/${userId}`)
}

export function getUserFavorites(
  userId: number
): Promise<{ is_public: boolean; list: UserFavoriteItem[] }> {
  return apiGet<{ is_public: boolean; list: UserFavoriteItem[] }>(
    `/users/${userId}/favorites`
  )
}

export function getMyFavorites(): Promise<{ list: UserFavoriteItem[] } | UserFavoriteItem[]> {
  return apiGet<{ list: UserFavoriteItem[] } | UserFavoriteItem[]>('/user/favorites')
}
