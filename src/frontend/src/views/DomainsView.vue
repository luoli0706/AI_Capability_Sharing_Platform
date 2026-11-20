<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { OfficeBuilding, Plus, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { addDomainMember, createDomain, getDomainMembers, getDomains } from '@/api/domain'
import type { DomainItem, DomainMember } from '@/types/api'
import { formatDateTime, getRoleLabel, normalizeList } from '@/utils/format'

const loading = ref(false)
const domains = ref<DomainItem[]>([])

const createModalOpen = ref(false)
const createFormRef = ref<FormInstance>()
const creating = ref(false)

const createForm = reactive({
  domain_name: '',
  domain_code: '',
  description: ''
})

const createRules: FormRules = {
  domain_name: [{ required: true, message: '请输入业务域名称', trigger: 'blur' }],
  domain_code: [
    { required: true, message: '请输入业务域编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ]
}

const memberDrawerOpen = ref(false)
const activeDomain = ref<DomainItem | null>(null)
const loadingMembers = ref(false)
const members = ref<DomainMember[]>([])

const memberFormRef = ref<FormInstance>()
const addingMember = ref(false)
const memberForm = reactive<{
  username: string
  domain_role: 'admin' | 'member'
}>({
  username: '',
  domain_role: 'member'
})

const memberRules: FormRules = {
  username: [{ required: true, message: '请输入要添加的用户名', trigger: 'blur' }],
  domain_role: [{ required: true, message: '请选择域角色', trigger: 'change' }]
}

async function fetchDomains() {
  loading.value = true
  try {
    const res = await getDomains({ page: 1, page_size: 100 })
    domains.value = normalizeList(res).items
  } catch {
    domains.value = []
  } finally {
    loading.value = false
  }
}

function openCreateModal() {
  createForm.domain_name = ''
  createForm.domain_code = ''
  createForm.description = ''
  createFormRef.value?.clearValidate()
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
      description: createForm.description.trim()
    })
    ElMessage.success('业务域创建成功')
    createModalOpen.value = false
    fetchDomains()
  } catch {
    // Handled by interceptor
  } finally {
    creating.value = false
  }
}

async function openMembersDrawer(domain: DomainItem) {
  activeDomain.value = domain
  memberDrawerOpen.value = true
  memberForm.username = ''
  memberForm.domain_role = 'member'
  await fetchMembers(domain.id)
}

async function fetchMembers(domainId: number) {
  loadingMembers.value = true
  try {
    const res = await getDomainMembers(domainId)
    members.value = normalizeList(res).items
  } catch {
    members.value = []
  } finally {
    loadingMembers.value = false
  }
}

async function handleAddMember() {
  if (!memberFormRef.value || !activeDomain.value) return
  const valid = await memberFormRef.value.validate().catch(() => false)
  if (!valid) return

  addingMember.value = true
  try {
    await addDomainMember(activeDomain.value.id, {
      username: memberForm.username.trim(),
      domain_role: memberForm.domain_role
    })
    ElMessage.success('成员添加成功')
    memberForm.username = ''
    await fetchMembers(activeDomain.value.id)
    fetchDomains()
  } catch {
    // Handled by interceptor
  } finally {
    addingMember.value = false
  }
}

function getRoleTagType(role?: string) {
  if (role === 'owner') return 'danger'
  if (role === 'admin') return 'warning'
  return 'info'
}

onMounted(() => {
  fetchDomains()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">业务域管理</h1>
        <p class="page-subtitle">按部门或项目划分隔离空间，管理域内成员角色与 AI 资产访问边界</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateModal">创建业务域</el-button>
    </div>

    <div v-if="loading" class="domain-grid">
      <div v-for="i in 4" :key="i" class="surface-card domain-card">
        <el-skeleton :rows="3" animated />
      </div>
    </div>

    <div v-else-if="domains.length === 0" class="surface-card empty-box">
      <el-empty description="暂无业务域，点击右上角「创建业务域」开始初始化组织空间" />
    </div>

    <div v-else class="domain-grid">
      <div v-for="domain in domains" :key="domain.id" class="surface-card domain-card">
        <div class="domain-card-top">
          <div class="domain-icon">
            <el-icon :size="18"><OfficeBuilding /></el-icon>
          </div>
          <el-tag
            v-if="domain.my_role"
            :type="getRoleTagType(domain.my_role)"
            size="small"
            effect="light"
          >
            {{ getRoleLabel(domain.my_role) }}
          </el-tag>
        </div>

        <h3 class="domain-name">{{ domain.domain_name }}</h3>
        <div class="domain-code mono-text">{{ domain.domain_code }}</div>
        <p class="domain-desc">{{ domain.description || '暂无业务域描述' }}</p>

        <div class="domain-footer">
          <div class="member-stat tabular-nums">
            <el-icon :size="15"><UserFilled /></el-icon>
            <span>{{ domain.member_count ?? '-' }} 位成员</span>
          </div>
          <el-button type="primary" link @click="openMembersDrawer(domain)">
            成员管理
          </el-button>
        </div>
      </div>
    </div>

    <!-- Create Domain Modal -->
    <el-dialog v-model="createModalOpen" title="创建业务域" width="480px" destroy-on-close>
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

    <!-- Members Drawer -->
    <el-drawer
      v-model="memberDrawerOpen"
      :title="`成员管理 - ${activeDomain?.domain_name || ''}`"
      size="580px"
      destroy-on-close
    >
      <div class="add-member-box">
        <div class="add-member-title">添加域成员</div>
        <el-form
          ref="memberFormRef"
          :model="memberForm"
          :rules="memberRules"
          inline
          class="add-member-form"
        >
          <el-form-item prop="username" style="flex: 1; margin-right: 10px">
            <el-input
              v-model="memberForm.username"
              placeholder="输入平台用户名（如 zhangsan）"
            />
          </el-form-item>
          <el-form-item prop="domain_role" style="width: 130px; margin-right: 10px">
            <el-select v-model="memberForm.domain_role">
              <el-option label="普通成员" value="member" />
              <el-option label="域管理员" value="admin" />
            </el-select>
          </el-form-item>
          <el-form-item style="margin-right: 0">
            <el-button type="primary" :loading="addingMember" @click="handleAddMember">
              添加
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <el-table v-loading="loadingMembers" :data="members" stripe style="width: 100%">
        <el-table-column label="用户" min-width="150">
          <template #default="{ row }">
            <div class="member-user">
              <span class="member-nickname">{{ row.nickname || row.username }}</span>
              <span class="member-username mono-text">@{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="域角色" width="110">
          <template #default="{ row }">
            <el-tag :type="getRoleTagType(row.domain_role)" size="small" effect="light">
              {{ getRoleLabel(row.domain_role) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="加入时间" width="150">
          <template #default="{ row }">
            {{ formatDateTime(row.joined_at) }}
          </template>
        </el-table-column>

        <template #empty>
          <el-empty description="该业务域暂无成员记录" />
        </template>
      </el-table>
    </el-drawer>
  </div>
</template>

<style scoped>
.domain-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
  gap: 16px;
}

.domain-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
}

.domain-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.domain-icon {
  width: 36px;
  height: 36px;
  border-radius: 9px;
  background-color: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  display: flex;
  align-items: center;
  justify-content: center;
}

.domain-name {
  margin: 0;
  font-size: 16.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.domain-code {
  margin-top: 3px;
  font-size: 12px;
  color: var(--ascp-text-muted);
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

.domain-footer {
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.member-stat {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--ascp-text-muted);
}

.empty-box {
  padding: 48px 16px;
}

.add-member-box {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 14px 16px;
  margin-bottom: 18px;
}

.add-member-title {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  margin-bottom: 10px;
}

.add-member-form {
  display: flex;
  align-items: flex-start;
}

.member-user {
  display: flex;
  flex-direction: column;
}

.member-nickname {
  font-weight: 500;
  color: var(--ascp-text-primary);
}

.member-username {
  font-size: 12px;
  color: var(--ascp-text-muted);
}
</style>
