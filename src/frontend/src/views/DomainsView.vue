<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  CopyDocument,
  Edit,
  Lock,
  OfficeBuilding,
  Plus,
  Right,
  Search,
  Unlock,
  UserFilled
} from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { createDomain, getDomains, updateDomain } from '@/api/domain'
import { getOrgs } from '@/api/org'
import { useDomainStore } from '@/stores/domain'
import type { DomainItem, DomainVisibility, OrgItem } from '@/types/api'
import { getRoleLabel, normalizeList } from '@/utils/format'

const router = useRouter()
const domainStore = useDomainStore()

const activeTab = ref<'mine' | 'square'>('mine')
const loading = ref(false)
const domains = ref<DomainItem[]>([])
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
}>({
  domain_name: '',
  domain_code: '',
  description: '',
  org_id: 0,
  visibility: 'public'
})

const createRules: FormRules = {
  domain_name: [{ required: true, message: '请输入业务域名称', trigger: 'blur' }],
  domain_code: [
    { required: true, message: '请输入业务域编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ]
}

// Edit Modal
const editModalOpen = ref(false)
const editFormRef = ref<FormInstance>()
const editing = ref(false)
const editDomainId = ref<number | null>(null)
const editForm = reactive<{
  domain_name: string
  description: string
  visibility: DomainVisibility
}>({
  domain_name: '',
  description: '',
  visibility: 'public'
})

const editRules: FormRules = {
  domain_name: [{ required: true, message: '请输入业务域名称', trigger: 'blur' }]
}

async function fetchDomainList() {
  loading.value = true
  try {
    const q = searchText.value.trim()
    const isUidQuery = activeTab.value === 'square' && /^DOM-/i.test(q)
    const res = await getDomains({
      tab: activeTab.value,
      keyword: q && !isUidQuery ? q : undefined,
      domain_uid: isUidQuery ? q.toUpperCase() : undefined,
      page: 1,
      page_size: 100
    })
    domains.value = normalizeList(res).items
  } catch {
    domains.value = []
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
  fetchDomainList()
}

function openCreateModal() {
  createForm.domain_name = ''
  createForm.domain_code = ''
  createForm.description = ''
  createForm.org_id = 0
  createForm.visibility = 'public'
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
    await createDomain({
      domain_name: createForm.domain_name.trim(),
      domain_code: createForm.domain_code.trim(),
      description: createForm.description.trim(),
      visibility: createForm.visibility,
      org_id: createForm.org_id
    })
    ElMessage.success('业务域创建成功')
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
  editForm.domain_name = domain.domain_name
  editForm.description = domain.description || ''
  editForm.visibility = domain.visibility || 'public'
  editFormRef.value?.clearValidate()
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
      visibility: editForm.visibility
    })
    ElMessage.success('业务域信息已更新')
    editModalOpen.value = false
    await domainStore.fetchDomains(true)
    fetchDomainList()
  } catch {
    // Handled by interceptor
  } finally {
    editing.value = false
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
        <h1 class="page-title">业务域大厅</h1>
        <p class="page-subtitle">选择一个研发团队业务域进入，查看域内能力广场、管理 AI 资产与私有知识库</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateModal">创建业务域</el-button>
    </div>

    <div class="surface-card tabs-toolbar">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="我的业务域" name="mine" />
        <el-tab-pane label="域广场" name="square" />
      </el-tabs>

      <div class="search-row">
        <el-input
          v-model="searchText"
          :placeholder="
            activeTab === 'square'
              ? '按名称搜索公开域，或输入唯一域 ID (如 DOM-...) 精确查找私密域'
              : '搜索我的业务域名称或编码...'
          "
          clearable
          :prefix-icon="Search"
          style="max-width: 520px; flex: 1"
          @keyup.enter="fetchDomainList"
          @clear="fetchDomainList"
        />
        <el-button type="primary" plain @click="fetchDomainList">搜索</el-button>
      </div>

      <el-alert
        v-if="activeTab === 'square'"
        title="私密域不会出现在域广场默认列表或名称搜索中，仅支持通过唯一域 ID（如 DOM-...）精确查找。"
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
            : '暂无可访问的业务域，点击右上角「创建业务域」开始初始化团队空间'
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
              :type="domain.visibility === 'private' ? 'warning' : 'success'"
              size="small"
              effect="plain"
            >
              <el-icon style="margin-right: 3px; vertical-align: -1px">
                <Lock v-if="domain.visibility === 'private'" />
                <Unlock v-else />
              </el-icon>
              {{ domain.visibility === 'private' ? '私密' : '公开' }}
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

        <p class="domain-desc">{{ domain.description || '暂无业务域描述' }}</p>

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
              编辑域
            </el-button>
            <el-button link type="info" size="small" @click="goMembers(domain)">
              成员管理
            </el-button>
          </div>
        </div>

        <div class="domain-actions">
          <el-button type="primary" class="enter-btn" @click="enterDomain(domain)">
            <span>进入域空间</span>
            <el-icon class="el-icon--right"><Right /></el-icon>
          </el-button>
        </div>
      </div>
    </div>

    <!-- Create Domain Modal -->
    <el-dialog v-model="createModalOpen" title="创建业务域" width="500px" destroy-on-close>
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-position="top"
      >
        <el-form-item label="业务域名称" prop="domain_name">
          <el-input v-model="createForm.domain_name" placeholder="例如：智能客服研发域" />
        </el-form-item>
        <el-form-item label="业务域编码" prop="domain_code">
          <el-input
            v-model="createForm.domain_code"
            placeholder="例如：smart-cs-domain"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="所属组织">
          <el-select v-model="createForm.org_id" style="width: 100%">
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
            <el-radio value="public">公开域（域广场可见/可搜）</el-radio>
            <el-radio value="private">私密域（仅通过唯一域 ID 搜索）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述说明" prop="description">
          <el-input
            v-model="createForm.description"
            type="textarea"
            :rows="3"
            placeholder="说明该业务域承载的团队职责与 AI 能力范围..."
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
    <el-dialog v-model="editModalOpen" title="编辑业务域" width="480px" destroy-on-close>
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="editRules"
        label-position="top"
      >
        <el-form-item label="业务域名称" prop="domain_name">
          <el-input v-model="editForm.domain_name" />
        </el-form-item>
        <el-form-item label="可见性">
          <el-radio-group v-model="editForm.visibility" class="visibility-radios">
            <el-radio value="public">公开域（域广场可见/可搜）</el-radio>
            <el-radio value="private">私密域（仅通过唯一域 ID 搜索）</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述说明" prop="description">
          <el-input v-model="editForm.description" type="textarea" :rows="3" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="editing" @click="handleUpdateDomain">
          保存修改
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
}

.domain-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(330px, 1fr));
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
  gap: 6px;
}

.empty-box {
  padding: 48px 16px;
}
</style>
