<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CopyDocument, Link, Promotion, Search, Share } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getMessageContacts, shareAssetCard } from '@/api/messages'
import type { AssetType, MessageContactItem } from '@/types/api'
import { getAssetTypeMeta } from '@/utils/format'

const props = defineProps<{
  modelValue: boolean
  assetId: number
  assetName?: string
  assetType?: AssetType | string
  domainId: number
  domainName?: string
  docId?: number
  docName?: string
  isFolder?: boolean
  refDomainId?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val: boolean) => emit('update:modelValue', val)
})

const loadingContacts = ref(false)
const contacts = ref<MessageContactItem[]>([])
const searchKeyword = ref('')
const selectedUserIds = ref<number[]>([])
const note = ref('')
const sending = ref(false)

const targetPath = computed(() => {
  if (props.assetType === 'kb' || props.docId) {
    const query = new URLSearchParams({ kbId: String(props.assetId) })
    if (props.docId) {
      query.set('docId', String(props.docId))
    }
    return `/domains/${props.domainId}/knowledge-bases?${query.toString()}`
  }
  return `/assets/${props.assetId}`
})

const fullShareUrl = computed(() => {
  if (typeof window === 'undefined') return targetPath.value
  return `${window.location.origin}${targetPath.value}`
})

const filteredContacts = computed(() => {
  const kw = searchKeyword.value.trim().toLowerCase()
  if (!kw) return contacts.value
  return contacts.value.filter((c) => {
    const nameMatch =
      (c.nickname || '').toLowerCase().includes(kw) ||
      (c.username || '').toLowerCase().includes(kw)
    const sourceMatch = (c.sources || []).some((s) => s.toLowerCase().includes(kw))
    return nameMatch || sourceMatch
  })
})

const allFilteredSelected = computed(() => {
  if (filteredContacts.value.length === 0) return false
  return filteredContacts.value.every((c) => selectedUserIds.value.includes(c.id))
})

const isIndeterminate = computed(() => {
  if (filteredContacts.value.length === 0) return false
  const count = filteredContacts.value.filter((c) => selectedUserIds.value.includes(c.id)).length
  return count > 0 && count < filteredContacts.value.length
})

const typeMeta = computed(() => {
  if (!props.assetType) return null
  return getAssetTypeMeta(props.assetType as AssetType)
})

function getUserDisplay(u: MessageContactItem): string {
  return u.nickname || u.username || `用户 #${u.id}`
}

function getUserInitial(u: MessageContactItem): string {
  return getUserDisplay(u).slice(0, 1).toUpperCase()
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

function handleToggleSelectAll(checked: boolean | string | number) {
  const filteredIds = filteredContacts.value.map((c) => c.id)
  if (checked) {
    const merged = new Set([...selectedUserIds.value, ...filteredIds])
    selectedUserIds.value = Array.from(merged)
  } else {
    const filteredSet = new Set(filteredIds)
    selectedUserIds.value = selectedUserIds.value.filter((id) => !filteredSet.has(id))
  }
}

async function handleCopyLink() {
  try {
    await navigator.clipboard.writeText(fullShareUrl.value)
    ElMessage.success('直达链接已复制到剪贴板')
  } catch {
    ElMessage.info(fullShareUrl.value)
  }
}

async function handleSendShareCard() {
  if (selectedUserIds.value.length === 0 || sending.value) return
  sending.value = true
  try {
    await shareAssetCard({
      target_user_ids: selectedUserIds.value,
      asset_id: props.assetId,
      doc_id: props.docId || undefined,
      ref_domain_id: props.refDomainId || undefined,
      note: note.value.trim() || undefined
    })
    ElMessage.success(`已向 ${selectedUserIds.value.length} 位成员发送分享卡片`)
    visible.value = false
  } catch {
    // Handled by interceptor
  } finally {
    sending.value = false
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      searchKeyword.value = ''
      selectedUserIds.value = []
      note.value = ''
      void fetchContacts()
    }
  }
)
</script>

<template>
  <el-dialog
    v-model="visible"
    title="分享资产"
    width="540px"
    destroy-on-close
    append-to-body
  >
    <div class="share-dialog-body">
      <!-- Target Summary -->
      <div class="share-target-summary">
        <div class="share-target-top">
          <el-icon :size="16" color="#4F46E5"><Share /></el-icon>
          <span class="share-target-name">{{ assetName || `资产 #${assetId}` }}</span>
          <el-tag v-if="typeMeta" :type="typeMeta.tagType" size="small" effect="light">
            {{ typeMeta.shortLabel }}
          </el-tag>
          <el-tag v-if="domainName" size="small" type="info" effect="plain">
            {{ domainName }}
          </el-tag>
        </div>
        <div v-if="docName" class="share-target-doc">
          <el-tag size="small" :type="isFolder ? 'warning' : 'success'" effect="plain">
            {{ isFolder ? '文件夹' : '文档' }}
          </el-tag>
          <span class="mono-text">{{ docName }}</span>
        </div>
      </div>

      <!-- Section 1: Copy Direct Link -->
      <div class="share-section">
        <div class="section-label">
          <el-icon :size="14"><Link /></el-icon>
          <span>复制直达链接</span>
        </div>
        <div class="link-copy-row">
          <el-input :model-value="fullShareUrl" readonly size="default" class="mono-text" />
          <el-button type="primary" plain :icon="CopyDocument" @click="handleCopyLink">
            复制链接
          </el-button>
        </div>
      </div>

      <!-- Section 2: Share Card to Contacts -->
      <div class="share-section">
        <div class="section-header-row">
          <div class="section-label">
            <el-icon :size="14"><Promotion /></el-icon>
            <span>分享资产卡片给同域/同组织成员（可多选）</span>
          </div>
          <span v-if="selectedUserIds.length > 0" class="selected-count">
            已选 {{ selectedUserIds.length }} 人
          </span>
        </div>

        <div class="contact-filter-bar">
          <el-input
            v-model="searchKeyword"
            placeholder="搜索成员昵称、用户名或所属域/组织..."
            clearable
            size="small"
            :prefix-icon="Search"
          />
          <el-checkbox
            :model-value="allFilteredSelected"
            :indeterminate="isIndeterminate"
            :disabled="filteredContacts.length === 0"
            @change="handleToggleSelectAll"
          >
            全选
          </el-checkbox>
        </div>

        <div v-loading="loadingContacts" class="contact-list">
          <el-empty
            v-if="!loadingContacts && filteredContacts.length === 0"
            :description="contacts.length === 0 ? '暂无同域或同组织的可分享成员' : '未找到匹配的成员'"
            :image-size="48"
          />

          <el-checkbox-group v-else v-model="selectedUserIds" class="contact-checkbox-group">
            <label
              v-for="contact in filteredContacts"
              :key="contact.id"
              class="contact-row"
              :class="{ 'is-selected': selectedUserIds.includes(contact.id) }"
            >
              <el-checkbox :value="contact.id" />
              <el-avatar :size="32" :src="contact.avatar || undefined">
                {{ getUserInitial(contact) }}
              </el-avatar>
              <div class="contact-meta">
                <div class="contact-name-row">
                  <span class="contact-name">{{ getUserDisplay(contact) }}</span>
                  <span class="contact-username mono-text">@{{ contact.username }}</span>
                </div>
                <div v-if="contact.sources?.length" class="contact-sources">
                  <el-tag
                    v-for="src in contact.sources"
                    :key="src"
                    size="small"
                    type="info"
                    effect="plain"
                  >
                    {{ src }}
                  </el-tag>
                </div>
              </div>
            </label>
          </el-checkbox-group>
        </div>

        <el-input
          v-model="note"
          type="textarea"
          :rows="2"
          maxlength="200"
          show-word-limit
          placeholder="附言（可选）：例如推荐查看这个文档/能力..."
          class="note-input"
        />
      </div>
    </div>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button
        type="primary"
        :icon="Promotion"
        :loading="sending"
        :disabled="selectedUserIds.length === 0"
        @click="handleSendShareCard"
      >
        发送分享卡片{{ selectedUserIds.length > 0 ? ` (${selectedUserIds.length})` : '' }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.share-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.share-target-summary {
  padding: 12px 14px;
  border-radius: 10px;
  background: #f8fafc;
  border: 1px solid var(--ascp-border, #e2e8f0);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.share-target-top {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.share-target-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-main, #0f172a);
}

.share-target-doc {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--ascp-text-sub, #475569);
}

.share-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.section-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--ascp-text-main, #0f172a);
}

.selected-count {
  font-size: 12px;
  font-weight: 600;
  color: var(--ascp-primary, #4f46e5);
}

.link-copy-row {
  display: flex;
  gap: 8px;
}

.contact-filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.contact-list {
  max-height: 220px;
  overflow-y: auto;
  border: 1px solid var(--ascp-border, #e2e8f0);
  border-radius: 8px;
  padding: 6px;
  background: #ffffff;
}

.contact-checkbox-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.contact-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.contact-row:hover {
  background: #f8fafc;
}

.contact-row.is-selected {
  background: #eef2ff;
}

.contact-meta {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.contact-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.contact-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--ascp-text-main, #0f172a);
}

.contact-username {
  font-size: 12px;
  color: var(--ascp-text-muted, #64748b);
}

.contact-sources {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.note-input {
  margin-top: 2px;
}
</style>
