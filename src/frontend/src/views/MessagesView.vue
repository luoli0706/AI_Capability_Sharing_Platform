<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import { useMediaQuery } from '@vueuse/core'
import { ArrowLeft, ChatDotRound, Plus, Promotion, Search, Share, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { checkAssetAccess, getAssets } from '@/api/asset'
import {
  createOrGetConversation,
  getMessageContacts,
  listConversations,
  listMessages,
  markConversationRead,
  sendMessage
} from '@/api/messages'
import { searchUsers } from '@/api/user'
import { useAuthStore } from '@/stores/auth'
import type {
  AssetCardPayload,
  AssetItem,
  AssetType,
  ConversationItem,
  MessageContactItem,
  UserBrief,
  UserMessageItem
} from '@/types/api'
import { formatDateTime, getAssetTypeMeta, normalizeList } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const isMobile = useMediaQuery('(max-width: 768px)')
const mobileShowChat = ref(false)

const loadingConversations = ref(false)
const conversations = ref<ConversationItem[]>([])
const activeConversationId = ref<number | null>(null)
const activeConversation = computed(
  () => conversations.value.find((c) => c.id === activeConversationId.value) ?? null
)

const loadingMessages = ref(false)
const loadingMore = ref(false)
const hasMoreMessages = ref(false)
const messages = ref<UserMessageItem[]>([])
const messageInput = ref('')
const sending = ref(false)
const messageListRef = ref<HTMLDivElement | null>(null)

// New conversation search modal
const newChatOpen = ref(false)
const userKeyword = ref('')
const searchingUsers = ref(false)
const searchedUsers = ref<UserBrief[]>([])
const creatingConv = ref(false)
const loadingContacts = ref(false)
const contacts = ref<MessageContactItem[]>([])

// Asset card picker modal
const assetPickerOpen = ref(false)
const assetKeyword = ref('')
const loadingAssets = ref(false)
const pickerAssets = ref<AssetItem[]>([])
const assetNote = ref('')
const sendingCard = ref(false)
const checkingCardMsgId = ref<number | null>(null)

let pollTimer: ReturnType<typeof setInterval> | null = null

function getUserDisplay(u?: UserBrief | null): string {
  if (!u) return '用户'
  return u.nickname || u.username || `用户 #${u.id}`
}

function getUserInitial(u?: UserBrief | null): string {
  return getUserDisplay(u).slice(0, 1).toUpperCase()
}

function formatShortTime(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const now = new Date()
  const sameDay =
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  if (sameDay) {
    return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

function scrollToBottom() {
  nextTick(() => {
    const el = messageListRef.value
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  })
}

function notifySidebarUnreadChanged() {
  window.dispatchEvent(new CustomEvent('ascp:messages-updated'))
}

async function fetchConversations(silent = false) {
  if (!silent) loadingConversations.value = true
  try {
    const res = await listConversations()
    conversations.value = Array.isArray(res) ? res : (res.list ?? [])
  } catch {
    if (!silent) conversations.value = []
  } finally {
    if (!silent) loadingConversations.value = false
  }
}

async function fetchMessagesForActive(silent = false) {
  if (!activeConversationId.value) return
  const convId = activeConversationId.value
  if (!silent) loadingMessages.value = true
  try {
    const res = await listMessages(convId, { limit: 50 })
    const rawItems = Array.isArray(res) ? res : (res.list ?? [])
    // Ensure chronological ascending order for chat display
    const sorted = [...rawItems].sort((a, b) => a.id - b.id)
    messages.value = sorted
    if (!Array.isArray(res) && typeof res.has_more === 'boolean') {
      hasMoreMessages.value = res.has_more
    } else {
      hasMoreMessages.value = sorted.length >= 50
    }
    if (!silent) {
      scrollToBottom()
    }
  } catch {
    if (!silent) messages.value = []
  } finally {
    if (!silent) loadingMessages.value = false
  }
}

async function loadOlderMessages() {
  if (!activeConversationId.value || messages.value.length === 0 || loadingMore.value) return
  const oldestId = messages.value[0].id
  loadingMore.value = true
  try {
    const el = messageListRef.value
    const prevHeight = el?.scrollHeight ?? 0
    const res = await listMessages(activeConversationId.value, {
      before_id: oldestId,
      limit: 50
    })
    const rawOlder = Array.isArray(res) ? res : (res.list ?? [])
    const older = [...rawOlder].sort((a, b) => a.id - b.id)
    if (older.length === 0) {
      hasMoreMessages.value = false
      return
    }
    const existingIds = new Set(messages.value.map((m) => m.id))
    const merged = [...older.filter((m) => !existingIds.has(m.id)), ...messages.value]
    messages.value = merged
    if (!Array.isArray(res) && typeof res.has_more === 'boolean') {
      hasMoreMessages.value = res.has_more
    } else {
      hasMoreMessages.value = older.length >= 50
    }
    nextTick(() => {
      if (el) {
        el.scrollTop = el.scrollHeight - prevHeight
      }
    })
  } finally {
    loadingMore.value = false
  }
}

async function selectConversation(conv: ConversationItem) {
  activeConversationId.value = conv.id
  if (isMobile.value) {
    mobileShowChat.value = true
  }
  await fetchMessagesForActive()
  if (conv.unread_count > 0) {
    try {
      await markConversationRead(conv.id)
      conv.unread_count = 0
      notifySidebarUnreadChanged()
    } catch {
      // Ignore
    }
  }
}

async function handleSend() {
  if (!activeConversationId.value || sending.value) return
  const content = messageInput.value.trim()
  if (!content) return

  sending.value = true
  try {
    const msg = await sendMessage(activeConversationId.value, content)
    messageInput.value = ''
    messages.value = [...messages.value, msg]
    const conv = conversations.value.find((c) => c.id === activeConversationId.value)
    if (conv) {
      conv.last_message_preview = content
      conv.last_message_at = msg.created_at
      conv.updated_at = msg.created_at
    }
    scrollToBottom()
  } catch {
    // Handled by interceptor
  } finally {
    sending.value = false
  }
}

function handleInputKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    void handleSend()
  }
}

function openNewChatModal() {
  userKeyword.value = ''
  searchedUsers.value = []
  newChatOpen.value = true
  void fetchContacts()
}

async function fetchContacts() {
  loadingContacts.value = true
  try {
    const res = await getMessageContacts()
    contacts.value = Array.isArray(res) ? res : (res.list ?? [])
  } catch {
    contacts.value = []
  } finally {
    loadingContacts.value = false
  }
}

async function fetchPickerAssets() {
  loadingAssets.value = true
  try {
    const res = await getAssets({
      keyword: assetKeyword.value.trim() || undefined,
      page: 1,
      page_size: 20
    })
    pickerAssets.value = normalizeList(res).items
  } catch {
    pickerAssets.value = []
  } finally {
    loadingAssets.value = false
  }
}

function openAssetPicker() {
  assetKeyword.value = ''
  assetNote.value = ''
  assetPickerOpen.value = true
  void fetchPickerAssets()
}

async function handleSendAssetCard(asset: AssetItem) {
  if (!activeConversationId.value || sendingCard.value) return
  sendingCard.value = true
  try {
    const cardPayload: AssetCardPayload = {
      asset_id: asset.id,
      asset_name: asset.name,
      asset_type: asset.asset_type,
      domain_id: asset.domain_id,
      domain_name: asset.domain_name,
      note: assetNote.value.trim() || undefined
    }
    const msg = await sendMessage(activeConversationId.value, {
      msg_type: 'asset_card',
      content: `[资产卡片] ${asset.name}`,
      payload: cardPayload
    })
    messages.value = [...messages.value, msg]
    const conv = conversations.value.find((c) => c.id === activeConversationId.value)
    if (conv) {
      conv.last_message_preview = `[资产卡片] ${asset.name}`
      conv.last_message_at = msg.created_at
      conv.updated_at = msg.created_at
    }
    assetPickerOpen.value = false
    scrollToBottom()
  } catch {
    // Handled by interceptor
  } finally {
    sendingCard.value = false
  }
}

function parseAssetCardPayload(msg: UserMessageItem): AssetCardPayload | null {
  if (msg.msg_type !== 'asset_card' || !msg.payload) return null
  let raw: unknown = msg.payload
  if (typeof raw === 'string') {
    try {
      raw = JSON.parse(raw)
    } catch {
      return null
    }
  }
  if (typeof raw === 'object' && raw !== null && 'asset_id' in raw) {
    return raw as AssetCardPayload
  }
  return null
}

async function handleClickAssetCard(msg: UserMessageItem) {
  const card = parseAssetCardPayload(msg)
  if (!card || checkingCardMsgId.value === msg.id) return
  checkingCardMsgId.value = msg.id
  try {
    const res = await checkAssetAccess(card.asset_id, {
      doc_id: card.doc_id,
      ref_domain_id: card.ref_domain_id
    })
    if (!res.accessible) {
      ElMessage.warning(res.reason || '您暂无该资产的访问权限，无法进入')
      return
    }
    if (card.asset_type === 'kb') {
      router.push({
        path: `/domains/${card.domain_id}/knowledge-bases`,
        query: {
          kbId: String(card.asset_id),
          ...(card.doc_id ? { docId: String(card.doc_id) } : {})
        }
      })
    } else {
      router.push(`/assets/${card.asset_id}`)
    }
  } catch {
    // Handled by interceptor
  } finally {
    checkingCardMsgId.value = null
  }
}

async function handleSearchUsers() {
  const kw = userKeyword.value.trim()
  if (!kw) {
    searchedUsers.value = []
    return
  }
  searchingUsers.value = true
  try {
    const res = await searchUsers(kw)
    const items = Array.isArray(res) ? res : (res.list ?? [])
    searchedUsers.value = items.filter((u) => u.id !== user.value?.id)
  } catch {
    searchedUsers.value = []
  } finally {
    searchingUsers.value = false
  }
}

async function startChatWithUser(target: UserBrief) {
  creatingConv.value = true
  try {
    const conv = await createOrGetConversation(target.id)
    newChatOpen.value = false
    const idx = conversations.value.findIndex((c) => c.id === conv.id)
    if (idx === -1) {
      conversations.value = [conv, ...conversations.value]
    }
    await selectConversation(conv)
  } catch {
    ElMessage.error('发起会话失败')
  } finally {
    creatingConv.value = false
  }
}

async function applyQueryConversation() {
  const rawId = Number(route.query.conversationId)
  if (!Number.isFinite(rawId) || rawId <= 0) return
  const match = conversations.value.find((c) => c.id === rawId)
  if (match) {
    await selectConversation(match)
  }
}

watch(
  () => route.query.conversationId,
  () => {
    void applyQueryConversation()
  }
)

onMounted(async () => {
  await fetchConversations()
  await applyQueryConversation()
  if (!activeConversationId.value && conversations.value.length > 0 && !isMobile.value) {
    await selectConversation(conversations.value[0])
  }
  pollTimer = setInterval(async () => {
    await fetchConversations(true)
    if (activeConversationId.value) {
      const prevLen = messages.value.length
      await fetchMessagesForActive(true)
      if (messages.value.length > prevLen) {
        scrollToBottom()
      }
      const active = conversations.value.find((c) => c.id === activeConversationId.value)
      if (active && active.unread_count > 0) {
        try {
          await markConversationRead(active.id)
          active.unread_count = 0
          notifySidebarUnreadChanged()
        } catch {
          // Ignore
        }
      }
    }
  }, 8000)
})

onBeforeUnmount(() => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
})
</script>

<template>
  <div class="page-container messages-page">
    <div class="surface-card chat-shell">
      <!-- Left: Conversation List -->
      <aside v-show="!isMobile || !mobileShowChat" class="conv-sidebar">
        <div class="conv-header">
          <div class="conv-title-wrap">
            <el-icon :size="18" color="#4F46E5"><ChatDotRound /></el-icon>
            <h2 class="conv-title">消息中心</h2>
          </div>
          <el-button type="primary" size="small" :icon="Plus" @click="openNewChatModal">
            发起私信
          </el-button>
        </div>

        <div v-loading="loadingConversations" class="conv-list">
          <el-empty
            v-if="!loadingConversations && conversations.length === 0"
            description="暂无私信会话"
            :image-size="64"
          >
            <el-button type="primary" size="small" @click="openNewChatModal">
              搜索用户发起私信
            </el-button>
          </el-empty>

          <button
            v-for="conv in conversations"
            :key="conv.id"
            type="button"
            class="conv-item"
            :class="{ 'is-active': conv.id === activeConversationId }"
            @click="selectConversation(conv)"
          >
            <el-avatar :size="40" :src="conv.peer?.avatar || undefined" class="conv-avatar">
              {{ getUserInitial(conv.peer) }}
            </el-avatar>
            <div class="conv-body">
              <div class="conv-top-row">
                <span class="conv-peer-name">{{ getUserDisplay(conv.peer) }}</span>
                <span class="conv-time tabular-nums">
                  {{ formatShortTime(conv.last_message_at || conv.updated_at) }}
                </span>
              </div>
              <div class="conv-bottom-row">
                <span class="conv-preview">{{ conv.last_message_preview || '暂无消息' }}</span>
                <span v-if="conv.unread_count > 0" class="conv-unread tabular-nums">
                  {{ conv.unread_count > 99 ? '99+' : conv.unread_count }}
                </span>
              </div>
            </div>
          </button>
        </div>
      </aside>

      <!-- Right: Active Chat Panel -->
      <section v-show="!isMobile || mobileShowChat" class="chat-main">
        <template v-if="activeConversation">
          <header class="chat-header">
            <div class="chat-peer-info">
              <el-button
                v-if="isMobile"
                link
                :icon="ArrowLeft"
                class="mobile-back-btn"
                @click="mobileShowChat = false"
              >
                列表
              </el-button>
              <button
                type="button"
                class="peer-profile-link"
                @click="router.push(`/users/${activeConversation.peer.id}`)"
              >
                <el-avatar :size="36" :src="activeConversation.peer?.avatar || undefined">
                  {{ getUserInitial(activeConversation.peer) }}
                </el-avatar>
                <div class="peer-meta">
                  <span class="peer-name">{{ getUserDisplay(activeConversation.peer) }}</span>
                  <span class="peer-username mono-text">@{{ activeConversation.peer?.username }}</span>
                </div>
              </button>
            </div>
            <el-button
              size="small"
              plain
              :icon="User"
              @click="router.push(`/users/${activeConversation.peer.id}`)"
            >
              TA 的主页
            </el-button>
          </header>

          <div ref="messageListRef" v-loading="loadingMessages" class="chat-messages">
            <div v-if="hasMoreMessages" class="load-more-wrap">
              <el-button size="small" text :loading="loadingMore" @click="loadOlderMessages">
                加载更早消息
              </el-button>
            </div>

            <div v-if="messages.length === 0 && !loadingMessages" class="chat-empty-hint">
              暂无消息记录，发送第一条消息打个招呼吧
            </div>

            <div
              v-for="msg in messages"
              :key="msg.id"
              class="msg-row"
              :class="{ 'is-mine': msg.sender_id === user?.id }"
            >
              <div class="msg-bubble-wrap">
                <div
                  v-if="parseAssetCardPayload(msg)"
                  class="asset-card-msg"
                  :class="{ 'is-checking': checkingCardMsgId === msg.id }"
                  @click="handleClickAssetCard(msg)"
                >
                  <div class="asset-card-msg-top">
                    <el-tag
                      :type="getAssetTypeMeta(parseAssetCardPayload(msg)!.asset_type as AssetType).tagType"
                      size="small"
                      effect="light"
                    >
                      {{ getAssetTypeMeta(parseAssetCardPayload(msg)!.asset_type as AssetType).shortLabel }}
                    </el-tag>
                    <span v-if="parseAssetCardPayload(msg)!.domain_name" class="asset-card-domain">
                      {{ parseAssetCardPayload(msg)!.domain_name }}
                    </span>
                  </div>
                  <div class="asset-card-msg-title">
                    {{ parseAssetCardPayload(msg)!.asset_name }}
                  </div>
                  <div v-if="parseAssetCardPayload(msg)!.doc_name" class="asset-card-msg-doc">
                    <el-tag
                      size="small"
                      :type="parseAssetCardPayload(msg)!.is_folder ? 'warning' : 'success'"
                      effect="plain"
                    >
                      {{ parseAssetCardPayload(msg)!.is_folder ? '文件夹' : '文档' }}
                    </el-tag>
                    <span class="mono-text">{{ parseAssetCardPayload(msg)!.doc_name }}</span>
                  </div>
                  <div v-if="parseAssetCardPayload(msg)!.note" class="asset-card-msg-note">
                    “{{ parseAssetCardPayload(msg)!.note }}”
                  </div>
                  <div class="asset-card-msg-footer">
                    <span>点击校验权限并查看详情 →</span>
                  </div>
                </div>
                <div v-else class="msg-bubble">{{ msg.content }}</div>
                <div class="msg-time tabular-nums">{{ formatDateTime(msg.created_at) }}</div>
              </div>
            </div>
          </div>

          <footer class="chat-composer">
            <el-input
              v-model="messageInput"
              type="textarea"
              :rows="3"
              resize="none"
              placeholder="输入私信内容，Enter 发送，Shift + Enter 换行..."
              @keydown="handleInputKeydown"
            />
            <div class="composer-actions">
              <div class="composer-left-tools">
                <el-button size="small" plain :icon="Share" @click="openAssetPicker">
                  分享资产卡片
                </el-button>
                <span class="composer-tip">按 Enter 快速发送</span>
              </div>
              <el-button
                type="primary"
                :icon="Promotion"
                :loading="sending"
                :disabled="!messageInput.trim()"
                @click="handleSend"
              >
                发送
              </el-button>
            </div>
          </footer>
        </template>

        <div v-else class="chat-placeholder">
          <el-empty description="请选择左侧会话，或点击「发起私信」开始交流" :image-size="80" />
        </div>
      </section>
    </div>

    <!-- Search User & Contacts Dialog -->
    <el-dialog v-model="newChatOpen" title="发起新对话" width="500px" destroy-on-close>
      <div class="contacts-section">
        <div class="dialog-section-title">同域 / 同组织好友</div>
        <div v-loading="loadingContacts" class="user-search-results contacts-list">
          <div
            v-for="peer in contacts"
            :key="peer.id"
            class="user-result-row"
          >
            <div class="user-result-left" @click="router.push(`/users/${peer.id}`)">
              <el-avatar :size="36" :src="peer.avatar || undefined">
                {{ getUserInitial(peer) }}
              </el-avatar>
              <div class="user-result-meta">
                <div class="user-result-name-row">
                  <span class="user-result-name">{{ getUserDisplay(peer) }}</span>
                  <span class="user-result-uname mono-text">@{{ peer.username }}</span>
                </div>
                <div v-if="peer.sources?.length" class="contact-sources">
                  <el-tag
                    v-for="src in peer.sources"
                    :key="src"
                    size="small"
                    type="info"
                    effect="plain"
                  >
                    {{ src }}
                  </el-tag>
                </div>
              </div>
            </div>
            <el-button
              type="primary"
              size="small"
              plain
              :loading="creatingConv"
              @click="startChatWithUser(peer)"
            >
              发私信
            </el-button>
          </div>
          <el-empty
            v-if="!loadingContacts && contacts.length === 0"
            description="暂无同域或同组织好友"
            :image-size="44"
          />
        </div>
      </div>

      <el-divider style="margin: 14px 0" />

      <div class="dialog-section-title" style="margin-bottom: 8px">全平台用户搜索</div>
      <div class="user-search-bar">
        <el-input
          v-model="userKeyword"
          placeholder="输入用户名或昵称搜索..."
          clearable
          :prefix-icon="Search"
          @keyup.enter="handleSearchUsers"
        />
        <el-button type="primary" :loading="searchingUsers" @click="handleSearchUsers">
          搜索
        </el-button>
      </div>

      <div v-loading="searchingUsers" class="user-search-results">
        <div
          v-for="item in searchedUsers"
          :key="item.id"
          class="user-result-row"
        >
          <div class="user-result-left" @click="router.push(`/users/${item.id}`)">
            <el-avatar :size="36" :src="item.avatar || undefined">
              {{ getUserInitial(item) }}
            </el-avatar>
            <div class="user-result-meta">
              <span class="user-result-name">{{ getUserDisplay(item) }}</span>
              <span class="user-result-uname mono-text">@{{ item.username }}</span>
            </div>
          </div>
          <el-button
            type="primary"
            size="small"
            plain
            :loading="creatingConv"
            @click="startChatWithUser(item)"
          >
            发私信
          </el-button>
        </div>
        <el-empty
          v-if="!searchingUsers && searchedUsers.length === 0"
          :description="userKeyword.trim() ? '未找到匹配的用户' : '输入关键词搜索平台用户'"
          :image-size="48"
        />
      </div>
    </el-dialog>

    <!-- Asset Picker Dialog -->
    <el-dialog v-model="assetPickerOpen" title="分享资产卡片" width="520px" destroy-on-close>
      <div class="user-search-bar">
        <el-input
          v-model="assetKeyword"
          placeholder="搜索可访问的 MCP / Skill / Agent / 知识库..."
          clearable
          :prefix-icon="Search"
          @keyup.enter="fetchPickerAssets"
        />
        <el-button type="primary" :loading="loadingAssets" @click="fetchPickerAssets">
          搜索
        </el-button>
      </div>

      <el-input
        v-model="assetNote"
        placeholder="附言（可选）：例如推荐查看这个能力..."
        clearable
        style="margin-bottom: 12px"
      />

      <div v-loading="loadingAssets" class="user-search-results">
        <div
          v-for="asset in pickerAssets"
          :key="asset.id"
          class="user-result-row"
        >
          <div class="picker-asset-meta">
            <div class="picker-asset-top">
              <el-tag :type="getAssetTypeMeta(asset.asset_type).tagType" size="small" effect="light">
                {{ getAssetTypeMeta(asset.asset_type).shortLabel }}
              </el-tag>
              <span class="user-result-name">{{ asset.name }}</span>
            </div>
            <div class="picker-asset-sub">
              <span>{{ asset.domain_name || `域 #${asset.domain_id}` }}</span>
              <span class="mono-text">{{ asset.asset_code }}</span>
            </div>
          </div>
          <el-button
            type="primary"
            size="small"
            :loading="sendingCard"
            @click="handleSendAssetCard(asset)"
          >
            发送卡片
          </el-button>
        </div>
        <el-empty
          v-if="!loadingAssets && pickerAssets.length === 0"
          description="未找到可分享的资产"
          :image-size="48"
        />
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.messages-page {
  height: calc(100vh - 108px);
  min-height: 540px;
}

.chat-shell {
  display: grid;
  grid-template-columns: 320px 1fr;
  height: 100%;
  overflow: hidden;
  padding: 0;
}

.conv-sidebar {
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--ascp-border, #e2e8f0);
  background: #f8fafc;
  min-height: 0;
}

.conv-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  border-bottom: 1px solid var(--ascp-border, #e2e8f0);
  background: #ffffff;
}

.conv-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.conv-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #0f172a;
}

.conv-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.conv-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background-color 0.15s ease;
}

.conv-item:hover {
  background: #eef2ff;
}

.conv-item.is-active {
  background: #e0e7ff;
}

.conv-avatar {
  flex-shrink: 0;
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  color: #ffffff;
  font-weight: 600;
}

.conv-body {
  flex: 1;
  min-width: 0;
}

.conv-top-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 4px;
}

.conv-peer-name {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-time {
  font-size: 11px;
  color: #64748b;
  flex-shrink: 0;
}

.conv-bottom-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.conv-preview {
  font-size: 12px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-unread {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: #ef4444;
  color: #ffffff;
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
  text-align: center;
  flex-shrink: 0;
}

.chat-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: #ffffff;
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--ascp-border, #e2e8f0);
}

.chat-peer-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.peer-profile-link {
  display: flex;
  align-items: center;
  gap: 10px;
  border: none;
  background: transparent;
  padding: 4px 8px 4px 0;
  border-radius: 8px;
  cursor: pointer;
  text-align: left;
}

.peer-profile-link:hover .peer-name {
  color: #4f46e5;
}

.peer-meta {
  display: flex;
  flex-direction: column;
}

.peer-name {
  font-size: 15px;
  font-weight: 600;
  color: #0f172a;
}

.peer-username {
  font-size: 12px;
  color: #64748b;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: #f8fafc;
}

.load-more-wrap {
  display: flex;
  justify-content: center;
}

.chat-empty-hint {
  margin: auto;
  font-size: 13px;
  color: #94a3b8;
}

.msg-row {
  display: flex;
  justify-content: flex-start;
}

.msg-row.is-mine {
  justify-content: flex-end;
}

.msg-bubble-wrap {
  max-width: 70%;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.msg-row.is-mine .msg-bubble-wrap {
  align-items: flex-end;
}

.msg-bubble {
  padding: 10px 14px;
  border-radius: 12px;
  background: #ffffff;
  color: #0f172a;
  font-size: 14px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-word;
  border: 1px solid #e2e8f0;
}

.msg-row.is-mine .msg-bubble {
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  color: #ffffff;
  border-color: transparent;
}

.msg-time {
  font-size: 11px;
  color: #94a3b8;
}

.chat-composer {
  padding: 14px 20px;
  border-top: 1px solid var(--ascp-border, #e2e8f0);
  background: #ffffff;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.composer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.composer-left-tools {
  display: flex;
  align-items: center;
  gap: 10px;
}

.asset-card-msg {
  width: 300px;
  max-width: 100%;
  padding: 12px 14px;
  border-radius: 12px;
  background: #ffffff;
  border: 1px solid #cbd5e1;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.06);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.15s ease;
}

.asset-card-msg:hover {
  border-color: #4f46e5;
  box-shadow: 0 4px 14px rgba(79, 70, 229, 0.14);
  transform: translateY(-1px);
}

.asset-card-msg.is-checking {
  opacity: 0.7;
  pointer-events: none;
}

.asset-card-msg-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.asset-card-domain {
  font-size: 11px;
  color: #64748b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-card-msg-title {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
  line-height: 1.4;
}

.asset-card-msg-doc {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #334155;
  padding: 4px 8px;
  border-radius: 6px;
  background: #f8fafc;
}

.asset-card-msg-note {
  font-size: 12px;
  color: #475569;
  font-style: italic;
  line-height: 1.45;
}

.asset-card-msg-footer {
  font-size: 11px;
  font-weight: 500;
  color: #4f46e5;
  border-top: 1px solid #f1f5f9;
  padding-top: 6px;
}

.dialog-section-title {
  font-size: 13px;
  font-weight: 600;
  color: #0f172a;
  margin-bottom: 8px;
}

.contacts-list {
  max-height: 200px;
}

.user-result-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.contact-sources {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 4px;
}

.picker-asset-meta {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-right: 10px;
}

.picker-asset-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.picker-asset-sub {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #64748b;
}

.composer-tip {
  font-size: 12px;
  color: #94a3b8;
}

.chat-placeholder {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.user-search-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}

.user-search-results {
  max-height: 320px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.user-result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 8px;
  border: 1px solid #e2e8f0;
}

.user-result-left {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.user-result-meta {
  display: flex;
  flex-direction: column;
}

.user-result-name {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
}

.user-result-uname {
  font-size: 12px;
  color: #64748b;
}

@media (max-width: 768px) {
  .messages-page {
    height: calc(100vh - 96px);
  }
  .chat-shell {
    grid-template-columns: 1fr;
  }
  .msg-bubble-wrap {
    max-width: 86%;
  }
}
</style>
