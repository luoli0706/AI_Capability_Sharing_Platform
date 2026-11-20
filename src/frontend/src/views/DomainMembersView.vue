<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Delete, UserFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  addDomainMember,
  getDomainMembers,
  removeDomainMember,
  updateDomainMemberRole
} from '@/api/domain'
import { useDomainStore } from '@/stores/domain'
import type { DomainMember } from '@/types/api'
import { formatDateTime, getRoleLabel, normalizeList } from '@/utils/format'

const route = useRoute()
const domainStore = useDomainStore()

const domainId = computed(() => Number(route.params.domainId))
const currentDomain = computed(() => domainStore.getDomainById(domainId.value))

const canManage = computed(() => Boolean(currentDomain.value?.can_manage))
const isOwner = computed(() => currentDomain.value?.my_role === 'owner')

const loading = ref(false)
const members = ref<DomainMember[]>([])
const memberPage = ref(1)
const memberPageSize = ref(10)
const pagedMembers = computed(() => {
  const start = (memberPage.value - 1) * memberPageSize.value
  return members.value.slice(start, start + memberPageSize.value)
})

const formRef = ref<FormInstance>()
const adding = ref(false)
const memberForm = reactive<{
  username: string
  domain_role: 'admin' | 'member'
}>({
  username: '',
  domain_role: 'member'
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入要添加的用户名', trigger: 'blur' }],
  domain_role: [{ required: true, message: '请选择域角色', trigger: 'change' }]
}

async function fetchMembers() {
  if (!domainId.value) return
  loading.value = true
  try {
    const res = await getDomainMembers(domainId.value)
    members.value = normalizeList(res).items
  } catch {
    members.value = []
  } finally {
    loading.value = false
  }
}

async function handleAddMember() {
  if (!formRef.value || !domainId.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  adding.value = true
  try {
    await addDomainMember(domainId.value, {
      username: memberForm.username.trim(),
      domain_role: memberForm.domain_role
    })
    ElMessage.success('成员添加成功')
    memberForm.username = ''
    await fetchMembers()
    domainStore.fetchDomains(true)
  } catch {
    // Handled by interceptor
  } finally {
    adding.value = false
  }
}

async function handleRoleChange(member: DomainMember, newRole: 'admin' | 'member') {
  if (!domainId.value) return
  try {
    await updateDomainMemberRole(domainId.value, member.user_id, newRole)
    ElMessage.success('成员角色已更新')
    await fetchMembers()
  } catch {
    await fetchMembers()
  }
}

async function handleRemoveMember(member: DomainMember) {
  if (!domainId.value) return
  try {
    await ElMessageBox.confirm(
      `确认将成员「${member.nickname || member.username}」移出当前域吗？`,
      '移除成员确认',
      {
        confirmButtonText: '确认移除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await removeDomainMember(domainId.value, member.user_id)
    ElMessage.success('成员已移除')
    await fetchMembers()
    domainStore.fetchDomains(true)
  } catch {
    // Cancelled or failed
  }
}

function getRoleTagType(role?: string) {
  if (role === 'owner') return 'danger'
  if (role === 'admin') return 'warning'
  return 'info'
}

watch(domainId, () => {
  fetchMembers()
})

onMounted(() => {
  domainStore.fetchDomains()
  fetchMembers()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">
          域成员管理
          <span v-if="currentDomain" class="domain-tag">{{ currentDomain.domain_name }}</span>
        </h1>
        <p class="page-subtitle">查看当前域内的成员列表与角色权限</p>
      </div>
    </div>

    <el-alert
      v-if="currentDomain && !canManage"
      title="仅域创建者及其指定的管理员可管理域成员"
      type="info"
      show-icon
      :closable="false"
    />

    <div v-if="canManage" class="surface-card add-member-card">
      <div class="card-section-title">
        <el-icon><UserFilled /></el-icon>
        <span>添加域成员</span>
      </div>
      <el-form
        ref="formRef"
        :model="memberForm"
        :rules="rules"
        inline
        class="add-member-form"
      >
        <el-form-item prop="username" style="min-width: 260px">
          <el-input
            v-model="memberForm.username"
            placeholder="输入平台用户名（如 zhangsan）"
          />
        </el-form-item>
        <el-form-item prop="domain_role" style="width: 150px">
          <el-select v-model="memberForm.domain_role">
            <el-option label="普通成员" value="member" />
            <el-option v-if="isOwner" label="域管理员" value="admin" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="adding" @click="handleAddMember">
            添加成员
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <div class="surface-card table-card">
      <el-table v-loading="loading" :data="pagedMembers" stripe style="width: 100%">
        <el-table-column label="成员姓名 / 用户名" min-width="200">
          <template #default="{ row }">
            <div class="member-user">
              <span class="member-nickname">{{ row.nickname || row.username }}</span>
              <span class="member-username mono-text">@{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="邮箱" min-width="200">
          <template #default="{ row }">
            {{ row.email || '-' }}
          </template>
        </el-table-column>

        <el-table-column label="域角色" width="160">
          <template #default="{ row }">
            <el-select
              v-if="isOwner && row.domain_role !== 'owner'"
              :model-value="row.domain_role"
              size="small"
              style="width: 120px"
              @change="(val: 'admin' | 'member') => handleRoleChange(row, val)"
            >
              <el-option label="域管理员" value="admin" />
              <el-option label="普通成员" value="member" />
            </el-select>
            <el-tag v-else :type="getRoleTagType(row.domain_role)" size="small" effect="light">
              {{ getRoleLabel(row.domain_role) }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column label="加入时间" width="180">
          <template #default="{ row }">
            {{ formatDateTime(row.joined_at) }}
          </template>
        </el-table-column>

        <el-table-column v-if="canManage" label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="row.domain_role !== 'owner'"
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
          <el-empty description="当前域暂无成员记录" />
        </template>
      </el-table>

      <div v-if="members.length > 0" class="table-pagination">
        <el-pagination
          v-model:current-page="memberPage"
          :page-size="memberPageSize"
          :total="members.length"
          layout="total, prev, pager, next"
          background
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.domain-tag {
  font-size: 14px;
  font-weight: 500;
  color: var(--ascp-primary);
  background: var(--ascp-primary-soft);
  padding: 2px 10px;
  border-radius: 999px;
  margin-left: 8px;
  vertical-align: middle;
}

.add-member-card {
  padding: 18px 20px;
}

.card-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
  margin-bottom: 14px;
}

.add-member-form {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 8px;
}

.add-member-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.table-card {
  overflow: hidden;
}

.member-user {
  display: flex;
  flex-direction: column;
}

.member-nickname {
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.member-username {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.table-pagination {
  padding: 14px 18px;
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--ascp-border);
}
</style>
