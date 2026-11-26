<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  Calendar,
  Check,
  Clock,
  Collection,
  Delete,
  Folder,
  InfoFilled,
  Lock,
  Plus,
  Trophy,
  User
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { getAssets } from '@/api/asset'
import {
  addDomainEventCandidate,
  closeDomainEvent,
  createDomainEvent,
  deleteDomainEvent,
  getDomainEventDetail,
  listDomainEvents,
  unvoteDomainEventCandidate,
  voteDomainEventCandidate,
  type DomainEventCandidateView,
  type DomainEventDetail,
  type DomainEventStatus,
  type DomainEventSummary,
  type DomainEventType
} from '@/api/domainEvent'
import { getDocuments } from '@/api/kb'
import type { AssetItem, KbDocument } from '@/types/api'
import { formatDateTime, normalizeList } from '@/utils/format'

interface FolderTreeNode {
  id: number
  label: string
  folderPath: string
  isLeaf?: boolean
}

const route = useRoute()
const domainId = computed(() => Number(route.params.domainId))

// Event list state
const loading = ref(false)
const events = ref<DomainEventSummary[]>([])
const statusFilter = ref<'all' | DomainEventStatus>('all')

const statusTabs: Array<{ value: 'all' | DomainEventStatus; label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'active', label: '进行中' },
  { value: 'upcoming', label: '未开始' },
  { value: 'ended', label: '已结束' }
]

const filteredEvents = computed(() => {
  if (statusFilter.value === 'all') return events.value
  return events.value.filter((item) => item.status === statusFilter.value)
})

// Event detail drawer state
const detailDrawerOpen = ref(false)
const detailLoading = ref(false)
const activeEvent = ref<DomainEventDetail | null>(null)
const votingCandidateId = ref<number | null>(null)
const managingAction = ref(false)

const sortedCandidates = computed<DomainEventCandidateView[]>(() => {
  if (!activeEvent.value?.candidates) return []
  return [...activeEvent.value.candidates].sort((a, b) => {
    if (b.vote_count !== a.vote_count) return b.vote_count - a.vote_count
    return a.id - b.id
  })
})

// Create event dialog state
const createDialogOpen = ref(false)
const createFormRef = ref<FormInstance>()
const creating = ref(false)
const createForm = reactive<{
  event_type: DomainEventType
  title: string
  description: string
  timeRange: [Date, Date] | null
  allow_multi_vote: boolean
}>({
  event_type: 'folder_vote',
  title: '',
  description: '',
  timeRange: null,
  allow_multi_vote: false
})

const createRules: FormRules = {
  event_type: [{ required: true, message: '请选择事件类型', trigger: 'change' }],
  title: [
    { required: true, message: '请输入事件标题', trigger: 'blur' },
    { max: 200, message: '标题长度不能超过 200 个字符', trigger: 'blur' }
  ],
  timeRange: [
    {
      required: true,
      validator: (_rule, value: [Date, Date] | null, callback) => {
        if (!value || value.length !== 2 || !value[0] || !value[1]) {
          callback(new Error('请选择起止时间'))
          return
        }
        if (value[1].getTime() <= value[0].getTime()) {
          callback(new Error('结束时间必须晚于开始时间'))
          return
        }
        callback()
      },
      trigger: 'change'
    }
  ]
}

// Add folder candidate dialog state
const candidateDialogOpen = ref(false)
const loadingKbs = ref(false)
const kbList = ref<AssetItem[]>([])
const selectedKbId = ref<number | null>(null)
const selectedFolder = ref<FolderTreeNode | null>(null)
const addingCandidate = ref(false)
const folderTreeKey = ref(0)

const folderTreeProps = {
  label: 'label',
  isLeaf: 'isLeaf'
}

function getEventTypeLabel(type: string): string {
  if (type === 'folder_vote') return '投票表决'
  if (type === 'lottery') return '抽奖事件'
  if (type === 'matching') return '匹配事件'
  return type
}

function getEventStatusMeta(status: DomainEventStatus): {
  label: string
  tagType: 'success' | 'info' | 'warning'
} {
  if (status === 'active') return { label: '进行中', tagType: 'success' }
  if (status === 'upcoming') return { label: '未开始', tagType: 'info' }
  return { label: '已结束', tagType: 'warning' }
}

function getCandidatePercentage(voteCount: number): number {
  const total = activeEvent.value?.total_votes ?? 0
  if (total <= 0) return 0
  return Math.round((voteCount / total) * 100)
}

async function fetchEvents() {
  if (!domainId.value) return
  loading.value = true
  try {
    const res = await listDomainEvents(domainId.value)
    events.value = Array.isArray(res) ? res : []
  } catch {
    events.value = []
  } finally {
    loading.value = false
  }
}

async function openEventDetail(eventId: number) {
  if (!domainId.value) return
  detailDrawerOpen.value = true
  detailLoading.value = true
  try {
    activeEvent.value = await getDomainEventDetail(domainId.value, eventId)
  } catch {
    activeEvent.value = null
  } finally {
    detailLoading.value = false
  }
}

function openCreateDialog() {
  const now = new Date()
  const nextWeek = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000)
  createForm.event_type = 'folder_vote'
  createForm.title = ''
  createForm.description = ''
  createForm.timeRange = [now, nextWeek]
  createForm.allow_multi_vote = false
  createDialogOpen.value = true
}

async function handleCreateEvent() {
  if (!createFormRef.value || !domainId.value) return
  const valid = await createFormRef.value.validate().catch(() => false)
  if (!valid || !createForm.timeRange) return

  creating.value = true
  try {
    const created = await createDomainEvent(domainId.value, {
      event_type: createForm.event_type,
      title: createForm.title.trim(),
      description: createForm.description.trim(),
      allow_multi_vote: createForm.allow_multi_vote,
      start_time: createForm.timeRange[0].toISOString(),
      end_time: createForm.timeRange[1].toISOString()
    })
    ElMessage.success('事件发起成功')
    createDialogOpen.value = false
    await fetchEvents()
    if (created?.id) {
      await openEventDetail(created.id)
    }
  } catch {
    // Handled by request interceptor
  } finally {
    creating.value = false
  }
}

async function handleVoteToggle(candidate: DomainEventCandidateView) {
  if (!domainId.value || !activeEvent.value) return
  if (activeEvent.value.status !== 'active') return

  votingCandidateId.value = candidate.id
  try {
    const updated = candidate.has_voted
      ? await unvoteDomainEventCandidate(domainId.value, activeEvent.value.id, candidate.id)
      : await voteDomainEventCandidate(domainId.value, activeEvent.value.id, candidate.id)
    activeEvent.value = updated
    ElMessage.success(candidate.has_voted ? '已取消投票' : '投票支持成功')
    await fetchEvents()
  } catch {
    // Handled by request interceptor
  } finally {
    votingCandidateId.value = null
  }
}

async function handleCloseEvent() {
  if (!domainId.value || !activeEvent.value) return
  try {
    await ElMessageBox.confirm(
      `确认提前结束事件「${activeEvent.value.title}」吗？结束后将无法继续投票或添加候选项。`,
      '提前结束确认',
      {
        type: 'warning',
        confirmButtonText: '确认结束',
        cancelButtonText: '取消'
      }
    )
    managingAction.value = true
    await closeDomainEvent(domainId.value, activeEvent.value.id)
    ElMessage.success('事件已提前结束')
    await Promise.all([openEventDetail(activeEvent.value.id), fetchEvents()])
  } catch {
    // User cancelled or error handled
  } finally {
    managingAction.value = false
  }
}

async function handleDeleteEvent() {
  if (!domainId.value || !activeEvent.value) return
  try {
    await ElMessageBox.confirm(
      `确认永久删除事件「${activeEvent.value.title}」及其所有投票记录吗？此操作不可恢复。`,
      '删除事件确认',
      {
        type: 'error',
        confirmButtonText: '确认删除',
        cancelButtonText: '取消'
      }
    )
    managingAction.value = true
    await deleteDomainEvent(domainId.value, activeEvent.value.id)
    ElMessage.success('事件已删除')
    detailDrawerOpen.value = false
    activeEvent.value = null
    await fetchEvents()
  } catch {
    // User cancelled or error handled
  } finally {
    managingAction.value = false
  }
}

async function openAddCandidateDialog() {
  if (!domainId.value) return
  candidateDialogOpen.value = true
  selectedKbId.value = null
  selectedFolder.value = null
  folderTreeKey.value += 1
  loadingKbs.value = true
  try {
    const res = await getAssets({
      domain_id: domainId.value,
      asset_type: 'kb',
      page: 1,
      page_size: 100
    })
    kbList.value = normalizeList(res).items
    if (kbList.value.length > 0) {
      selectedKbId.value = kbList.value[0].id
      folderTreeKey.value += 1
    }
  } catch {
    kbList.value = []
  } finally {
    loadingKbs.value = false
  }
}

function handleKbChange() {
  selectedFolder.value = null
  folderTreeKey.value += 1
}

async function loadFolderTreeNode(
  node: { level: number; data?: FolderTreeNode },
  resolve: (data: FolderTreeNode[]) => void
) {
  if (!selectedKbId.value) {
    resolve([])
    return
  }
  const kbId = selectedKbId.value
  const parentPath = node.level === 0 ? '' : node.data?.folderPath || ''
  try {
    const res = await getDocuments(kbId, {
      lazy: true,
      folder_path: parentPath,
      limit: 200
    })
    const docs: KbDocument[] = Array.isArray(res) ? res : res.list ?? res.items ?? []
    const folderNodes: FolderTreeNode[] = docs
      .filter((doc) => doc.is_folder)
      .map((doc) => ({
        id: doc.id,
        label: doc.file_name,
        folderPath:
          doc.folder_path || (parentPath ? `${parentPath}/${doc.file_name}` : doc.file_name),
        isLeaf: false
      }))
    resolve(folderNodes)
  } catch {
    resolve([])
  }
}

function handleFolderNodeClick(data: FolderTreeNode) {
  selectedFolder.value = data
}

async function handleConfirmAddCandidate() {
  if (!domainId.value || !activeEvent.value || !selectedKbId.value || !selectedFolder.value) return
  addingCandidate.value = true
  try {
    const updated = await addDomainEventCandidate(domainId.value, activeEvent.value.id, {
      kb_asset_id: selectedKbId.value,
      folder_doc_id: selectedFolder.value.id
    })
    activeEvent.value = updated
    candidateDialogOpen.value = false
    ElMessage.success('已添加文件夹候选项并自动为您投出 1 票')
    await fetchEvents()
  } catch {
    // Handled by request interceptor
  } finally {
    addingCandidate.value = false
  }
}

watch(
  domainId,
  () => {
    detailDrawerOpen.value = false
    activeEvent.value = null
    fetchEvents()
  },
  { immediate: false }
)

onMounted(() => {
  fetchEvents()
})
</script>

<template>
  <div class="page-container">
    <!-- Header -->
    <div class="page-header">
      <div>
        <h1 class="page-title">域内事件库</h1>
        <p class="page-subtitle">
          围绕域内知识库与协作场景发起投票表决等互动事件，后续可扩展抽奖、匹配等更多事件类型
        </p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateDialog">
        发起事件
      </el-button>
    </div>

    <!-- Filter Bar -->
    <div class="surface-card filter-bar">
      <div class="status-tabs">
        <button
          v-for="tab in statusTabs"
          :key="tab.value"
          type="button"
          class="status-tab-btn"
          :class="{ active: statusFilter === tab.value }"
          @click="statusFilter = tab.value"
        >
          {{ tab.label }}
        </button>
      </div>
      <div class="filter-summary">
        共 <span class="tabular-nums">{{ filteredEvents.length }}</span> 个事件
      </div>
    </div>

    <!-- Event Grid -->
    <div v-if="loading" class="events-grid">
      <div v-for="i in 3" :key="i" class="surface-card event-card-skeleton">
        <el-skeleton :rows="4" animated />
      </div>
    </div>

    <div v-else-if="filteredEvents.length === 0" class="surface-card empty-state">
      <el-empty description="当前筛选条件下暂无域内事件">
        <el-button type="primary" :icon="Plus" @click="openCreateDialog">
          发起第一个事件
        </el-button>
      </el-empty>
    </div>

    <div v-else class="events-grid">
      <article
        v-for="ev in filteredEvents"
        :key="ev.id"
        class="surface-card event-card"
        @click="openEventDetail(ev.id)"
      >
        <div class="event-card-top">
          <div class="event-tags">
            <el-tag size="small" type="primary" effect="light">
              {{ getEventTypeLabel(ev.event_type) }}
            </el-tag>
            <el-tag size="small" :type="getEventStatusMeta(ev.status).tagType" effect="light">
              {{ getEventStatusMeta(ev.status).label }}
            </el-tag>
            <el-tag size="small" type="info" effect="plain">
              {{ ev.allow_multi_vote ? '多选支持' : '单选投票' }}
            </el-tag>
          </div>
        </div>

        <h3 class="event-title">{{ ev.title }}</h3>
        <p class="event-desc">
          {{ ev.description || '暂无事件说明' }}
        </p>

        <div class="event-meta-list">
          <div class="event-meta-row">
            <el-icon><User /></el-icon>
            <span>发起人：{{ ev.creator_name || `用户 #${ev.creator_id}` }}</span>
          </div>
          <div class="event-meta-row">
            <el-icon><Clock /></el-icon>
            <span class="tabular-nums">
              {{ formatDateTime(ev.start_time) }} ~ {{ formatDateTime(ev.end_time) }}
            </span>
          </div>
        </div>

        <div class="event-card-footer">
          <div class="event-stat-pill">
            <el-icon><Folder /></el-icon>
            <span class="tabular-nums">{{ ev.candidate_count }} 个候选项</span>
          </div>
          <div class="event-stat-pill highlight">
            <el-icon><Trophy /></el-icon>
            <span class="tabular-nums">{{ ev.total_votes }} 票</span>
          </div>
        </div>
      </article>
    </div>

    <!-- Event Detail Sub-window Dialog -->
    <el-dialog
      v-model="detailDrawerOpen"
      width="760px"
      top="6vh"
      destroy-on-close
      :title="activeEvent?.title || '事件详情'"
      class="event-detail-dialog"
    >
      <div v-if="detailLoading" class="drawer-loading">
        <el-skeleton :rows="6" animated />
      </div>

      <div v-else-if="activeEvent" class="detail-body">
        <div class="detail-meta-card">
          <div class="detail-badges">
            <el-tag size="small" type="primary" effect="light">
              {{ getEventTypeLabel(activeEvent.event_type) }}
            </el-tag>
            <el-tag
              size="small"
              :type="getEventStatusMeta(activeEvent.status).tagType"
              effect="light"
            >
              {{ getEventStatusMeta(activeEvent.status).label }}
            </el-tag>
            <el-tag size="small" type="info" effect="plain">
              {{ activeEvent.allow_multi_vote ? '多选支持' : '单选投票' }}
            </el-tag>
          </div>

          <p class="detail-description">
            {{ activeEvent.description || '暂无事件说明' }}
          </p>

          <div class="detail-info-grid">
            <div class="detail-info-item">
              <span class="detail-info-label">发起人</span>
              <span class="detail-info-val">{{
                activeEvent.creator_name || `用户 #${activeEvent.creator_id}`
              }}</span>
            </div>
            <div class="detail-info-item">
              <span class="detail-info-label">累计票数</span>
              <span class="detail-info-val tabular-nums">{{ activeEvent.total_votes }} 票</span>
            </div>
            <div class="detail-info-item full">
              <span class="detail-info-label">时间窗口</span>
              <span class="detail-info-val tabular-nums">
                {{ formatDateTime(activeEvent.start_time) }} ~
                {{ formatDateTime(activeEvent.end_time) }}
              </span>
            </div>
          </div>

          <div class="detail-actions-row">
            <el-button
              type="primary"
              :icon="Plus"
              :disabled="activeEvent.status !== 'active'"
              @click="openAddCandidateDialog"
            >
              添加文件夹候选项
            </el-button>

            <div v-if="activeEvent.can_manage" class="manager-actions">
              <el-button
                v-if="activeEvent.status !== 'ended'"
                type="warning"
                plain
                :icon="Lock"
                :loading="managingAction"
                @click="handleCloseEvent"
              >
                提前结束
              </el-button>
              <el-button
                type="danger"
                plain
                :icon="Delete"
                :loading="managingAction"
                @click="handleDeleteEvent"
              >
                删除事件
              </el-button>
            </div>
          </div>
        </div>

        <!-- Candidate List -->
        <div class="candidates-section">
          <div class="section-header">
            <h4 class="section-title">
              候选项排行
              <span class="section-count tabular-nums">({{ sortedCandidates.length }})</span>
            </h4>
            <span v-if="activeEvent.status !== 'active'" class="section-hint">
              {{
                activeEvent.status === 'upcoming'
                  ? '事件尚未开始，暂不可投票或添加候选项'
                  : '事件已结束，投票结果已锁定'
              }}
            </span>
          </div>

          <el-empty
            v-if="sortedCandidates.length === 0"
            description="暂无候选项，点击上方「添加文件夹候选项」率先提案"
            :image-size="80"
          />

          <div v-else class="candidate-list">
            <div
              v-for="(cand, index) in sortedCandidates"
              :key="cand.id"
              class="candidate-item"
              :class="{ voted: cand.has_voted }"
            >
              <div class="candidate-header">
                <div class="candidate-rank-title">
                  <span class="rank-badge" :class="{ top: index < 3 }"> #{{ index + 1 }} </span>
                  <div class="candidate-title-group">
                    <div class="candidate-folder-line">
                      <el-icon class="folder-icon"><Folder /></el-icon>
                      <span class="candidate-folder-path">{{
                        cand.folder_path || cand.folder_name
                      }}</span>
                    </div>
                    <div class="candidate-sub-line">
                      <el-icon><Collection /></el-icon>
                      <span>{{ cand.kb_name }}</span>
                      <span class="dot-sep">·</span>
                      <span>由 {{ cand.added_by_name || `用户 #${cand.added_by_user_id}` }} 提案</span>
                    </div>
                  </div>
                </div>

                <el-button
                  :type="cand.has_voted ? 'success' : 'primary'"
                  :plain="!cand.has_voted"
                  size="small"
                  :icon="cand.has_voted ? Check : undefined"
                  :disabled="activeEvent.status !== 'active'"
                  :loading="votingCandidateId === cand.id"
                  @click="handleVoteToggle(cand)"
                >
                  {{ cand.has_voted ? '已支持 (点击取消)' : '投票支持' }}
                </el-button>
              </div>

              <div class="candidate-progress-row">
                <el-progress
                  :percentage="getCandidatePercentage(cand.vote_count)"
                  :stroke-width="8"
                  :show-text="false"
                  :status="cand.has_voted ? 'success' : undefined"
                  class="candidate-progress-bar"
                />
                <span class="candidate-vote-stat tabular-nums">
                  {{ cand.vote_count }} 票 ({{ getCandidatePercentage(cand.vote_count) }}%)
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- Create Event Dialog -->
    <el-dialog v-model="createDialogOpen" title="发起域内事件" width="540px" destroy-on-close>
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-width="108px"
        label-position="right"
      >
        <el-form-item label="事件类型" prop="event_type">
          <el-select v-model="createForm.event_type" style="width: 100%">
            <el-option label="投票表决事件" value="folder_vote" />
            <el-option label="抽奖事件（暂不开发）" value="lottery" disabled />
            <el-option label="匹配事件（暂不开发）" value="matching" disabled />
          </el-select>
        </el-form-item>

        <el-form-item label="事件标题" prop="title">
          <el-input
            v-model="createForm.title"
            placeholder="例如：本周优秀知识库专栏评选"
            maxlength="200"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="事件说明" prop="description">
          <el-input
            v-model="createForm.description"
            type="textarea"
            :rows="3"
            placeholder="介绍本次事件背景、评审标准或参与规则（选填）"
            maxlength="2000"
            show-word-limit
          />
        </el-form-item>

        <el-form-item label="起止时间" prop="timeRange">
          <el-date-picker
            v-model="createForm.timeRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            style="width: 100%"
          />
        </el-form-item>

        <el-form-item label="允许多选支持" prop="allow_multi_vote">
          <div class="switch-row">
            <el-switch v-model="createForm.allow_multi_vote" />
            <span class="switch-hint">
              {{
                createForm.allow_multi_vote
                  ? '每位成员可同时为多个候选项投票'
                  : '每位成员仅限支持 1 个候选项'
              }}
            </span>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="createDialogOpen = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreateEvent">
          确认发起
        </el-button>
      </template>
    </el-dialog>

    <!-- Add Folder Candidate Dialog -->
    <el-dialog
      v-model="candidateDialogOpen"
      title="添加文件夹候选项"
      width="560px"
      destroy-on-close
    >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        class="candidate-tip-alert"
        title="添加文件夹后将自动为您投出 1 票"
      />

      <div class="candidate-picker-form">
        <div class="picker-field">
          <label class="picker-label">1. 选择域内知识库</label>
          <el-select
            v-model="selectedKbId"
            placeholder="请选择知识库"
            :loading="loadingKbs"
            style="width: 100%"
            @change="handleKbChange"
          >
            <el-option
              v-for="kb in kbList"
              :key="kb.id"
              :label="kb.name"
              :value="kb.id"
            />
          </el-select>
        </div>

        <div class="picker-field">
          <label class="picker-label">2. 选择目标文件夹</label>
          <div class="folder-tree-box">
            <el-empty
              v-if="!selectedKbId"
              description="请先选择一个知识库"
              :image-size="60"
            />
            <el-tree
              v-else
              :key="folderTreeKey"
              lazy
              :load="loadFolderTreeNode"
              :props="folderTreeProps"
              node-key="id"
              highlight-current
              :expand-on-click-node="false"
              empty-text="该知识库下暂无可选文件夹"
              @node-click="handleFolderNodeClick"
            >
              <template #default="{ data }">
                <span class="tree-folder-node">
                  <el-icon><Folder /></el-icon>
                  <span>{{ data.label }}</span>
                </span>
              </template>
            </el-tree>
          </div>
        </div>

        <div v-if="selectedFolder" class="selected-folder-preview">
          <el-icon><InfoFilled /></el-icon>
          <span>已选中文件夹：</span>
          <strong class="mono-text">{{ selectedFolder.folderPath }}</strong>
        </div>
      </div>

      <template #footer>
        <el-button @click="candidateDialogOpen = false">取消</el-button>
        <el-button
          type="primary"
          :disabled="!selectedKbId || !selectedFolder"
          :loading="addingCandidate"
          @click="handleConfirmAddCandidate"
        >
          确认添加并投票
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.filter-bar {
  padding: 12px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.status-tabs {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.status-tab-btn {
  appearance: none;
  border: 1px solid var(--ascp-border);
  background: var(--ascp-bg-canvas);
  color: var(--ascp-text-secondary);
  font-size: 13px;
  font-weight: 500;
  padding: 6px 14px;
  border-radius: 999px;
  cursor: pointer;
  transition: all 0.16s ease;
}

.status-tab-btn:hover {
  border-color: var(--ascp-primary);
  color: var(--ascp-primary);
}

.status-tab-btn.active {
  background: var(--ascp-primary);
  border-color: var(--ascp-primary);
  color: #ffffff;
}

.filter-summary {
  font-size: 13px;
  color: var(--ascp-text-muted);
}

.events-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.event-card-skeleton,
.empty-state {
  padding: 24px;
}

.event-card {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  cursor: pointer;
  transition: border-color 0.18s ease, box-shadow 0.18s ease, transform 0.18s ease;
}

.event-card:hover {
  border-color: var(--ascp-primary);
  box-shadow: 0 8px 24px rgba(79, 70, 229, 0.08);
  transform: translateY(-2px);
}

.event-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.event-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.event-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  line-height: 1.4;
}

.event-desc {
  margin: 0;
  font-size: 13.5px;
  color: var(--ascp-text-secondary);
  line-height: 1.55;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 42px;
}

.event-meta-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding-top: 4px;
}

.event-meta-row {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.event-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--ascp-border);
}

.event-stat-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12.5px;
  color: var(--ascp-text-secondary);
  background: var(--ascp-bg-canvas);
  padding: 4px 10px;
  border-radius: 999px;
}

.event-stat-pill.highlight {
  color: var(--ascp-primary);
  background: var(--ascp-primary-soft);
  font-weight: 600;
}

.drawer-loading {
  padding: 16px 0;
}

.detail-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-height: 75vh;
  overflow-y: auto;
  padding-right: 4px;
}

.detail-meta-card {
  background: var(--ascp-bg-canvas);
  border: 1px solid var(--ascp-border);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.detail-badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.detail-description {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--ascp-text-secondary);
  white-space: pre-wrap;
}

.detail-info-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 16px;
  padding-top: 4px;
}

.detail-info-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.detail-info-item.full {
  grid-column: 1 / -1;
}

.detail-info-label {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.detail-info-val {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--ascp-text-primary);
}

.detail-actions-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 8px;
  border-top: 1px solid var(--ascp-border);
}

.manager-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.candidates-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.section-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.section-count {
  color: var(--ascp-text-muted);
  font-weight: 500;
}

.section-hint {
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.candidate-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.candidate-item {
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 14px 16px;
  background: var(--ascp-bg-surface);
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: border-color 0.16s ease;
}

.candidate-item.voted {
  border-color: #10b981;
  background: #f0fdf4;
}

.candidate-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.candidate-rank-title {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.rank-badge {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 24px;
  padding: 0 6px;
  border-radius: 6px;
  background: var(--ascp-bg-canvas);
  color: var(--ascp-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.rank-badge.top {
  background: var(--ascp-primary-soft);
  color: var(--ascp-primary);
}

.candidate-title-group {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.candidate-folder-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  word-break: break-all;
}

.folder-icon {
  color: var(--ascp-primary);
  flex-shrink: 0;
}

.candidate-sub-line {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
  flex-wrap: wrap;
}

.dot-sep {
  margin: 0 2px;
}

.candidate-progress-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.candidate-progress-bar {
  flex: 1;
}

.candidate-vote-stat {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--ascp-text-secondary);
  min-width: 92px;
  text-align: right;
}

.switch-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.switch-hint {
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.candidate-tip-alert {
  margin-bottom: 16px;
}

.candidate-picker-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.picker-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.picker-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--ascp-text-primary);
}

.folder-tree-box {
  border: 1px solid var(--ascp-border);
  border-radius: 8px;
  padding: 8px;
  min-height: 180px;
  max-height: 280px;
  overflow-y: auto;
  background: var(--ascp-bg-canvas);
}

.tree-folder-node {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13.5px;
}

.selected-folder-preview {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ascp-primary);
  background: var(--ascp-primary-soft);
  padding: 8px 12px;
  border-radius: 8px;
  word-break: break-all;
}
</style>
