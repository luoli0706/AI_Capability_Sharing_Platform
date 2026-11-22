<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  ArrowLeft,
  ChatDotRound,
  Collection,
  Document,
  Lock,
  Setting
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { createOrGetConversation } from '@/api/messages'
import { getPublicUserProfile, getUserFavorites } from '@/api/user'
import { useAuthStore } from '@/stores/auth'
import type { PublicUserProfile, UserFavoriteItem } from '@/types/api'
import { formatDateTime } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { user: currentUser } = storeToRefs(authStore)

const userId = computed(() => Number(route.params.id))
const isSelf = computed(() => Boolean(currentUser.value && currentUser.value.id === userId.value))

const loadingProfile = ref(false)
const profile = ref<PublicUserProfile | null>(null)
const startingChat = ref(false)

const loadingFavorites = ref(false)
const favoritesPublic = ref(false)
const favorites = ref<UserFavoriteItem[]>([])

const displayName = computed(
  () => profile.value?.nickname || profile.value?.username || `用户 #${userId.value}`
)
const userInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())

async function loadData() {
  if (!Number.isFinite(userId.value) || userId.value <= 0) {
    profile.value = null
    return
  }
  loadingProfile.value = true
  loadingFavorites.value = true
  try {
    profile.value = await getPublicUserProfile(userId.value)
  } catch {
    profile.value = null
  } finally {
    loadingProfile.value = false
  }

  try {
    const favRes = await getUserFavorites(userId.value)
    favoritesPublic.value = Boolean(favRes.is_public)
    favorites.value = Array.isArray(favRes.list) ? favRes.list : []
  } catch {
    favoritesPublic.value = false
    favorites.value = []
  } finally {
    loadingFavorites.value = false
  }
}

async function handleSendMessage() {
  if (!profile.value || isSelf.value) return
  startingChat.value = true
  try {
    const conv = await createOrGetConversation(profile.value.id)
    router.push({ path: '/messages', query: { conversationId: String(conv.id) } })
  } catch {
    ElMessage.error('无法发起私信会话')
  } finally {
    startingChat.value = false
  }
}

function goToFavoriteDoc(item: UserFavoriteItem) {
  router.push({
    path: `/domains/${item.domain_id}/knowledge-bases`,
    query: {
      kbId: String(item.kb_asset_id),
      docId: String(item.doc_id)
    }
  })
}

watch(userId, () => {
  void loadData()
})

onMounted(() => {
  void loadData()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div class="header-left-row">
        <el-button :icon="ArrowLeft" @click="router.back()">返回</el-button>
        <div>
          <h1 class="page-title">用户主页</h1>
          <p class="page-subtitle">查看成员公开资料与知识库收藏动态</p>
        </div>
      </div>
    </div>

    <div v-if="loadingProfile" class="surface-card p-24">
      <el-skeleton :rows="4" animated />
    </div>

    <div v-else-if="!profile" class="surface-card empty-box">
      <el-empty description="未找到该用户信息" />
    </div>

    <template v-else>
      <div class="surface-card user-banner">
        <div class="user-banner-left">
          <el-avatar :size="76" :src="profile.avatar || undefined" class="user-avatar">
            {{ userInitial }}
          </el-avatar>
          <div class="user-info">
            <div class="user-name-row">
              <h2 class="user-name">{{ displayName }}</h2>
              <el-tag v-if="isSelf" size="small" type="info" effect="light">我自己</el-tag>
            </div>
            <div class="user-sub">
              <span class="mono-text">@{{ profile.username }}</span>
              <span class="dot">·</span>
              <span>加入于 {{ formatDateTime(profile.created_at) }}</span>
            </div>
            <p class="user-bio">{{ profile.bio || '这位用户暂未填写个人简介' }}</p>
          </div>
        </div>

        <div class="user-banner-actions">
          <el-button
            v-if="!isSelf"
            type="primary"
            :icon="ChatDotRound"
            :loading="startingChat"
            @click="handleSendMessage"
          >
            发送私信
          </el-button>
          <el-button
            v-else
            :icon="Setting"
            @click="router.push('/profile')"
          >
            前往个人中心
          </el-button>
        </div>
      </div>

      <div class="surface-card favorites-card">
        <div class="section-header">
          <div class="section-title-wrap">
            <el-icon :size="18" color="#4F46E5"><Collection /></el-icon>
            <h3 class="section-title">TA 的收藏</h3>
          </div>
          <el-tag v-if="favoritesPublic" size="small" type="success" effect="plain">
            公开可见 · {{ favorites.length }} 项
          </el-tag>
          <el-tag v-else size="small" type="info" effect="plain">
            <el-icon style="vertical-align: -2px; margin-right: 4px"><Lock /></el-icon>
            仅自己可见
          </el-tag>
        </div>

        <div v-loading="loadingFavorites">
          <div v-if="!favoritesPublic" class="privacy-empty">
            <el-icon :size="36" color="#94A3B8"><Lock /></el-icon>
            <div class="privacy-title">该用户已设置收藏仅自己可见</div>
            <div class="privacy-sub">出于隐私保护，暂无法查看 TA 的知识库文档收藏列表</div>
          </div>

          <el-empty
            v-else-if="favorites.length === 0"
            description="TA 还没有收藏任何知识库文档"
            :image-size="64"
          />

          <div v-else class="fav-grid">
            <div
              v-for="item in favorites"
              :key="item.id"
              class="fav-item"
              @click="goToFavoriteDoc(item)"
            >
              <div class="fav-item-top">
                <el-icon :size="18" color="#4F46E5"><Document /></el-icon>
                <span class="fav-filename mono-text">{{ item.file_name }}</span>
              </div>
              <div class="fav-item-meta">
                <el-tag size="small" type="info" effect="plain">{{ item.domain_name }}</el-tag>
                <span class="fav-kb">{{ item.kb_name }}</span>
              </div>
              <div class="fav-item-footer">
                <span class="fav-path mono-text">/{{ item.folder_path || '' }}</span>
                <span class="fav-time tabular-nums">{{ formatDateTime(item.created_at) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.header-left-row {
  display: flex;
  align-items: center;
  gap: 14px;
}

.p-24 {
  padding: 24px;
}

.empty-box {
  padding: 48px 24px;
}

.user-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 24px;
  flex-wrap: wrap;
}

.user-banner-left {
  display: flex;
  align-items: center;
  gap: 18px;
}

.user-avatar {
  flex-shrink: 0;
  background: linear-gradient(135deg, #4f46e5, #6366f1);
  color: #ffffff;
  font-size: 26px;
  font-weight: 700;
}

.user-name-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.user-name {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
}

.user-sub {
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #64748b;
  flex-wrap: wrap;
}

.user-bio {
  margin: 8px 0 0;
  font-size: 14px;
  color: #334155;
}

.favorites-card {
  padding: 20px 24px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18px;
}

.section-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #0f172a;
}

.privacy-empty {
  padding: 44px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
  background: #f8fafc;
  border-radius: 10px;
  border: 1px dashed #cbd5e1;
}

.privacy-title {
  font-size: 15px;
  font-weight: 600;
  color: #334155;
}

.privacy-sub {
  font-size: 13px;
  color: #64748b;
}

.fav-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 14px;
}

.fav-item {
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid #e2e8f0;
  background: #ffffff;
  cursor: pointer;
  transition: all 0.15s ease;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fav-item:hover {
  border-color: #818cf8;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.08);
  transform: translateY(-1px);
}

.fav-item-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.fav-filename {
  font-size: 14px;
  font-weight: 600;
  color: #0f172a;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fav-item-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: #475569;
}

.fav-kb {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.fav-item-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: #94a3b8;
  gap: 8px;
}

.fav-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
