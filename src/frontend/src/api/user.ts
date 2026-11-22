import { apiGet } from './request'
import type { PublicUserProfile, UserBrief, UserFavoriteItem } from '@/types/api'

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
