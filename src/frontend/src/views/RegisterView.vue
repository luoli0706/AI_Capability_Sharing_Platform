<script setup lang="ts">
import { onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Key, Lock, Message, Postcard, User } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { register, sendRegisterEmailCode } from '@/api/auth'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const sendingCode = ref(false)
const countdown = ref(0)
const submitError = ref('')
let countdownTimer: ReturnType<typeof setInterval> | null = null

const form = reactive({
  username: '',
  nickname: '',
  email: '',
  email_code: '',
  password: '',
  confirmPassword: ''
})

function extractErrorMessage(err: unknown, fallback: string): string {
  if (err && typeof err === 'object') {
    const maybeAxios = err as { response?: { data?: { message?: string } }; message?: string }
    if (maybeAxios.response?.data?.message) return maybeAxios.response.data.message
    if (maybeAxios.message) return maybeAxios.message
  }
  return fallback
}

function startCountdown() {
  if (countdownTimer) clearInterval(countdownTimer)
  countdown.value = 60
  countdownTimer = setInterval(() => {
    if (countdown.value <= 1) {
      countdown.value = 0
      if (countdownTimer) {
        clearInterval(countdownTimer)
        countdownTimer = null
      }
      return
    }
    countdown.value -= 1
  }, 1000)
}

onUnmounted(() => {
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
})

const validateEmail = (_rule: unknown, value: string, callback: (err?: Error) => void) => {
  const trimmed = (value || '').trim()
  if (!trimmed) {
    callback(new Error('请输入邮箱地址'))
    return
  }
  const emailRe = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  if (!emailRe.test(trimmed)) {
    callback(new Error('请输入有效的邮箱格式'))
    return
  }
  callback()
}

const validateConfirmPassword = (_rule: unknown, value: string, callback: (err?: Error) => void) => {
  if (!value) {
    callback(new Error('请再次输入密码'))
    return
  }
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
    return
  }
  callback()
}

const rules: FormRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 32, message: '用户名长度需在 3 到 32 个字符之间', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]+$/, message: '仅支持字母、数字、下划线和连字符', trigger: 'blur' }
  ],
  nickname: [{ max: 32, message: '昵称不超过 32 个字符', trigger: 'blur' }],
  email: [{ required: true, validator: validateEmail, trigger: 'blur' }],
  email_code: [
    { required: true, message: '请输入 6 位邮箱验证码', trigger: 'blur' },
    { pattern: /^\d{6}$/, message: '验证码为 6 位数字', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, max: 64, message: '密码长度需在 8 到 64 个字符之间', trigger: 'blur' },
    { pattern: /^(?=.*[A-Za-z])(?=.*\d).+$/, message: '密码必须同时包含字母和数字', trigger: 'blur' }
  ],
  confirmPassword: [{ validator: validateConfirmPassword, trigger: 'blur' }]
}

async function handleSendCode() {
  if (!formRef.value || sendingCode.value || countdown.value > 0) return
  submitError.value = ''
  const emailValid = await formRef.value
    .validateField('email')
    .then(() => true)
    .catch(() => false)
  if (!emailValid) return

  sendingCode.value = true
  try {
    await sendRegisterEmailCode(form.email.trim())
    ElMessage.success('验证码已发送至您的邮箱，请注意查收')
    startCountdown()
  } catch (err: unknown) {
    submitError.value = extractErrorMessage(err, '发送验证码失败，请稍后重试')
  } finally {
    sendingCode.value = false
  }
}

async function handleSubmit() {
  if (!formRef.value) return
  submitError.value = ''
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  try {
    await register({
      username: form.username.trim(),
      nickname: form.nickname.trim() || form.username.trim(),
      email: form.email.trim(),
      email_code: form.email_code.trim(),
      password: form.password
    })
    ElMessage.success('注册成功，请登录')
    await router.push('/login')
  } catch (err: unknown) {
    submitError.value = extractErrorMessage(err, '注册失败，请检查填写信息')
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

        <h1 class="showcase-title">加入 AI 能力共享协作网络</h1>
        <p class="showcase-desc">
          注册账号后即可进入域空间，浏览团队内能力广场、参与域协作，并创建发布您的 MCP 服务、Skills 与 Agent 资产。
        </p>
      </div>
    </section>

    <section class="auth-form-panel">
      <div class="auth-card">
        <div class="auth-card-header">
          <h2>创建 ASCP 账号</h2>
          <p>填写以下信息完成开发者账号注册</p>
        </div>

        <el-alert
          v-if="submitError"
          :title="submitError"
          type="error"
          show-icon
          closable
          style="margin-bottom: 16px"
          @close="submitError = ''"
        />

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
              placeholder="例如：zhangsan"
              :prefix-icon="User"
              autocomplete="username"
            />
          </el-form-item>

          <el-form-item label="显示昵称" prop="nickname">
            <el-input
              v-model="form.nickname"
              placeholder="选填，默认同用户名"
              :prefix-icon="Postcard"
            />
          </el-form-item>

          <el-form-item label="邮箱" prop="email">
            <el-input
              v-model="form.email"
              placeholder="例如 name@example.com（不支持临时邮箱）"
              :prefix-icon="Message"
              autocomplete="email"
            />
            <div class="field-hint">请使用常用个人或企业邮箱，暂不支持临时邮箱</div>
          </el-form-item>

          <el-form-item label="邮箱验证码" prop="email_code">
            <div class="code-row">
              <el-input
                v-model="form.email_code"
                maxlength="6"
                placeholder="请输入 6 位验证码"
                :prefix-icon="Key"
                autocomplete="one-time-code"
              />
              <el-button
                class="send-code-btn"
                :loading="sendingCode"
                :disabled="sendingCode || countdown > 0"
                @click="handleSendCode"
              >
                {{ countdown > 0 ? `${countdown}s 后重试` : '发送验证码' }}
              </el-button>
            </div>
          </el-form-item>

          <el-form-item label="密码" prop="password">
            <el-input
              v-model="form.password"
              type="password"
              placeholder="至少 8 位，需同时包含字母和数字"
              :prefix-icon="Lock"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>

          <el-form-item label="确认密码" prop="confirmPassword">
            <el-input
              v-model="form.confirmPassword"
              type="password"
              placeholder="请再次输入密码"
              :prefix-icon="Lock"
              show-password
              autocomplete="new-password"
            />
          </el-form-item>

          <el-button
            type="primary"
            native-type="submit"
            :loading="loading"
            class="submit-btn"
          >
            立即注册
          </el-button>
        </el-form>

        <div class="auth-footer">
          已有账号？
          <router-link to="/login" class="auth-link">返回登录</router-link>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  background-color: var(--ascp-bg-canvas);
}

.auth-showcase {
  flex: 1.05;
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
  max-width: 500px;
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
}

.showcase-desc {
  margin: 0;
  font-size: 15px;
  line-height: 1.7;
  color: #cbd5e1;
}

.auth-form-panel {
  flex: 0.95;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px;
}

.auth-card {
  width: 100%;
  max-width: 440px;
  background: #ffffff;
  border: 1px solid var(--ascp-border);
  border-radius: 16px;
  padding: 36px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.06);
}

.auth-card-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--ascp-text-primary);
}

.auth-card-header p {
  margin: 8px 0 24px;
  font-size: 14px;
  color: var(--ascp-text-muted);
}

.field-hint {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.4;
  color: var(--ascp-text-muted);
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
  margin-top: 20px;
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
