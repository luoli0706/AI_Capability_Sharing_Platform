<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Lock, User } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
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

const highlights = [
  { title: 'MCP 服务统一接入', desc: '标准化 SSE / Streamable HTTP 服务注册与探测' },
  { title: 'Skills 技能沉淀', desc: 'Prompt、脚本与工作流资产跨团队安全复用' },
  { title: 'Agent 智能体编排', desc: '灵活绑定模型参数、工具链与领域知识库' },
  { title: '业务域隔离治理', desc: '基于业务域与 RBAC 的研发团队权限边界' }
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
          <p>请输入您的团队账号以进入业务域工作台</p>
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
