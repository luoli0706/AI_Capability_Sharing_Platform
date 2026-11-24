import { apiGet, apiPost } from './request'
import type {
  ConversationItem,
  MessageContactItem,
  SendMessagePayload,
  ShareAssetCardPayload,
  UserMessageItem
} from '@/types/api'

export function getUnreadMessageCount(): Promise<{ unread_count: number }> {
  return apiGet<{ unread_count: number }>('/messages/unread-count')
}

export function getMessageContacts(): Promise<{ list: MessageContactItem[] } | MessageContactItem[]> {
  return apiGet<{ list: MessageContactItem[] } | MessageContactItem[]>('/messages/contacts')
}

export function listConversations(): Promise<{ list: ConversationItem[] } | ConversationItem[]> {
  return apiGet<{ list: ConversationItem[] } | ConversationItem[]>('/messages/conversations')
}

export function createOrGetConversation(targetUserId: number): Promise<ConversationItem> {
  return apiPost<ConversationItem, { target_user_id: number }>('/messages/conversations', {
    target_user_id: targetUserId
  })
}

export function listMessages(
  conversationId: number,
  params?: { before_id?: number; limit?: number }
): Promise<{ list: UserMessageItem[]; has_more?: boolean } | UserMessageItem[]> {
  return apiGet<{ list: UserMessageItem[]; has_more?: boolean } | UserMessageItem[]>(
    `/messages/conversations/${conversationId}/messages`,
    { params }
  )
}

export function sendMessage(
  conversationId: number,
  payload: string | SendMessagePayload
): Promise<UserMessageItem> {
  const rawBody: SendMessagePayload = typeof payload === 'string' ? { content: payload } : payload
  const body = {
    ...rawBody,
    payload:
      rawBody.payload !== undefined && rawBody.payload !== null && typeof rawBody.payload !== 'string'
        ? JSON.stringify(rawBody.payload)
        : rawBody.payload
  }
  return apiPost<UserMessageItem, typeof body>(
    `/messages/conversations/${conversationId}/messages`,
    body
  )
}

export function shareAssetCard(
  payload: ShareAssetCardPayload
): Promise<{ sent_count?: number; results?: unknown[] } | null> {
  return apiPost<{ sent_count?: number; results?: unknown[] } | null, ShareAssetCardPayload>(
    '/messages/share-card',
    payload
  )
}

export function markConversationRead(conversationId: number): Promise<null> {
  return apiPost<null>(`/messages/conversations/${conversationId}/read`)
}
