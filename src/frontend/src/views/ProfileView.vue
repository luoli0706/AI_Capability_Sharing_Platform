<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { storeToRefs } from 'pinia'
import { CopyDocument, Key, Plus, Refresh, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  createApiKey,
  getApiKeys,
  getMyAssets,
  revokeApiKey,
  updatePassword,
  updateProfile
} from '@/api/auth'
import { getInvokeLogs } from '@/api/invoke'
import AssetDetailDrawer from '@/components/AssetDetailDrawer.vue'
import { useAuthStore } from '@/stores/auth'
import type { AssetItem, InvokeLogItem, UserApiKey } from '@/types/api'
import {
  formatDateTime,
  getAssetStatusMeta,
  getAssetTypeMeta,
  normalizeList
} from '@/utils/format'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const activeTab = ref('basic')

// Profile form
const profileFormRef = ref<FormInstance>()
const savingProfile = ref(false)
const avatarInputRef = ref<HTMLInputElement | null>(null)
const profileForm = reactive({
  nickname: '',
  email: '',
  bio: '',
  avatar: ''
})

const profileRules: FormRules = {
  nickname: [{ required: true, message: '请输入显示昵称', trigger: 'blur' }],
  email: [
    {
      validator: (_rule: unknown, value: string, cb: (err?: Error) => void) => {
        if (!value || !value.trim()) {
          cb()
          return
        }
        const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
        if (!emailRe.test(value.trim())) {
          cb(new Error('请输入有效的邮箱格式'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ]
}

// Password form
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

// My Assets (paginated)
const loadingAssets = ref(false)
const myAssets = ref<AssetItem[]>([])
const assetsTotal = ref(0)
const assetsPage = ref(1)
const assetsPageSize = ref(10)
const detailDrawerOpen = ref(false)
const selectedAssetId = ref<number | null>(null)

// API Keys (paginated)
const loadingKeys = ref(false)
const apiKeys = ref<UserApiKey[]>([])
const keysPage = ref(1)
const keysPageSize = ref(10)
const pagedApiKeys = computed(() => {
  const start = (keysPage.value - 1) * keysPageSize.value
  return apiKeys.value.slice(start, start + keysPageSize.value)
})
const createKeyModalOpen = ref(false)
const creatingKey = ref(false)
const keyFormRef = ref<FormInstance>()
const keyForm = reactive({
  key_name: '',
  expires_days: 90
})
const keyRules: FormRules = {
  key_name: [{ required: true, message: '请输入 API Key 名称', trigger: 'blur' }]
}
const createdPlainKey = ref('')
const plainKeyModalOpen = ref(false)

// Invoke Logs (paginated)
const loadingLogs = ref(false)
const invokeLogs = ref<InvokeLogItem[]>([])
const logsTotal = ref(0)
const logsPage = ref(1)
const logsPageSize = ref(10)

const displayName = computed(
  () => user.value?.nickname || user.value?.username || '用户'
)
const userInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())
const isSuperAdmin = computed(() => Boolean(user.value?.roles?.includes('super_admin')))

function syncProfileForm() {
  profileForm.nickname = user.value?.nickname || ''
  profileForm.email = user.value?.email || ''
  profileForm.bio = user.value?.bio || ''
  profileForm.avatar = user.value?.avatar || ''
}

function triggerAvatarUpload() {
  avatarInputRef.value?.click()
}

function resizeImageToDataUrl(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('读取图片失败'))
    reader.onload = () => {
      const img = new Image()
      img.onerror = () => reject(new Error('图片格式无效'))
      img.onload = () => {
        const maxSide = 256
        const scale = Math.min(1, maxSide / Math.max(img.width, img.height))
        const width = Math.max(1, Math.round(img.width * scale))
        const height = Math.max(1, Math.round(img.height * scale))
        const canvas = document.createElement('canvas')
        canvas.width = width
        canvas.height = height
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          reject(new Error('Canvas 初始化失败'))
          return
        }
        ctx.drawImage(img, 0, 0, width, height)
        let quality = 0.85
        let dataUrl = canvas.toDataURL('image/jpeg', quality)
        // Ensure <= 200KB
        while (dataUrl.length > 200 * 1024 && quality > 0.3) {
          quality -= 0.15
          dataUrl = canvas.toDataURL('image/jpeg', quality)
        }
        if (dataUrl.length > 200 * 1024) {
          reject(new Error('图片体积过大，请选择更小的图片'))
          return
        }
        resolve(dataUrl)
      }
      img.src = reader.result as string
    }
    reader.readAsDataURL(file)
  })
}

async function handleAvatarFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  try {
    const dataUrl = await resizeImageToDataUrl(file)
    profileForm.avatar = dataUrl
    ElMessage.success('头像已加载，点击「保存基本信息」生效')
  } catch (err: unknown) {
    ElMessage.error(err instanceof Error ? err.message : '头像处理失败')
  } finally {
    input.value = ''
  }
}

async function handleSaveProfile() {
  if (!profileFormRef.value) return
  const valid = await profileFormRef.value.validate().catch(() => false)
  if (!valid) return

  savingProfile.value = true
  try {
    const updated = await updateProfile({
      nickname: profileForm.nickname.trim(),
      email: profileForm.email.trim() || undefined,
      bio: profileForm.bio.trim(),
      avatar: profileForm.avatar.trim()
    })
    authStore.setUser(updated)
    syncProfileForm()
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
    const res = await getMyAssets({
      page: assetsPage.value,
      page_size: assetsPageSize.value
    })
    const normalized = normalizeList(res)
    myAssets.value = normalized.items
    assetsTotal.value = normalized.total
  } catch {
    myAssets.value = []
    assetsTotal.value = 0
  } finally {
    loadingAssets.value = false
  }
}

async function fetchApiKeys() {
  loadingKeys.value = true
  try {
    const res = await getApiKeys()
    apiKeys.value = normalizeList(res).items
    keysPage.value = 1
  } catch {
    apiKeys.value = []
  } finally {
    loadingKeys.value = false
  }
}

function openCreateKeyModal() {
  keyForm.key_name = ''
  keyForm.expires_days = 90
  keyFormRef.value?.clearValidate()
  createKeyModalOpen.value = true
}

async function handleCreateKey() {
  if (!keyFormRef.value) return
  const valid = await keyFormRef.value.validate().catch(() => false)
  if (!valid) return

  creatingKey.value = true
  try {
    const res = await createApiKey({
      key_name: keyForm.key_name.trim(),
      expires_days: keyForm.expires_days
    })
    createKeyModalOpen.value = false
    const plain = res.plain_key || res.key || ''
    if (plain) {
      createdPlainKey.value = plain
      plainKeyModalOpen.value = true
    } else {
      ElMessage.success('API Key 创建成功')
    }
    fetchApiKeys()
  } catch {
    // Handled by interceptor
  } finally {
    creatingKey.value = false
  }
}

async function copyPlainKey() {
  try {
    await navigator.clipboard.writeText(createdPlainKey.value)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.info('请手动选中并复制密钥')
  }
}

async function handleRevokeKey(item: UserApiKey) {
  try {
    await ElMessageBox.confirm(`确认吊销 API Key「${item.key_name}」吗？吊销后立即失效。`, '吊销确认', {
      confirmButtonText: '确认吊销',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await revokeApiKey(item.id)
    ElMessage.success('API Key 已吊销')
    fetchApiKeys()
  } catch {
    // Cancelled or failed
  }
}

async function fetchInvokeLogs() {
  loadingLogs.value = true
  try {
    const res = await getInvokeLogs({
      page: logsPage.value,
      page_size: logsPageSize.value
    })
    const normalized = normalizeList(res)
    invokeLogs.value = normalized.items
    logsTotal.value = normalized.total
  } catch {
    invokeLogs.value = []
    logsTotal.value = 0
  } finally {
    loadingLogs.value = false
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
  fetchApiKeys()
  fetchInvokeLogs()
})
</script>

<template>
  <div class="page-container">
    <div class="page-header">
      <div>
        <h1 class="page-title">个人中心</h1>
        <p class="page-subtitle">管理您的账号资料、登录密码、个人 API Key 密钥及能力调用审计记录</p>
      </div>
    </div>

    <div class="surface-card profile-banner">
      <el-avatar :size="72" :src="user?.avatar || undefined" class="profile-avatar">
        {{ userInitial }}
      </el-avatar>
      <div class="profile-info">
        <div class="profile-name-row">
          <h2 class="profile-name">{{ displayName }}</h2>
          <el-tag v-if="isSuperAdmin" size="small" type="primary" effect="light">
            超级管理员
          </el-tag>
        </div>
        <div class="profile-sub">
          <span class="mono-text">@{{ user?.username }}</span>
          <template v-if="user?.email">
            <span class="dot">·</span>
            <span>{{ user.email }}</span>
          </template>
        </div>
        <div v-if="user?.bio" class="profile-bio">{{ user.bio }}</div>
      </div>
    </div>

    <div class="surface-card profile-tabs-card">
      <el-tabs v-model="activeTab">
        <el-tab-pane label="基本信息与密码" name="basic">
          <div class="basic-grid">
            <el-form
              ref="profileFormRef"
              :model="profileForm"
              :rules="profileRules"
              label-position="top"
              class="narrow-form"
            >
              <h4 class="sub-heading">基本信息</h4>
              <el-form-item label="自定义头像">
                <div class="avatar-editor">
                  <el-avatar :size="72" :src="profileForm.avatar || undefined" class="profile-avatar">
                    {{ userInitial }}
                  </el-avatar>
                  <div class="avatar-controls">
                    <div class="avatar-btn-row">
                      <el-button size="small" :icon="Upload" @click="triggerAvatarUpload">
                        上传本地图片
                      </el-button>
                      <el-button
                        v-if="profileForm.avatar"
                        size="small"
                        link
                        type="info"
                        @click="profileForm.avatar = ''"
                      >
                        恢复默认
                      </el-button>
                    </div>
                    <el-input
                      v-model="profileForm.avatar"
                      size="small"
                      placeholder="或输入头像图片 URL"
                      class="mono-text"
                    />
                  </div>
                  <input
                    ref="avatarInputRef"
                    type="file"
                    accept="image/*"
                    style="display: none"
                    @change="handleAvatarFileChange"
                  />
                </div>
              </el-form-item>

              <el-form-item label="用户名">
                <el-input :model-value="user?.username || ''" disabled class="mono-text" />
              </el-form-item>
              <el-form-item label="显示昵称" prop="nickname">
                <el-input v-model="profileForm.nickname" placeholder="请输入显示昵称" />
              </el-form-item>
              <el-form-item label="邮箱（选填）" prop="email">
                <el-input v-model="profileForm.email" placeholder="选填，例如 name@example.com" />
              </el-form-item>
              <el-form-item label="个性签名">
                <el-input
                  v-model="profileForm.bio"
                  type="textarea"
                  :rows="2"
                  maxlength="120"
                  show-word-limit
                  placeholder="介绍一下自己或常用的 AI 能力方向..."
                />
              </el-form-item>
              <el-button type="primary" :loading="savingProfile" @click="handleSaveProfile">
                保存基本信息
              </el-button>
            </el-form>

            <el-form
              ref="pwdFormRef"
              :model="pwdForm"
              :rules="pwdRules"
              label-position="top"
              class="narrow-form"
            >
              <h4 class="sub-heading">修改密码</h4>
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
          </div>
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

          <div v-if="assetsTotal > 0" class="tab-pagination">
            <el-pagination
              v-model:current-page="assetsPage"
              :page-size="assetsPageSize"
              :total="assetsTotal"
              layout="total, prev, pager, next"
              background
              @current-change="fetchMyAssets"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="API Key 管理" name="apikeys">
          <div class="tab-toolbar">
            <span class="tab-hint">通过 <code class="mono-text">X-API-Key</code> 请求头在外部脚本或服务中调用已授权的 AI 能力</span>
            <el-button type="primary" :icon="Plus" size="small" @click="openCreateKeyModal">
              新建 API Key
            </el-button>
          </div>

          <el-table v-loading="loadingKeys" :data="pagedApiKeys" stripe style="width: 100%">
            <el-table-column label="名称" min-width="160">
              <template #default="{ row }">
                <span class="key-name"><el-icon><Key /></el-icon> {{ row.key_name }}</span>
              </template>
            </el-table-column>
            <el-table-column label="密钥前缀" width="160">
              <template #default="{ row }">
                <span class="mono-text">{{ row.key_prefix }}****</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small" effect="light">
                  {{ row.status === 1 ? '启用中' : '已吊销' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="过期时间" width="155">
              <template #default="{ row }">
                {{ row.expires_at ? formatDateTime(row.expires_at) : '永不过期' }}
              </template>
            </el-table-column>
            <el-table-column label="最近使用" width="155">
              <template #default="{ row }">
                {{ formatDateTime(row.last_used_at) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="100" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="row.status === 1"
                  link
                  type="danger"
                  size="small"
                  @click="handleRevokeKey(row)"
                >
                  吊销
                </el-button>
              </template>
            </el-table-column>

            <template #empty>
              <el-empty description="暂无 API Key，点击右上角「新建 API Key」生成" />
            </template>
          </el-table>

          <div v-if="apiKeys.length > 0" class="tab-pagination">
            <el-pagination
              v-model:current-page="keysPage"
              :page-size="keysPageSize"
              :total="apiKeys.length"
              layout="total, prev, pager, next"
              background
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="调用记录" name="logs">
          <div class="tab-toolbar">
            <span class="tab-hint">展示您通过控制台、JWT 或 API Key 发起的 AI 能力调用审计日志</span>
            <el-button :icon="Refresh" size="small" @click="fetchInvokeLogs">刷新</el-button>
          </div>

          <el-table v-loading="loadingLogs" :data="invokeLogs" stripe style="width: 100%">
            <el-table-column label="Trace ID" width="175">
              <template #default="{ row }">
                <span class="mono-text trace-id">{{ row.trace_id }}</span>
              </template>
            </el-table-column>
            <el-table-column label="调用资产" min-width="170">
              <template #default="{ row }">
                <div class="asset-cell">
                  <span>{{ row.asset_name || row.asset_code || `资产 #${row.asset_id}` }}</span>
                  <span v-if="row.asset_version" class="asset-code mono-text">{{ row.asset_version }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="鉴权方式" width="105">
              <template #default="{ row }">
                <el-tag size="small" type="info" effect="plain" class="mono-text">
                  {{ row.auth_mode }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态码" width="110">
              <template #default="{ row }">
                <el-tag
                  :type="row.is_success ? 'success' : 'danger'"
                  size="small"
                  effect="light"
                  class="tabular-nums"
                >
                  {{ row.status_code }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="95" align="right">
              <template #default="{ row }">
                <span class="tabular-nums">{{ row.latency_ms }} ms</span>
              </template>
            </el-table-column>
            <el-table-column label="调用时间" width="160">
              <template #default="{ row }">
                {{ formatDateTime(row.created_at) }}
              </template>
            </el-table-column>

            <template #empty>
              <el-empty description="暂无调用审计记录" />
            </template>
          </el-table>

          <div v-if="logsTotal > 0" class="tab-pagination">
            <el-pagination
              v-model:current-page="logsPage"
              :page-size="logsPageSize"
              :total="logsTotal"
              layout="total, prev, pager, next"
              background
              @current-change="fetchInvokeLogs"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <AssetDetailDrawer v-model="detailDrawerOpen" :asset-id="selectedAssetId" />

    <!-- Create API Key Modal -->
    <el-dialog v-model="createKeyModalOpen" title="新建 API Key" width="420px" destroy-on-close>
      <el-form ref="keyFormRef" :model="keyForm" :rules="keyRules" label-position="top">
        <el-form-item label="密钥名称" prop="key_name">
          <el-input v-model="keyForm.key_name" placeholder="例如：本地调试脚本 Key" />
        </el-form-item>
        <el-form-item label="有效期 (天)">
          <el-input-number v-model="keyForm.expires_days" :min="1" :max="3650" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createKeyModalOpen = false">取消</el-button>
        <el-button type="primary" :loading="creatingKey" @click="handleCreateKey">确认生成</el-button>
      </template>
    </el-dialog>

    <!-- One-time Plain Key Display Dialog -->
    <el-dialog
      v-model="plainKeyModalOpen"
      title="请妥善保存您的 API Key"
      width="480px"
      :close-on-click-modal="false"
    >
      <el-alert
        title="出于安全考虑，明文密钥仅在此处展示一次，关闭窗口后将无法再次查看。"
        type="warning"
        show-icon
        :closable="false"
        style="margin-bottom: 16px"
      />
      <div class="plain-key-box">
        <code class="mono-text">{{ createdPlainKey }}</code>
        <el-button type="primary" :icon="CopyDocument" size="small" @click="copyPlainKey">
          复制
        </el-button>
      </div>
      <template #footer>
        <el-button type="primary" @click="plainKeyModalOpen = false">我已保存</el-button>
      </template>
    </el-dialog>
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

.profile-bio {
  margin-top: 6px;
  font-size: 13.5px;
  color: var(--ascp-text-secondary);
}

.profile-tabs-card {
  padding: 16px 24px 24px;
}

.basic-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 290px), 1fr));
  gap: 32px;
  padding-top: 8px;
}

.sub-heading {
  margin: 0 0 16px;
  font-size: 15px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.narrow-form {
  max-width: 420px;
}

.avatar-editor {
  display: flex;
  align-items: center;
  gap: 16px;
  width: 100%;
}

.avatar-controls {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.avatar-btn-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tab-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.tab-hint {
  font-size: 13px;
  color: var(--ascp-text-muted);
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

.asset-code,
.trace-id {
  font-size: 12px;
  color: var(--ascp-text-muted);
}

.key-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.plain-key-box {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: #0f172a;
  color: #f8fafc;
  border-radius: 8px;
  word-break: break-all;
}

.tab-pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  .profile-tabs-card {
    padding: 12px 14px 18px;
  }

  .basic-grid {
    gap: 20px;
  }

  .avatar-editor {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
