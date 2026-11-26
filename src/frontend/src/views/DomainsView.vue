<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  CopyDocument,
  Delete,
  Edit,
  Lock,
  OfficeBuilding,
  Plus,
  Right,
  Search,
  Setting,
  Unlock,
  UserFilled
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  applyJoinDomain,
  createDomain,
  deleteDomain,
  getDomains,
  joinDomainByQuiz,
  updateDomain,
  updateDomainJoinSettings
} from '@/api/domain'
import { getOrgs } from '@/api/org'
import { useDomainStore } from '@/stores/domain'
import type { DomainItem, DomainJoinMode, DomainVisibility, OrgItem } from '@/types/api'
import { getDomainVisibilityMeta, getRoleLabel, normalizeList } from '@/utils/format'

const router = useRouter()
const domainStore = useDomainStore()

const activeTab = ref<'mine' | 'square'>('mine')
const loading = ref(false)
const domains = ref<DomainItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(12)
const searchText = ref('')
const myOrgs = ref<OrgItem[]>([])

// Create Modal
const createModalOpen = ref(false)
const createFormRef = ref<FormInstance>()
const creating = ref(false)
const createForm = reactive<{
  domain_name: string
  domain_code: string
  description: string
  org_id: number
  visibility: DomainVisibility
  join_mode: DomainJoinMode
  join_question: string
  join_answer: string
}>({
  domain_name: '',
  domain_code: '',
  description: '',
  org_id: 0,
  visibility: 'public',
  join_mode: 'approval_only',
  join_question: '',
  join_answer: ''
})

const canCreateOrgPrivate = computed(() => {
  if (!createForm.org_id) return false
  const org = myOrgs.value.find((o) => o.id === createForm.org_id)
  return org?.my_role === 'owner' || org?.my_role === 'admin'
})

const createRules: FormRules = {
  domain_name: [{ required: true, message: '请输入域名称', trigger: 'blur' }],
  domain_code: [
    { required: true, message: '请输入域编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ],
  join_question: [
    {
      validator: (_rule, value: string, callback) => {
        if (
          createForm.visibility === 'public' &&
          createForm.join_mode === 'approval_or_quiz' &&
          !String(value || '').trim()
        ) {
          callback(new Error('请填写准入问题'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  join_answer: [
    {
      validator: (_rule, value: string, callback) => {
        if (
          createForm.visibility === 'public' &&
          createForm.join_mode === 'approval_or_quiz' &&
          !String(value || '').trim()
        ) {
          callback(new Error('请填写预设答案'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

// Edit Modal
const editModalOpen = ref(false)
const editFormRef = ref<FormInstance>()
const editing = ref(false)
const editDomainId = ref<number | null>(null)
const editDomainOrgId = ref<number>(0)
const editForm = reactive<{
  domain_name: string
  description: string
  visibility: DomainVisibility
  allow_member_kb_write: boolean
}>({
  domain_name: '',
  description: '',
  visibility: 'public',
  allow_member_kb_write: false
})

const canEditOrgPrivate = computed(() => {
  if (!editDomainOrgId.value) return false
  if (editForm.visibility === 'org_private') return true
  const org = myOrgs.value.find((o) => o.id === editDomainOrgId.value)
  return org?.my_role === 'owner' || org?.my_role === 'admin'
})

const editRules: FormRules = {
  domain_name: [{ required: true, message: '请输入域名称', trigger: 'blur' }]
}

// Join Settings Modal
const joinSettingsModalOpen = ref(false)
const joinSettingsFormRef = ref<FormInstance>()
const savingJoinSettings = ref(false)
const joinSettingsDomainId = ref<number | null>(null)
const joinSettingsDomainName = ref('')
const joinSettingsForm = reactive<{
  join_mode: DomainJoinMode
  join_question: string
  join_answer: string
}>({
  join_mode: 'approval_only',
  join_question: '',
  join_answer: ''
})

// Apply Join Domain Modal
const applyJoinModalOpen = ref(false)
const applyingJoin = ref(false)
const applyJoinTarget = ref<DomainItem | null>(null)
const applyJoinMethod = ref<'quiz' | 'approval'>('quiz')
const applyQuizAnswer = ref('')
const applyJoinReason = ref('')

const canQuizJoin = computed(
  () =>
    applyJoinTarget.value?.join_mode === 'approval_or_quiz' &&
    Boolean(applyJoinTarget.value?.join_question)
)

async function fetchDomainList() {
  loading.value = true
  try {
    const q = searchText.value.trim()
    const isUidQuery = activeTab.value === 'square' && /^DOM-/i.test(q)
    const res = await getDomains({
      tab: activeTab.value,
      keyword: q && !isUidQuery ? q : undefined,
      domain_uid: isUidQuery ? q.toUpperCase() : undefined,
      page: page.value,
      page_size: pageSize.value
    })
    const normalized = normalizeList(res)
    domains.value = normalized.items
    total.value = normalized.total
  } catch {
    domains.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function fetchMyOrgs() {
  try {
    const res = await getOrgs({ page: 1, page_size: 100 })
    myOrgs.value = normalizeList(res).items
  } catch {
    myOrgs.value = []
  }
}

function handleTabChange() {
  searchText.value = ''
  page.value = 1
  fetchDomainList()
}

function handleSearch() {
  page.value = 1
  fetchDomainList()
}

function handlePageChange(newPage: number) {
  page.value = newPage
  fetchDomainList()
}

function handleCreateOrgChange() {
  if (!canCreateOrgPrivate.value && createForm.visibility === 'org_private') {
    createForm.visibility = 'public'
  }
}

function openCreateModal() {
  createForm.domain_name = ''
  createForm.domain_code = ''
  createForm.description = ''
  createForm.org_id = 0
  createForm.visibility = 'public'
  createForm.join_mode = 'approval_only'
  createForm.join_question = ''
  createForm.join_answer = ''
  createFormRef.value?.clearValidate()
  fetchMyOrgs()
  createModalOpen.value = true
}

async function handleCreateDomain() {
  if (!createFormRef.value) return
  const valid = await createFormRef.value.validate().catch(() => false)
  if (!valid) return

  creating.value = true
  try {
    const isPublicQuiz =
      createForm.visibility === 'public' && createForm.join_mode === 'approval_or_quiz'
    await createDomain({
      domain_name: createForm.domain_name.trim(),
      domain_code: createForm.domain_code.trim(),
      description: createForm.description.trim(),
      visibility: createForm.visibility,
      org_id: createForm.org_id,
      ...(createForm.visibility === 'public'
        ? {
            join_mode: createForm.join_mode,
            ...(isPublicQuiz
              ? {
                  join_question: createForm.join_question.trim(),
                  join_answer: createForm.join_answer.trim()
                }
              : {})
          }
        : {})
    })
    ElMessage.success('域创建成功')
    createModalOpen.value = false
    await domainStore.fetchDomains(true)
    fetchDomainList()
  } catch {
    // Handled by interceptor
  } finally {
    creating.value = false
  }
}

function openEditModal(domain: DomainItem) {
  editDomainId.value = domain.id
  editDomainOrgId.value = domain.org_id || 0
  editForm.domain_name = domain.domain_name
  editForm.description = domain.description || ''
  editForm.visibility = domain.visibility || 'public'
  editForm.allow_member_kb_write = Boolean(domain.allow_member_kb_write)
  editFormRef.value?.clearValidate()
  fetchMyOrgs()
  editModalOpen.value = true
}

async function handleUpdateDomain() {
  if (!editFormRef.value || !editDomainId.value) return
  const valid = await editFormRef.value.validate().catch(() => false)
  if (!valid) return

  editing.value = true
  try {
    await updateDomain(editDomainId.value, {
      domain_name: editForm.domain_name.trim(),
      description: editForm.description.trim(),
      visibility: editForm.visibility,
      allow_member_kb_write: editForm.allow_member_kb_write
    })
    ElMessage.success('域信息已更新')
    editModalOpen.value = false
    await domainStore.fetchDomains(true)
    fetchDomainList()
  } catch {
    // Handled by interceptor
  } finally {
    editing.value = false
  }
}

async function handleDeleteDomain(domain: DomainItem) {
  try {
    await ElMessageBox.confirm(
      `确认删除域「${domain.domain_name}」吗？删除后该域及其成员关系将被清除。`,
      '删除域确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'error'
      }
    )
    await deleteDomain(domain.id)
    ElMessage.success('域已删除')
    await domainStore.fetchDomains(true)
    fetchDomainList()
  } catch {
    // Cancelled or failed
  }
}

async function copyDomainUid(uid: string) {
  if (!uid) return
  try {
    await navigator.clipboard.writeText(uid)
    ElMessage.success(`已复制唯一域 ID：${uid}`)
  } catch {
    ElMessage.info(`唯一域 ID：${uid}`)
  }
}

function enterDomain(domain: DomainItem) {
  domainStore.setLastDomainId(domain.id)
  router.push(`/domains/${domain.id}/marketplace`)
}

function goMembers(domain: DomainItem) {
  domainStore.setLastDomainId(domain.id)
  router.push(`/domains/${domain.id}/members`)
}

function openJoinSettingsModal(domain: DomainItem) {
  joinSettingsDomainId.value = domain.id
  joinSettingsDomainName.value = domain.domain_name
  joinSettingsForm.join_mode = domain.join_mode || 'approval_only'
  joinSettingsForm.join_question = domain.join_question || ''
  joinSettingsForm.join_answer = ''
  joinSettingsFormRef.value?.clearValidate()
  joinSettingsModalOpen.value = true
}

async function handleUpdateJoinSettings() {
  if (!joinSettingsDomainId.value) return
  if (
    joinSettingsForm.join_mode === 'approval_or_quiz' &&
    !joinSettingsForm.join_question.trim()
  ) {
    ElMessage.warning('选择审批与问题并存时，请填写准入问题')
    return
  }

  savingJoinSettings.value = true
  try {
    const payload = {
      join_mode: joinSettingsForm.join_mode,
      ...(joinSettingsForm.join_mode === 'approval_or_quiz'
        ? {
            join_question: joinSettingsForm.join_question.trim(),
            ...(joinSettingsForm.join_answer.trim()
              ? { join_answer: joinSettingsForm.join_answer.trim() }
              : {})
          }
        : {})
    }
    await updateDomainJoinSettings(joinSettingsDomainId.value, payload)
    ElMessage.success('域准入设置已更新')
    joinSettingsModalOpen.value = false
    await fetchDomainList()
  } catch {
    // Handled by interceptor
  } finally {
    savingJoinSettings.value = false
  }
}

function openApplyJoinModal(domain: DomainItem) {
  applyJoinTarget.value = domain
  applyJoinMethod.value =
    domain.join_mode === 'approval_or_quiz' && domain.join_question ? 'quiz' : 'approval'
  applyQuizAnswer.value = ''
  applyJoinReason.value = ''
  applyJoinModalOpen.value = true
}

async function handleApplyJoinSubmit() {
  if (!applyJoinTarget.value) return
  if (canQuizJoin.value && applyJoinMethod.value === 'quiz') {
    const answer = applyQuizAnswer.value.trim()
    if (!answer) {
      ElMessage.warning('请输入准入问题答案')
      return
    }
    applyingJoin.value = true
    try {
      await joinDomainByQuiz(applyJoinTarget.value.id, answer)
      ElMessage.success('回答正确，已直接成为该域普通成员')
      applyJoinModalOpen.value = false
      await domainStore.fetchDomains(true)
      await fetchDomainList()
    } catch {
      // Handled by interceptor
    } finally {
      applyingJoin.value = false
    }
    return
  }

  applyingJoin.value = true
  try {
    await applyJoinDomain(
      applyJoinTarget.value.id,
      applyJoinReason.value.trim() || undefined
    )
    ElMessage.success('加入域申请已提交，请等待域所有者/管理员审批')
    applyJoinModalOpen.value = false
  } catch {
    // Handled by interceptor
  } finally {
    applyingJoin.value = false
  }
}

function getRoleTagType(role?: string) {
  if (role === 'owner') return 'danger'
  if (role === 'admin') return 'warning'
  return 'info'
}

onMounted(() => {
  fetchDomainList()
  domainStore.fetchDomains(true)
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">域大厅</h1>
        <p class="page-subtitle">浏览、搜索并进入域空间，管理域成员与共享权限边界</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateModal">创建域</el-button>
    </div>

    <div class="surface-card tabs-toolbar">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="我的域" name="mine" />
        <el-tab-pane label="域广场" name="square" />
      </el-tabs>

      <div class="search-row">
        <el-input
          v-model="searchText"
          :placeholder="
            activeTab === 'square'
              ? '按名称搜索公开域，或输入唯一域 ID (如 DOM-...) 精确查找私密域'
              : '搜索我的域名称或编码...'
          "
          clearable
          :prefix-icon="Search"
          style="max-width: 520px; flex: 1"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />
        <el-button type="primary" plain @click="handleSearch">搜索</el-button>
      </div>

      <el-alert
        v-if="activeTab === 'square'"
        title="私密域与组织私有域不会出现在域广场默认列表或名称搜索中，私密域仅支持通过唯一域 ID（如 DOM-...）精确查找。"
        type="info"
        show-icon
        :closable="false"
        style="margin-top: 12px"
      />
    </div>

    <div v-if="loading" class="domain-grid">
      <div v-for="i in 4" :key="i" class="surface-card domain-card">
        <el-skeleton :rows="4" animated />
      </div>
    </div>

    <div v-else-if="domains.length === 0" class="surface-card empty-box">
      <el-empty
        :description="
          activeTab === 'square'
            ? '未找到匹配的公开域或对应唯一域 ID 的私密域'
            : '暂无可访问的域，点击右上角「创建域」开始初始化团队空间'
        "
      />
    </div>

    <div v-else class="domain-grid">
      <div v-for="domain in domains" :key="domain.id" class="surface-card domain-card">
        <div class="domain-card-top">
          <div class="domain-icon">
            <el-icon :size="18"><OfficeBuilding /></el-icon>
          </div>
          <div class="badge-group">
            <el-tag
              :type="getDomainVisibilityMeta(domain.visibility).tagType"
              size="small"
              effect="plain"
            >
              <el-icon style="margin-right: 3px; vertical-align: -1px">
                <Lock v-if="domain.visibility === 'private' || domain.visibility === 'org_private'" />
                <Unlock v-else />
              </el-icon>
              {{ getDomainVisibilityMeta(domain.visibility).label }}
            </el-tag>
            <el-tag size="small" type="info" effect="light">
              {{ domain.org_id && domain.org_name ? domain.org_name : '个人' }}
            </el-tag>
            <el-tag
              v-if="domain.has_org_access && !domain.my_role"
              size="small"
              type="primary"
              effect="light"
            >
              组织共享
            </el-tag>
            <el-tag
              v-if="domain.my_role"
              :type="getRoleTagType(domain.my_role)"
              size="small"
              effect="light"
            >
              {{ getRoleLabel(domain.my_role) }}
            </el-tag>
          </div>
        </div>

        <h3 class="domain-name">{{ domain.domain_name }}</h3>

        <div class="domain-ids">
          <span class="domain-code mono-text">{{ domain.domain_code }}</span>
          <button
            v-if="domain.domain_uid"
            type="button"
            class="uid-copy-btn mono-text"
            title="点击复制唯一域 ID"
            @click="copyDomainUid(domain.domain_uid)"
          >
            <span>{{ domain.domain_uid }}</span>
            <el-icon :size="12"><CopyDocument /></el-icon>
          </button>
        </div>

        <p class="domain-desc">{{ domain.description || '暂无域描述' }}</p>

        <div class="domain-meta">
          <div class="member-stat tabular-nums">
            <el-icon :size="15"><UserFilled /></el-icon>
            <span>{{ domain.member_count ?? '-' }} 位成员</span>
          </div>
          <div class="meta-links">
            <el-button
              v-if="domain.can_manage"
              link
              type="primary"
              size="small"
              :icon="Edit"
              @click="openEditModal(domain)"
            >
              编辑
            </el-button>
            <el-button
              v-if="domain.can_manage"
              link
              type="primary"
              size="small"
              :icon="Setting"
              @click="openJoinSettingsModal(domain)"
            >
              准入设置
            </el-button>
            <el-button
              v-if="domain.can_manage"
              link
              type="danger"
              size="small"
              :icon="Delete"
              @click="handleDeleteDomain(domain)"
            >
              删除
            </el-button>
            <el-button link type="info" size="small" @click="goMembers(domain)">
              成员管理
            </el-button>
          </div>
        </div>

        <div class="domain-actions">
          <el-button
            v-if="domain.my_role || domain.has_org_access"
            type="primary"
            class="enter-btn"
            @click="enterDomain(domain)"
          >
            <span>进入域空间</span>
            <el-icon class="el-icon--right"><Right /></el-icon>
          </el-button>
          <el-button
            v-else-if="domain.visibility === 'public'"
            type="primary"
            plain
            class="enter-btn"
            @click="openApplyJoinModal(domain)"
          >
            <span>申请加入域</span>
          </el-button>
          <el-button v-else type="primary" class="enter-btn" @click="enterDomain(domain)">
            <span>进入域空间</span>
            <el-icon class="el-icon--right"><Right /></el-icon>
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="total > pageSize" class="pagination-wrap">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        background
        @current-change="handlePageChange"
      />
    </div>

    <!-- Create Domain Modal -->
    <el-dialog v-model="createModalOpen" title="创建域" width="520px" destroy-on-close>
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-position="top"
      >
        <el-form-item label="域名称" prop="domain_name">
          <el-input v-model="createForm.domain_name" placeholder="例如：智能客服研发域" />
        </el-form-item>
        <el-form-item label="域编码" prop="domain_code">
          <el-input
            v-model="createForm.domain_code"
            placeholder="例如：smart-cs-domain"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="所属组织">
          <el-select
            v-model="createForm.org_id"
            style="width: 100%"
            @change="handleCreateOrgChange"
          >
            <el-option label="个人 (未加入组织或个人私有)" :value="0" />
            <el-option
              v-for="org in myOrgs"
              :key="org.id"
              :label="`${org.org_name} (${org.org_code})`"
              :value="org.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="createForm.visibility" class="visibility-radios">
            <el-radio value="public">
              {{ createForm.org_id > 0 ? '公开域 / 组织公有域（域广场可见，组织成员共享）' : '公开域（域广场可见/可搜）' }}
            </el-radio>
            <el-radio value="private">私密域（仅通过唯一域 ID 搜索）</el-radio>
            <el-radio v-if="canCreateOrgPrivate" value="org_private">
              组织私有域（仅组织创建者/管理员可管理与邀请，对其他组织成员不可见）
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="createForm.visibility === 'public'">
          <el-form-item label="公开域准入方式">
            <el-radio-group v-model="createForm.join_mode" class="visibility-radios">
              <el-radio value="approval_only">仅审批（默认）</el-radio>
              <el-radio value="approval_or_quiz">审批与问题并存（答对问题可直接加入，也可提交审批）</el-radio>
            </el-radio-group>
          </el-form-item>
          <template v-if="createForm.join_mode === 'approval_or_quiz'">
            <el-form-item label="准入问题" prop="join_question">
              <el-input
                v-model="createForm.join_question"
                placeholder="例如：本域研发团队每周例会是周几？"
              />
            </el-form-item>
            <el-form-item label="预设答案" prop="join_answer">
              <el-input
                v-model="createForm.join_answer"
                placeholder="请输入用户答对即可直接加入的预设答案"
              />
            </el-form-item>
          </template>
        </template>
        <el-form-item label="描述说明" prop="description">
          <el-input
            v-model="createForm.description"
            type="textarea"
            :rows="3"
            placeholder="说明该域承载的团队职责与 AI 能力范围..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreateDomain">
          确认创建
        </el-button>
      </template>
    </el-dialog>

    <!-- Edit Domain Modal -->
    <el-dialog v-model="editModalOpen" title="编辑域" width="520px" destroy-on-close>
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="editRules"
        label-position="top"
      >
        <el-form-item label="域名称" prop="domain_name">
          <el-input v-model="editForm.domain_name" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="editForm.visibility" class="visibility-radios">
            <el-radio value="public">公开域（域广场可见/可搜）</el-radio>
            <el-radio value="private">私密域（仅通过唯一域 ID 搜索）</el-radio>
            <el-radio v-if="canEditOrgPrivate" value="org_private">
              组织私有域（仅组织创建者/管理员可管理与邀请，对其他组织成员不可见）
            </el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述说明" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="成员知识库写权限">
          <el-switch
            v-model="editForm.allow_member_kb_write"
            active-text="允许普通成员编辑域内知识库"
            inactive-text="普通成员仅只读浏览"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="editing" @click="handleUpdateDomain">
          保存修改
        </el-button>
      </template>
    </el-dialog>

    <!-- Join Settings Modal -->
    <el-dialog
      v-model="joinSettingsModalOpen"
      :title="`准入设置 - ${joinSettingsDomainName}`"
      width="500px"
      destroy-on-close
    >
      <el-form ref="joinSettingsFormRef" :model="joinSettingsForm" label-position="top">
        <el-form-item label="准入方式">
          <el-radio-group v-model="joinSettingsForm.join_mode" class="visibility-radios">
            <el-radio value="approval_only">仅审批（默认）</el-radio>
            <el-radio value="approval_or_quiz">审批与问题并存（答对问题可直接加入，也可提交审批）</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="joinSettingsForm.join_mode === 'approval_or_quiz'">
          <el-form-item label="准入问题">
            <el-input
              v-model="joinSettingsForm.join_question"
              placeholder="例如：本域研发团队每周例会是周几？"
            />
          </el-form-item>
          <el-form-item label="预设答案（留空则保持原答案不变）">
            <el-input
              v-model="joinSettingsForm.join_answer"
              placeholder="留空表示保持原答案不变；如需修改请输入新答案"
            />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="joinSettingsModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="savingJoinSettings" @click="handleUpdateJoinSettings">
          保存设置
        </el-button>
      </template>
    </el-dialog>

    <!-- Apply Join Domain Modal -->
    <el-dialog
      v-model="applyJoinModalOpen"
      :title="`申请加入域 - ${applyJoinTarget?.domain_name || ''}`"
      width="480px"
      destroy-on-close
    >
      <template v-if="canQuizJoin">
        <el-tabs v-model="applyJoinMethod" class="join-method-tabs">
          <el-tab-pane label="回答问题直接加入" name="quiz">
            <div class="quiz-question-box">
              <div class="quiz-question-label">准入问题：</div>
              <div class="quiz-question-text">{{ applyJoinTarget?.join_question }}</div>
            </div>
            <el-form label-position="top">
              <el-form-item label="您的答案">
                <el-input
                  v-model="applyQuizAnswer"
                  placeholder="请输入问题答案，回答正确将直接成为该域普通成员"
                  @keyup.enter="handleApplyJoinSubmit"
                />
              </el-form-item>
            </el-form>
          </el-tab-pane>
          <el-tab-pane label="提交审批申请" name="approval">
            <el-form label-position="top">
              <el-form-item label="申请留言（选填）">
                <el-input
                  v-model="applyJoinReason"
                  type="textarea"
                  :rows="3"
                  placeholder="可附带一句留言，说明您的身份或加入原因，便于审核者判断"
                />
              </el-form-item>
            </el-form>
          </el-tab-pane>
        </el-tabs>
      </template>
      <template v-else>
        <el-form label-position="top">
          <el-form-item label="申请留言（选填）">
            <el-input
              v-model="applyJoinReason"
              type="textarea"
              :rows="3"
              placeholder="可附带一句留言，说明您的身份或加入原因，便于审核者判断"
            />
          </el-form-item>
        </el-form>
      </template>
      <template #footer>
        <el-button @click="applyJoinModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="applyingJoin" @click="handleApplyJoinSubmit">
          {{ canQuizJoin && applyJoinMethod === 'quiz' ? '验证答案并加入' : '提交审批申请' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.tabs-toolbar {
  padding: 12px 20px 18px;
}

.search-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.domain-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 290px), 1fr));
  gap: 18px;
}

.domain-card {
  padding: 22px;
  display: flex;
  flex-direction: column;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.domain-card:hover {
  transform: translateY(-2px);
  border-color: #a5b4fc;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.07);
}

.domain-card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 14px;
}

.domain-icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background-color: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.badge-group {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
}

.domain-name {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.domain-ids {
  margin-top: 5px;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.domain-code {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.uid-copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 1px solid var(--ascp-border);
  background: #f8fafc;
  color: var(--ascp-text-secondary);
  font-size: 11.5px;
  padding: 2px 7px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.uid-copy-btn:hover {
  border-color: var(--ascp-primary);
  color: var(--ascp-primary);
  background: var(--ascp-primary-soft);
}

.domain-desc {
  margin: 10px 0 16px;
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--ascp-text-secondary);
  min-height: 42px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.domain-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  border-top: 1px solid #f1f5f9;
}

.member-stat {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ascp-text-muted);
}

.meta-links {
  display: flex;
  align-items: center;
  gap: 4px;
}

.domain-actions {
  margin-top: 8px;
}

.enter-btn {
  width: 100%;
  font-weight: 500;
}

.visibility-radios {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.join-method-tabs {
  margin-top: -6px;
}

.quiz-question-box {
  margin-bottom: 14px;
  padding: 12px 14px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
}

.quiz-question-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--ascp-text-muted);
  margin-bottom: 4px;
}

.quiz-question-text {
  font-size: 14px;
  line-height: 1.5;
  color: var(--ascp-text-primary);
}

.empty-box {
  padding: 48px 16px;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  padding-top: 4px;
}

@media (max-width: 768px) {
  .tabs-toolbar {
    padding: 10px 14px 14px;
  }

  .search-row .el-input {
    width: 100%;
    max-width: 100%;
  }
}
</style>
