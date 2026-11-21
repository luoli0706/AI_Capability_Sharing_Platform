<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Delete, Edit, Grid, OfficeBuilding, Plus, Right, Search, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { getDomains } from '@/api/domain'
import {
  addOrgMember,
  createOrg,
  deleteOrg,
  getOrgDetail,
  getOrgMembers,
  getOrgs,
  joinOrg,
  removeOrgMember,
  updateOrg
} from '@/api/org'
import { useDomainStore } from '@/stores/domain'
import type { DomainItem, OrgDetail, OrgItem, OrgMember } from '@/types/api'
import { formatDateTime, getRoleLabel, normalizeList } from '@/utils/format'

const router = useRouter()
const domainStore = useDomainStore()

const activeTab = ref<'mine' | 'square'>('mine')
const loading = ref(false)
const orgs = ref<OrgItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(12)
const keyword = ref('')

// Join Org Modal
const joinModalOpen = ref(false)
const joining = ref(false)
const joinTarget = ref<OrgItem | null>(null)
const joinReason = ref('')

const createModalOpen = ref(false)
const createFormRef = ref<FormInstance>()
const creating = ref(false)
const createForm = reactive({
  org_name: '',
  org_code: '',
  description: ''
})

const createRules: FormRules = {
  org_name: [{ required: true, message: '请输入组织名称', trigger: 'blur' }],
  org_code: [
    { required: true, message: '请输入组织编码', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ]
}

// Edit Modal
const editModalOpen = ref(false)
const editFormRef = ref<FormInstance>()
const editing = ref(false)
const editOrgId = ref<number | null>(null)
const editForm = reactive({
  org_name: '',
  description: ''
})

const editRules: FormRules = {
  org_name: [{ required: true, message: '请输入组织名称', trigger: 'blur' }]
}

// Detail Drawer
const drawerOpen = ref(false)
const drawerTab = ref<'members' | 'domains'>('members')
const loadingDetail = ref(false)
const activeOrg = ref<OrgDetail | null>(null)
const orgMembers = ref<OrgMember[]>([])
const memberPage = ref(1)
const memberPageSize = ref(10)
const pagedOrgMembers = computed(() => {
  const start = (memberPage.value - 1) * memberPageSize.value
  return orgMembers.value.slice(start, start + memberPageSize.value)
})
const orgPublicDomains = ref<DomainItem[]>([])

const canManageActiveOrg = computed(
  () => activeOrg.value?.my_role === 'owner' || activeOrg.value?.my_role === 'admin'
)

const memberFormRef = ref<FormInstance>()
const addingMember = ref(false)
const memberForm = reactive<{
  username: string
  org_role: 'admin' | 'member'
}>({
  username: '',
  org_role: 'member'
})

const memberRules: FormRules = {
  username: [{ required: true, message: '请输入要添加的用户名', trigger: 'blur' }],
  org_role: [{ required: true, message: '请选择组织角色', trigger: 'change' }]
}

async function fetchOrgs() {
  loading.value = true
  try {
    const res = await getOrgs({
      tab: activeTab.value,
      keyword: keyword.value.trim() || undefined,
      page: page.value,
      page_size: pageSize.value
    })
    const normalized = normalizeList(res)
    orgs.value = normalized.items
    total.value = normalized.total
  } catch {
    orgs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function handleTabChange() {
  keyword.value = ''
  page.value = 1
  fetchOrgs()
}

function openJoinModal(org: OrgItem) {
  joinTarget.value = org
  joinReason.value = ''
  joinModalOpen.value = true
}

async function handleJoinOrg() {
  if (!joinTarget.value) return
  joining.value = true
  try {
    await joinOrg(joinTarget.value.id, joinReason.value.trim() || undefined)
    ElMessage.success('加入组织申请已提交，请等待组织管理员审批')
    joinModalOpen.value = false
  } catch {
    // Handled by interceptor
  } finally {
    joining.value = false
  }
}

function handleCardClick(org: OrgItem) {
  if (org.my_role) {
    openOrgDrawer(org)
  } else {
    openJoinModal(org)
  }
}

function handleSearch() {
  page.value = 1
  fetchOrgs()
}

function handlePageChange(newPage: number) {
  page.value = newPage
  fetchOrgs()
}

function openCreateModal() {
  createForm.org_name = ''
  createForm.org_code = ''
  createForm.description = ''
  createFormRef.value?.clearValidate()
  createModalOpen.value = true
}

async function handleCreateOrg() {
  if (!createFormRef.value) return
  const valid = await createFormRef.value.validate().catch(() => false)
  if (!valid) return

  creating.value = true
  try {
    await createOrg({
      org_name: createForm.org_name.trim(),
      org_code: createForm.org_code.trim(),
      description: createForm.description.trim()
    })
    ElMessage.success('组织创建成功')
    createModalOpen.value = false
    fetchOrgs()
  } catch {
    // Handled by interceptor
  } finally {
    creating.value = false
  }
}

function openEditModal(org: OrgItem) {
  editOrgId.value = org.id
  editForm.org_name = org.org_name
  editForm.description = org.description || ''
  editFormRef.value?.clearValidate()
  editModalOpen.value = true
}

async function handleUpdateOrg() {
  if (!editFormRef.value || !editOrgId.value) return
  const valid = await editFormRef.value.validate().catch(() => false)
  if (!valid) return

  editing.value = true
  try {
    await updateOrg(editOrgId.value, {
      org_name: editForm.org_name.trim(),
      description: editForm.description.trim()
    })
    ElMessage.success('组织信息已更新')
    editModalOpen.value = false
    await fetchOrgs()
    if (activeOrg.value?.id === editOrgId.value) {
      await loadOrgDetailData(editOrgId.value)
    }
  } catch {
    // Handled by interceptor
  } finally {
    editing.value = false
  }
}

async function handleDeleteOrg(org: OrgItem) {
  try {
    await ElMessageBox.confirm(
      `确认删除组织「${org.org_name}」吗？删除后该组织及其成员关系将被清除。`,
      '删除组织确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'error'
      }
    )
    await deleteOrg(org.id)
    ElMessage.success('组织已删除')
    if (activeOrg.value?.id === org.id) {
      drawerOpen.value = false
    }
    fetchOrgs()
  } catch {
    // Cancelled or failed
  }
}

async function openOrgDrawer(org: OrgItem) {
  activeOrg.value = org
  drawerTab.value = 'members'
  memberPage.value = 1
  memberForm.username = ''
  memberForm.org_role = 'member'
  drawerOpen.value = true
  await loadOrgDetailData(org.id)
}

async function loadOrgDetailData(orgId: number) {
  loadingDetail.value = true
  try {
    const [detailRes, membersRes, domainsRes] = await Promise.all([
      getOrgDetail(orgId).catch(() => null),
      getOrgMembers(orgId).catch(() => []),
      getDomains({ org_id: orgId, page: 1, page_size: 100 }).catch(() => [])
    ])
    if (detailRes) {
      activeOrg.value = detailRes
    }
    const membersList = normalizeList(membersRes).items
    orgMembers.value = membersList.length > 0 ? membersList : detailRes?.members || []
    const domainsList = normalizeList(domainsRes).items
    orgPublicDomains.value =
      domainsList.length > 0 ? domainsList : detailRes?.public_domains || []
  } finally {
    loadingDetail.value = false
  }
}

async function handleAddMember() {
  if (!memberFormRef.value || !activeOrg.value) return
  const valid = await memberFormRef.value.validate().catch(() => false)
  if (!valid) return

  addingMember.value = true
  try {
    await addOrgMember(activeOrg.value.id, {
      username: memberForm.username.trim(),
      org_role: memberForm.org_role
    })
    ElMessage.success('组织成员添加成功')
    memberForm.username = ''
    await loadOrgDetailData(activeOrg.value.id)
    fetchOrgs()
  } catch {
    // Handled by interceptor
  } finally {
    addingMember.value = false
  }
}

async function handleRemoveMember(member: OrgMember) {
  if (!activeOrg.value) return
  try {
    await ElMessageBox.confirm(
      `确认将成员「${member.nickname || member.username}」移出当前组织吗？`,
      '移除成员确认',
      {
        confirmButtonText: '确认移除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await removeOrgMember(activeOrg.value.id, member.user_id)
    ElMessage.success('已移出组织')
    await loadOrgDetailData(activeOrg.value.id)
    fetchOrgs()
  } catch {
    // Cancelled or failed
  }
}

function enterPublicDomain(domain: DomainItem) {
  drawerOpen.value = false
  domainStore.setLastDomainId(domain.id)
  router.push(`/domains/${domain.id}/marketplace`)
}

function getRoleTagType(role?: string) {
  if (role === 'owner') return 'danger'
  if (role === 'admin') return 'warning'
  return 'info'
}

onMounted(() => {
  fetchOrgs()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">组织中心</h1>
        <p class="page-subtitle">管理研发团队组织架构，组织内所有成员可自动共享所属组织的公开域权限</p>
      </div>
      <el-button type="primary" :icon="Plus" @click="openCreateModal">创建组织</el-button>
    </div>

    <div class="surface-card tabs-toolbar">
      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="我的组织" name="mine" />
        <el-tab-pane label="组织广场" name="square" />
      </el-tabs>

      <div class="search-row">
        <el-input
          v-model="keyword"
          placeholder="按组织名称或编码检索..."
          clearable
          :prefix-icon="Search"
          style="max-width: 360px"
          @keyup.enter="handleSearch"
          @clear="handleSearch"
        />
        <el-button type="primary" plain @click="handleSearch">搜索</el-button>
      </div>
    </div>

    <div v-if="loading" class="org-grid">
      <div v-for="i in 4" :key="i" class="surface-card org-card">
        <el-skeleton :rows="3" animated />
      </div>
    </div>

    <div v-else-if="orgs.length === 0" class="surface-card empty-box">
      <el-empty
        :description="
          activeTab === 'square'
            ? '未找到匹配的组织'
            : '暂未加入任何组织，可前往「组织广场」申请加入或点击右上角「创建组织」'
        "
      />
    </div>

    <div v-else class="org-grid">
      <article
        v-for="org in orgs"
        :key="org.id"
        class="surface-card org-card"
        tabindex="0"
        @click="handleCardClick(org)"
        @keyup.enter="handleCardClick(org)"
      >
        <div class="org-card-top">
          <div class="org-icon">
            <el-icon :size="18"><Grid /></el-icon>
          </div>
          <div style="display: flex; gap: 6px; align-items: center">
            <el-tag v-if="org.my_role && activeTab === 'square'" size="small" type="success" effect="plain">
              已加入
            </el-tag>
            <el-tag
              v-if="org.my_role"
              :type="getRoleTagType(org.my_role)"
              size="small"
              effect="light"
            >
              {{ getRoleLabel(org.my_role) }}
            </el-tag>
            <el-tag v-else size="small" type="info" effect="plain">未加入</el-tag>
          </div>
        </div>

        <h3 class="org-name">{{ org.org_name }}</h3>
        <div class="org-code mono-text">{{ org.org_code }}</div>
        <p class="org-desc">{{ org.description || '暂无组织说明' }}</p>

        <div class="org-footer">
          <div class="org-stats tabular-nums">
            <span class="stat-item">
              <el-icon :size="14"><UserFilled /></el-icon>
              <span>{{ org.member_count ?? 0 }} 成员</span>
            </span>
            <span class="stat-item">
              <el-icon :size="14"><OfficeBuilding /></el-icon>
              <span>{{ org.domain_count ?? org.public_domain_count ?? 0 }} 公有域</span>
            </span>
          </div>
          <div class="org-card-actions" @click.stop>
            <template v-if="org.my_role">
              <el-button
                v-if="org.my_role === 'owner' || org.my_role === 'admin'"
                link
                type="primary"
                size="small"
                :icon="Edit"
                @click="openEditModal(org)"
              >
                编辑
              </el-button>
              <el-button
                v-if="org.my_role === 'owner'"
                link
                type="danger"
                size="small"
                :icon="Delete"
                @click="handleDeleteOrg(org)"
              >
                删除
              </el-button>
              <span class="view-detail-hint" @click="openOrgDrawer(org)">管理详情 →</span>
            </template>
            <el-button
              v-else
              type="primary"
              size="small"
              plain
              @click="openJoinModal(org)"
            >
              申请加入
            </el-button>
          </div>
        </div>
      </article>
    </div>

    <div v-if="total > 0" class="pagination-wrap">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="total, prev, pager, next"
        background
        @current-change="handlePageChange"
      />
    </div>

    <!-- Create Org Modal -->
    <el-dialog v-model="createModalOpen" title="创建组织" width="460px" destroy-on-close>
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-position="top"
      >
        <el-form-item label="组织名称" prop="org_name">
          <el-input v-model="createForm.org_name" placeholder="例如：基础架构研发部" />
        </el-form-item>
        <el-form-item label="组织编码" prop="org_code">
          <el-input
            v-model="createForm.org_code"
            placeholder="例如：infra-team"
            class="mono-text"
          />
        </el-form-item>
        <el-form-item label="组织描述" prop="description">
          <el-input
            v-model="createForm.description"
            type="textarea"
            :rows="3"
            placeholder="说明该组织的研发方向与成员协作范围..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="handleCreateOrg">
          确认创建
        </el-button>
      </template>
    </el-dialog>

    <!-- Edit Org Modal -->
    <el-dialog v-model="editModalOpen" title="编辑组织" width="460px" destroy-on-close>
      <el-form
        ref="editFormRef"
        :model="editForm"
        :rules="editRules"
        label-position="top"
      >
        <el-form-item label="组织名称" prop="org_name">
          <el-input v-model="editForm.org_name" maxlength="128" />
        </el-form-item>
        <el-form-item label="组织简介" prop="description">
          <el-input
            v-model="editForm.description"
            type="textarea"
            :rows="3"
            maxlength="512"
            show-word-limit
            placeholder="请输入组织简介..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="editing" @click="handleUpdateOrg">
          保存修改
        </el-button>
      </template>
    </el-dialog>

    <!-- Join Org Modal -->
    <el-dialog
      v-model="joinModalOpen"
      :title="`申请加入组织 - ${joinTarget?.org_name || ''}`"
      width="440px"
      destroy-on-close
    >
      <el-form label-position="top">
        <el-form-item label="申请理由（选填）">
          <el-input
            v-model="joinReason"
            type="textarea"
            :rows="3"
            placeholder="请简要说明您的团队角色或加入原因..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="joinModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="joining" @click="handleJoinOrg">
          提交申请
        </el-button>
      </template>
    </el-dialog>

    <!-- Org Detail Drawer -->
    <el-drawer
      v-model="drawerOpen"
      :title="`组织详情 - ${activeOrg?.org_name || ''}`"
      size="620px"
      destroy-on-close
    >
      <div v-if="loadingDetail" class="drawer-loading">
        <el-skeleton :rows="6" animated />
      </div>

      <div v-else-if="activeOrg" class="drawer-body">
        <div class="org-summary-box">
          <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px;">
            <div class="summary-code mono-text">编码：{{ activeOrg.org_code }}</div>
            <el-button
              v-if="canManageActiveOrg"
              link
              type="primary"
              size="small"
              :icon="Edit"
              @click="openEditModal(activeOrg)"
            >
              编辑组织
            </el-button>
          </div>
          <p class="summary-desc">{{ activeOrg.description || '暂无组织说明' }}</p>
        </div>

        <el-tabs v-model="drawerTab">
          <el-tab-pane label="组织成员" name="members">
            <div v-if="canManageActiveOrg" class="add-member-box">
              <el-form
                ref="memberFormRef"
                :model="memberForm"
                :rules="memberRules"
                inline
                class="add-member-form"
              >
                <el-form-item prop="username" style="flex: 1; margin-right: 8px">
                  <el-input
                    v-model="memberForm.username"
                    placeholder="输入用户名添加成员"
                  />
                </el-form-item>
                <el-form-item prop="org_role" style="width: 120px; margin-right: 8px">
                  <el-select v-model="memberForm.org_role">
                    <el-option label="成员" value="member" />
                    <el-option label="管理员" value="admin" />
                  </el-select>
                </el-form-item>
                <el-form-item style="margin-right: 0">
                  <el-button type="primary" :loading="addingMember" @click="handleAddMember">
                    添加
                  </el-button>
                </el-form-item>
              </el-form>
            </div>

            <el-table :data="pagedOrgMembers" stripe style="width: 100%">
              <el-table-column label="成员" min-width="150">
                <template #default="{ row }">
                  <div class="member-cell">
                    <span class="member-nick">{{ row.nickname || row.username }}</span>
                    <span class="member-uname mono-text">@{{ row.username }}</span>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="角色" width="100">
                <template #default="{ row }">
                  <el-tag :type="getRoleTagType(row.org_role)" size="small" effect="light">
                    {{ getRoleLabel(row.org_role) }}
                  </el-tag>
                </template>
              </el-table-column>

              <el-table-column label="加入时间" width="145">
                <template #default="{ row }">
                  {{ formatDateTime(row.joined_at) }}
                </template>
              </el-table-column>

              <el-table-column v-if="canManageActiveOrg" label="操作" width="80" fixed="right">
                <template #default="{ row }">
                  <el-button
                    v-if="row.org_role !== 'owner'"
                    link
                    type="danger"
                    size="small"
                    :icon="Delete"
                    @click="handleRemoveMember(row)"
                  >
                    移除
                  </el-button>
                </template>
              </el-table-column>

              <template #empty>
                <el-empty description="暂无组织成员" :image-size="60" />
              </template>
            </el-table>

            <div v-if="orgMembers.length > 0" class="drawer-pagination">
              <el-pagination
                v-model:current-page="memberPage"
                :page-size="memberPageSize"
                :total="orgMembers.length"
                layout="total, prev, pager, next"
                background
              />
            </div>
          </el-tab-pane>

          <el-tab-pane label="组织公有域" name="domains">
            <el-alert
              title="组织内所有成员共享以下组织公有域权限，可直接进入域内能力广场与工作空间。"
              type="info"
              show-icon
              :closable="false"
              style="margin-bottom: 14px"
            />

            <el-empty
              v-if="orgPublicDomains.length === 0"
              description="该组织下暂无公有域"
              :image-size="64"
            />

            <div v-else class="public-domain-list">
              <div v-for="dom in orgPublicDomains" :key="dom.id" class="public-domain-item">
                <div class="pd-info">
                  <div class="pd-title-row">
                    <span class="pd-name">{{ dom.domain_name }}</span>
                    <span v-if="dom.domain_uid" class="pd-uid mono-text">{{ dom.domain_uid }}</span>
                  </div>
                  <div class="pd-desc">{{ dom.description || '暂无描述' }}</div>
                </div>
                <el-button type="primary" size="small" @click="enterPublicDomain(dom)">
                  <span>进入域空间</span>
                  <el-icon class="el-icon--right"><Right /></el-icon>
                </el-button>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>
  </div>
</template>

<style scoped>
.org-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(310px, 1fr));
  gap: 18px;
}

.org-card {
  padding: 22px;
  display: flex;
  flex-direction: column;
  cursor: pointer;
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease;
}

.org-card:hover {
  transform: translateY(-2px);
  border-color: #a5b4fc;
  box-shadow: 0 10px 24px rgba(15, 23, 42, 0.07);
}

.org-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.org-icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  background-color: var(--ascp-primary-soft);
  color: var(--ascp-primary);
  display: flex;
  align-items: center;
  justify-content: center;
}

.org-name {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.org-code {
  margin-top: 3px;
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.org-desc {
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

.org-footer {
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid #f1f5f9;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.org-stats {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.view-detail-hint {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--ascp-primary);
}

.org-card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.empty-box {
  padding: 48px 16px;
}

.drawer-loading {
  padding: 12px 0;
}

.org-summary-box {
  background: #f8fafc;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  padding: 12px 14px;
  margin-bottom: 16px;
}

.summary-code {
  font-size: 12.5px;
  color: var(--ascp-text-muted);
}

.summary-desc {
  margin: 6px 0 0;
  font-size: 13.5px;
  color: var(--ascp-text-secondary);
}

.add-member-box {
  margin-bottom: 14px;
}

.add-member-form {
  display: flex;
  align-items: flex-start;
}

.add-member-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.member-cell {
  display: flex;
  flex-direction: column;
}

.member-nick {
  font-weight: 500;
  color: var(--ascp-text-primary);
}

.member-uname {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.public-domain-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.public-domain-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--ascp-border);
  border-radius: 10px;
  background: #ffffff;
}

.pd-info {
  min-width: 0;
}

.pd-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pd-name {
  font-weight: 600;
  font-size: 14.5px;
  color: var(--ascp-text-primary);
}

.pd-uid {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
  background: #f1f5f9;
  padding: 1px 6px;
  border-radius: 6px;
}

.pd-desc {
  margin-top: 4px;
  font-size: 13px;
  color: var(--ascp-text-secondary);
}

.tabs-toolbar {
  padding: 12px 20px 18px;
}

.search-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pagination-wrap,
.drawer-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 12px;
}
</style>
