<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { getMyAssets, updatePassword, updateProfile } from '@/api/auth'
import AssetDetailDrawer from '@/components/AssetDetailDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import type { AssetItem } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  getRoleLabel,
  normalizeList
} from '@/utils/format'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const activeTab = ref('basic')

const profileFormRef = ref<FormInstance>()
const savingProfile = ref(false)
const profileForm = reactive({
  nickname: '',
  email: ''
})

const profileRules: FormRules = {
  nickname: [{ required: true, message: '请输入显示昵称', trigger: 'blur' }],
  email: [
    { required: true, message: '请输入邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱格式', trigger: 'blur' }
  ]
}

const pwdFormRef = ref<FormInstance>()
const savingPwd = ref(false)
const pwdForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

const validateConfirmPwd = (_rule: unknown, value: string, cb: (err?: Error) => void) => {
  if (!value) {
    cb(new Error('请再次输入新密码'))
    return
  }
  if (value !== pwdForm.new_password) {
    cb(new Error('两次输入的新密码不一致'))
    return
  }
  cb()
}

const pwdRules: FormRules = {
  old_password: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 64, message: '新密码长度需在 6 到 64 个字符之间', trigger: 'blur' }
  ],
  confirm_password: [{ validator: validateConfirmPwd, trigger: 'blur' }]
}

const loadingAssets = ref(false)
const myAssets = ref<AssetItem[]>([])
const detailDrawerOpen = ref(false)
const selectedAssetId = ref<number | null>(null)

const displayName = computed(
  () => user.value?.nickname || user.value?.username || '用户'
)
const userInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())

function syncProfileForm() {
  profileForm.nickname = user.value?.nickname || ''
  profileForm.email = user.value?.email || ''
}

async function handleSaveProfile() {
  if (!profileFormRef.value) return
  const valid = await profileFormRef.value.validate().catch(() => false)
  if (!valid) return

  savingProfile.value = true
  try {
    const updated = await updateProfile({
      nickname: profileForm.nickname.trim(),
      email: profileForm.email.trim()
    })
    authStore.setUser(updated)
    ElMessage.success('个人信息已更新')
  } catch {
    // Handled by interceptor
  } finally {
    savingProfile.value = false
  }
}

async function handleUpdatePassword() {
  if (!pwdFormRef.value) return
  const valid = await pwdFormRef.value.validate().catch(() => false)
  if (!valid) return

  savingPwd.value = true
  try {
    await updatePassword({
      old_password: pwdForm.old_password,
      new_password: pwdForm.new_password
    })
    ElMessage.success('密码修改成功')
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm_password = ''
    pwdFormRef.value.clearValidate()
  } catch {
    // Handled by interceptor
  } finally {
    savingPwd.value = false
  }
}

async function fetchMyAssets() {
  loadingAssets.value = true
  try {
    const res = await getMyAssets({ page: 1, page_size: 50 })
    myAssets.value = normalizeList(res).items
  } catch {
    myAssets.value = []
  } finally {
    loadingAssets.value = false
  }
}

function openAssetDetail(asset: AssetItem) {
  selectedAssetId.value = asset.id
  detailDrawerOpen.value = true
}

onMounted(async () => {
  syncProfileForm()
  try {
    await authStore.fetchProfile()
    syncProfileForm()
  } catch {
    // Handled by interceptor
  }
  fetchMyAssets()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">个人中心</h1>
        <p class="page-subtitle">管理您的账号资料、登录密码及个人名下创建的 AI 能力资产</p>
      </div>
    </div>

    <div class="surface-card profile-banner">
      <el-avatar :size="64" class="profile-avatar">{{ userInitial }}</el-avatar>
      <div class="profile-info">
        <div class="profile-name-row">
          <h2 class="profile-name">{{ displayName }}</h2>
          <el-tag
            v-for="role in user?.roles || ['developer']"
            :key="role"
            size="small"
            type="primary"
            effect="light"
          >
            {{ getRoleLabel(role) }}
          </el-tag>
        </div>
        <div class="profile-sub">
          <span class="mono-text">@{{ user?.username }}</span>
          <span class="dot">·</span>
          <span>{{ user?.email || '未绑定邮箱' }}</span>
        </div>
      </div>
    </div>

    <div class="surface-card profile-tabs-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="基本信息与修改" name="basic">
          <el-form
            ref="profileFormRef"
            :model="profileForm"
            :rules="profileRules"
            label-position="top"
            class="narrow-form"
          >
            <el-form-item label="用户名">
              <el-input :model-value="user?.username || ''" disabled class="mono-text" />
            </el-form-item>
            <el-form-item label="显示昵称" prop="nickname">
              <el-input v-model="profileForm.nickname" placeholder="请输入显示昵称" />
            </el-form-item>
            <el-form-item label="企业邮箱" prop="email">
              <el-input v-model="profileForm.email" placeholder="name@company.com" />
            </el-form-item>
            <el-button type="primary" :loading="savingProfile" @click="handleSaveProfile">
              保存基本信息
            </el-button>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="修改密码" name="password">
          <el-form
            ref="pwdFormRef"
            :model="pwdForm"
            :rules="pwdRules"
            label-position="top"
            class="narrow-form"
          >
            <el-form-item label="原密码" prop="old_password">
              <el-input
                v-model="pwdForm.old_password"
                type="password"
                show-password
                placeholder="请输入当前密码"
              />
            </el-form-item>
            <el-form-item label="新密码" prop="new_password">
              <el-input
                v-model="pwdForm.new_password"
                type="password"
                show-password
                placeholder="至少 6 位字符"
              />
            </el-form-item>
            <el-form-item label="确认新密码" prop="confirm_password">
              <el-input
                v-model="pwdForm.confirm_password"
                type="password"
                show-password
                placeholder="请再次输入新密码"
              />
            </el-form-item>
            <el-button type="primary" :loading="savingPwd" @click="handleUpdatePassword">
              更新密码
            </el-button>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="我的资产" name="assets">
          <el-table v-loading="loadingAssets" :data="myAssets" stripe style="width: 100%">
            <el-table-column label="资产名称 / 编码" min-width="220">
              <template #default="{ row }">
                <div class="asset-cell">
                  <span class="asset-link" @click="openAssetDetail(row)">{{ row.name }}</span>
                  <span class="asset-code mono-text">{{ row.asset_code }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="类型" width="115">
              <template #default="{ row }">
                <el-tag :type="getAssetTypeMeta(row.asset_type).tagType" size="small" effect="light">
                  {{ getAssetTypeMeta(row.asset_type).shortLabel }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column label="版本" width="95">
              <template #default="{ row }">
                <span class="mono-text">{{ row.current_version || 'v0.1.0' }}</span>
              </template>
            </el-table-column>

            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="getAssetStatusMeta(row.status).tagType" size="small" effect="plain">
                  {{ getAssetStatusMeta(row.status).label }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column label="调用量" width="95" align="right">
              <template #default="{ row }">
                <span class="tabular-nums">{{ row.invoke_count ?? 0 }}</span>
              </template>
            </el-table-column>

            <el-table-column label="更新时间" width="160">
              <template #default="{ row }">
                {{ formatDateTime(row.updated_at) }}
              </template>
            </el-table-column>

            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" size="small" @click="openAssetDetail(row)">
                  查看
                </el-button>
              </template>
            </el-table-column>

            <template #empty>
              <el-empty description="您还没有创建任何 AI 资产" />
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </div>

    <AssetDetailDrawer v-model="detailDrawerOpen" :asset-id="selectedAssetId" />
  </div>
</template>

<style scoped>
.profile-banner {
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 20px;
}

.profile-avatar {
  background: linear-gradient(135deg, #4f46e5 0%, #6366f1 100%);
  color: #ffffff;
  font-size: 24px;
  font-weight: 700;
  flex-shrink: 0;
}

.profile-name-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.profile-name {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.profile-sub {
  margin-top: 6px;
  font-size: 13.5px;
  color: var(--ascp-text-muted);
  display: flex;
  align-items: center;
  gap: 8px;
}

.profile-tabs-card {
  padding: 16px 24px 24px;
}

.narrow-form {
  max-width: 440px;
  padding-top: 8px;
}

.asset-cell {
  display: flex;
  flex-direction: column;
}

.asset-link {
  font-weight: 600;
  color: var(--ascp-text-primary);
  cursor: pointer;
}

.asset-link:hover {
  color: var(--ascp-primary);
}

.asset-code {
  font-size: 12px;
  color: var(--ascp-text-muted);
}
</style>
