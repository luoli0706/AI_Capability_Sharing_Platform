import { apiGet, apiPost } from './request'
import type { ConversationItem, UserMessageItem } from '@/types/api'

export function getUnreadMessageCount(): Promise<{ unread_count: number }> {
  return apiGet<{ unread_count: number }>('/messages/unread-count')
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

export function sendMessage(conversationId: number, content: string): Promise<UserMessageItem> {
  return apiPost<UserMessageItem, { content: string }>(
    `/messages/conversations/${conversationId}/messages`,
    { content }
  )
}

export function markConversationRead(conversationId: number): Promise<null> {
  return apiPost<null>(`/messages/conversations/${conversationId}/read`)
}
