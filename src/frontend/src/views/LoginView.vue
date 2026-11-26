<script setup lang="ts">
import { onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Key, Lock, User } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { resetPasswordByEmail, sendResetPasswordCode } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()

const formRef = ref<FormInstance>()
const loading = ref(false)

const form = reactive({
  username: '',
  password: ''
})

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const resetDialogVisible = ref(false)
const resetFormRef = ref<FormInstance>()
const resetLoading = ref(false)
const sendingResetCode = ref(false)
const resetCountdown = ref(0)
const resetError = ref('')
const resetSuccessHint = ref('')
let resetCountdownTimer: ReturnType<typeof setInterval> | null = null

const resetForm = reactive({
  account: '',
  email_code: ''
})

const resetRules: FormRules = {
  account: [{ required: true, message: '请输入用户名或已绑定邮箱', trigger: 'blur' }],
  email_code: [
    { required: true, message: '请输入 6 位数字验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' }
  ]
}

function extractErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === 'object') {
    const maybeAxios = err as { response?: { data?: { message?: string } }; message?: string }
    if (maybeAxios.response?.data?.message) return maybeAxios.response.data.message
    if (maybeAxios.message) return maybeAxios.message
  }
  return fallback
}

function clearResetTimer() {
  if (resetCountdownTimer) {
    clearInterval(resetCountdownTimer)
    resetCountdownTimer = null
  }
  resetCountdown.value = 0
}

function startResetCountdown() {
  clearResetTimer()
  resetCountdown.value = 60
  resetCountdownTimer = setInterval(() => {
    if (resetCountdown.value <= 1) {
      clearResetTimer()
      return
    }
    resetCountdown.value -= 1
  }, 1000)
}

onUnmounted(() => {
  clearResetTimer()
})

function openResetDialog() {
  resetError.value = ''
  resetSuccessHint.value = ''
  resetForm.account = form.username.trim()
  resetForm.email_code = ''
  resetDialogVisible.value = true
  resetFormRef.value?.clearValidate()
}

function handleResetDialogClose() {
  clearResetTimer()
  resetError.value = ''
  resetSuccessHint.value = ''
  resetForm.email_code = ''
  resetFormRef.value?.clearValidate()
}

async function handleSendResetCode() {
  if (!resetFormRef.value || sendingResetCode.value || resetCountdown.value > 0) return
  resetError.value = ''
  resetSuccessHint.value = ''
  const accountValid = await resetFormRef.value
    .validateField('account')
    .then(() => true)
    .catch(() => false)
  if (!accountValid) return

  sendingResetCode.value = true
  try {
    const res = await sendResetPasswordCode(resetForm.account.trim())
    const maskedEmail = res?.masked_email
    const hint = maskedEmail
      ? `验证码已发送至绑定邮箱 ${maskedEmail}，请注意查收`
      : '验证码已发送至您的绑定邮箱，请注意查收'
    resetSuccessHint.value = hint
    ElMessage.success(hint)
    startResetCountdown()
  } catch (err: unknown) {
    resetError.value = extractErrorMessage(err, '发送验证码失败，请稍后重试')
  } finally {
    sendingResetCode.value = false
  }
}

async function handleResetSubmit() {
  if (!resetFormRef.value) return
  resetError.value = ''
  const valid = await resetFormRef.value.validate().catch(() => false)
  if (!valid) return

  const account = resetForm.account.trim()
  resetLoading.value = true
  try {
    await resetPasswordByEmail({
      account,
      email_code: resetForm.email_code.trim()
    })
    resetDialogVisible.value = false
    if (!account.includes('@')) {
      form.username = account
    }
    form.password = ''
    await ElMessageBox.alert(
      '密码已重置为 000000（六个零），并已向您的绑定邮箱发送提醒，请立即使用 000000 登录并前往个人中心修改密码！',
      '密码重置成功',
      {
        type: 'success',
        confirmButtonText: '知道了'
      }
    )
  } catch (err: unknown) {
    resetError.value = extractErrorMessage(err, '重置密码失败，请检查输入信息')
  } finally {
    resetLoading.value = false
  }
}

const highlights = [
  { title: 'MCP 服务统一接入', desc: '标准化 SSE / Streamable HTTP 服务注册与探测' },
  { title: 'Skills 技能沉淀', desc: 'Prompt、脚本与工作流资产跨团队安全复用' },
  { title: 'Agent 智能体编排', desc: '灵活绑定模型参数、工具链与领域知识库' },
  { title: '域隔离治理', desc: '基于域与 RBAC 的研发团队权限边界' }
]

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    await authStore.login({
      username: form.username.trim(),
      password: form.password
    })
    ElMessage.success('登录成功')
    const redirect = (route.query.redirect as string) || '/domains'
    await router.replace(redirect)
  } catch {
    // Error message is handled by request interceptor
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="auth-page">
    <section class="auth-showcase">
      <div class="showcase-inner">
        <div class="brand-badge">
          <div class="brand-icon">
            <svg viewBox="0 0 24 24" width="22" height="22" fill="none">
              <path
                d="M12 2L20 6.5V17.5L12 22L4 17.5V6.5L12 2Z"
                stroke="currentColor"
                stroke-width="2"
                stroke-linejoin="round"
              />
              <path
                d="M12 8L16 10.5V15.5L12 18L8 15.5V10.5L12 8Z"
                fill="currentColor"
                fill-opacity="0.9"
              />
            </svg>
          </div>
          <div>
            <div class="brand-name">ASCP</div>
            <div class="brand-tagline">AI Capability Sharing Platform</div>
          </div>
        </div>

        <h1 class="showcase-title">研发团队 AI 能力共享与治理平台</h1>
        <p class="showcase-desc">
          统一纳管 MCP Server、Skills 技能、Agent 实例与私有知识库，打破跨团队数据孤岛，实现高可信 AI 资产复用。
        </p>

        <div class="highlight-grid">
          <div v-for="item in highlights" :key="item.title" class="highlight-card">
            <div class="highlight-title">{{ item.title }}</div>
            <div class="highlight-desc">{{ item.desc }}</div>
          </div>
        </div>
      </div>
    </section>

    <section class="auth-form-panel">
      <div class="auth-card">
        <div class="auth-card-header">
          <h2>欢迎登录 ASCP</h2>
          <p>请输入您的团队账号以进入域工作台</p>
        </div>

        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          label-position="top"
          size="large"
          @submit.prevent="handleSubmit"
        >
          <el-form-item label="用户名" prop="username">
            <el-input
              v-model="form.username"
              placeholder="请输入用户名"
              :prefix-icon="User"
              autocomplete="username"
            />
          </el-form-item>

          <el-form-item label="密码" prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="请输入密码"
              :prefix-icon="Lock"
              show-password
              autocomplete="current-password"
            />
          </el-form-item>

          <div class="login-actions-row">
            <el-button link type="primary" class="forgot-link-btn" @click="openResetDialog">
              忘记密码？
            </el-button>
          </div>

          <el-button
            type="primary"
            native-type="submit"
            :loading="loading"
            class="submit-btn"
          >
            登录工作台
          </el-button>
        </el-form>

        <div class="auth-footer">
          还没有账号？
          <router-link to="/register" class="auth-link">立即注册</router-link>
        </div>
      </div>
    </section>

    <el-dialog
      v-model="resetDialogVisible"
      title="找回密码"
      width="440px"
      destroy-on-close
      @close="handleResetDialogClose"
    >
      <p class="reset-desc">
        输入用户名或已绑定邮箱，验证绑定邮箱后将把密码重置为 <code>000000</code>（六个零）；若账号尚未绑定邮箱，请联系网站管理员解决。
      </p>

      <el-alert
        v-if="resetError"
        :title="resetError"
        type="error"
        show-icon
        closable
        style="margin-bottom: 16px"
        @close="resetError = ''"
      />

      <el-alert
        v-if="resetSuccessHint"
        :title="resetSuccessHint"
        type="success"
        show-icon
        closable
        style="margin-bottom: 16px"
        @close="resetSuccessHint = ''"
      />

      <el-form
        ref="resetFormRef"
        :model="resetForm"
        :rules="resetRules"
        label-position="top"
        size="large"
        @submit.prevent="handleResetSubmit"
      >
        <el-form-item label="用户名或绑定邮箱" prop="account">
          <el-input
            v-model="resetForm.account"
            placeholder="请输入用户名或已绑定邮箱"
            :prefix-icon="User"
          />
        </el-form-item>

        <el-form-item label="邮箱验证码" prop="email_code">
          <div class="code-row">
            <el-input
              v-model="resetForm.email_code"
              maxlength="6"
              placeholder="请输入 6 位数字验证码"
              :prefix-icon="Key"
              autocomplete="one-time-code"
            />
            <el-button
              class="send-code-btn"
              :loading="sendingResetCode"
              :disabled="sendingResetCode || resetCountdown > 0"
              @click="handleSendResetCode"
            >
              {{ resetCountdown > 0 ? `${resetCountdown}s 后重试` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetLoading" @click="handleResetSubmit">
          确认重置密码
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  background-color: var(--ascp-bg-canvas);
}

.auth-showcase {
  flex: 1.1;
  background: radial-gradient(circle at top right, #312e81 0%, #0f172a 55%, #020617 100%);
  color: #f8fafc;
  padding: 56px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
}

.auth-showcase::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(148, 163, 184, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(148, 163, 184, 0.08) 1px, transparent 1px);
  background-size: 36px 36px;
  pointer-events: none;
}

.showcase-inner {
  max-width: 540px;
  position: relative;
  z-index: 1;
}

.brand-badge {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 36px;
}

.brand-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 20px rgba(79, 70, 229, 0.4);
}

.brand-name {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.brand-tagline {
  font-size: 12px;
  color: #94a3b8;
}

.showcase-title {
  margin: 0 0 16px;
  font-size: 32px;
  font-weight: 700;
  line-height: 1.25;
  letter-spacing: -0.02em;
}

.showcase-desc {
  margin: 0 0 36px;
  font-size: 15px;
  line-height: 1.7;
  color: #cbd5e1;
}

.highlight-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 14px;
}

.highlight-card {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 16px;
  backdrop-filter: blur(8px);
}

.highlight-title {
  font-size: 14px;
  font-weight: 600;
  color: #f8fafc;
  margin-bottom: 6px;
}

.highlight-desc {
  font-size: 12.5px;
  color: #94a3b8;
  line-height: 1.5;
}

.auth-form-panel {
  flex: 0.9;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
}

.auth-card {
  width: 100%;
  max-width: 420px;
  background: #ffffff;
  border: 1px solid var(--ascp-border);
  border-radius: 16px;
  padding: 40px 36px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
}

.auth-card-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--ascp-text-primary);
}

.auth-card-header p {
  margin: 8px 0 28px;
  font-size: 14px;
  color: var(--ascp-text-muted);
}

.login-actions-row {
  display: flex;
  justify-content: flex-end;
  margin-top: -10px;
  margin-bottom: 10px;
}

.forgot-link-btn {
  padding: 0;
  font-size: 13.5px;
  font-weight: 500;
}

.reset-desc {
  margin: 0 0 16px;
  font-size: 13.5px;
  line-height: 1.6;
  color: var(--ascp-text-secondary);
}

.reset-desc code {
  padding: 1px 6px;
  border-radius: 4px;
  background-color: var(--ascp-bg-subtle, #f1f5f9);
  color: var(--ascp-primary);
  font-weight: 600;
}

.code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.send-code-btn {
  flex-shrink: 0;
  min-width: 116px;
}

.submit-btn {
  width: 100%;
  margin-top: 8px;
  height: 44px;
  font-weight: 600;
}

.auth-footer {
  margin-top: 24px;
  text-align: center;
  font-size: 13.5px;
  color: var(--ascp-text-muted);
}

.auth-link {
  color: var(--ascp-primary);
  font-weight: 600;
  text-decoration: none;
  margin-left: 4px;
}

.auth-link:hover {
  text-decoration: underline;
}

@media (max-width: 960px) {
  .auth-showcase {
    display: none;
  }
}
</style>
