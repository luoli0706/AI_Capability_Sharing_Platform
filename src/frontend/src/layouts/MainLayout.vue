<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { storeToRefs } from 'pinia'
import {
  Box,
  Compass,
  Expand,
  Fold,
  OfficeBuilding,
  SwitchButton,
  User
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { getRoleLabel } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const { user } = storeToRefs(authStore)

const isCollapsed = ref(false)

const navItems = [
  { path: '/marketplace', title: '能力广场', icon: Compass },
  { path: '/assets', title: '资产工作台', icon: Box },
  { path: '/domains', title: '业务域管理', icon: OfficeBuilding },
  { path: '/profile', title: '个人中心', icon: User }
]

const activePath = computed(() => route.path)
const currentTitle = computed(() => (route.meta.title as string) || '工作台')

const displayName = computed(
  () => user.value?.nickname || user.value?.username || '用户'
)

const userInitial = computed(() => displayName.value.slice(0, 1).toUpperCase())

const primaryRoleLabel = computed(() => {
  const roles = user.value?.roles
  if (!roles || roles.length === 0) return '研发人员'
  return getRoleLabel(roles[0])
})

onMounted(() => {
  authStore.fetchProfile().catch(() => {
    // Handled by axios interceptor if 401
  })
})

async function handleCommand(command: string) {
  if (command === 'profile') {
    router.push('/profile')
    return
  }
  if (command === 'logout') {
    try {
      await ElMessageBox.confirm('确定要退出当前登录吗？', '退出确认', {
        confirmButtonText: '退出登录',
        cancelButtonText: '取消',
        type: 'warning'
      })
      await authStore.logout()
      ElMessage.success('已安全退出')
      router.replace('/login')
    } catch {
      // Cancelled
    }
  }
}
</script>

<template>
  <div class="layout-shell">
    <aside class="layout-sidebar" :class="{ 'is-collapsed': isCollapsed }">
      <div class="sidebar-brand" @click="router.push('/marketplace')">
        <div class="brand-logo">
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none">
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
        <div v-if="!isCollapsed" class="brand-text">
          <span class="brand-title">ASCP</span>
          <span class="brand-sub">AI 能力共享平台</span>
        </div>
      </div>

      <nav class="sidebar-nav" aria-label="主导航">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="nav-item"
          :class="{ 'is-active': activePath === item.path }"
        >
          <el-icon class="nav-icon" :size="18">
            <component :is="item.icon" />
          </el-icon>
          <span v-if="!isCollapsed" class="nav-label">{{ item.title }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button
          type="button"
          class="collapse-btn"
          :title="isCollapsed ? '展开侧边栏' : '收起侧边栏'"
          @click="isCollapsed = !isCollapsed"
        >
          <el-icon :size="16">
            <Expand v-if="isCollapsed" />
            <Fold v-else />
          </el-icon>
          <span v-if="!isCollapsed">收起菜单</span>
        </button>
      </div>
    </aside>

    <div class="layout-main">
      <header class="layout-header">
        <div class="header-left">
          <el-breadcrumb separator="/">
            <el-breadcrumb-item :to="{ path: '/marketplace' }">ASCP 工作台</el-breadcrumb-item>
            <el-breadcrumb-item>{{ currentTitle }}</el-breadcrumb-item>
          </el-breadcrumb>
        </div>

        <div class="header-right">
          <el-dropdown trigger="click" @command="handleCommand">
            <div class="user-trigger" role="button" tabindex="0">
              <el-avatar :size="32" class="user-avatar">
                {{ userInitial }}
              </el-avatar>
              <div class="user-meta">
                <span class="user-name">{{ displayName }}</span>
                <span class="user-role">{{ primaryRoleLabel }}</span>
              </div>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <div class="dropdown-user-header">
                  <div class="dropdown-name">{{ displayName }}</div>
                  <div class="dropdown-username">@{{ user?.username }}</div>
                  <div v-if="user?.roles?.length" class="dropdown-roles">
                    <el-tag
                      v-for="role in user.roles"
                      :key="role"
                      size="small"
                      type="primary"
                      effect="light"
                    >
                      {{ getRoleLabel(role) }}
                    </el-tag>
                  </div>
                </div>
                <el-dropdown-item divided command="profile">
                  <el-icon><User /></el-icon>
                  <span>个人中心</span>
                </el-dropdown-item>
                <el-dropdown-item command="logout">
                  <el-icon><SwitchButton /></el-icon>
                  <span>退出登录</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="layout-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.layout-shell {
  display: flex;
  height: 100vh;
  width: 100%;
  overflow: hidden;
  background-color: var(--ascp-bg-canvas);
}

.layout-sidebar {
  width: 232px;
  flex-shrink: 0;
  background-color: var(--ascp-sidebar-bg);
  border-right: 1px solid var(--ascp-sidebar-border);
  display: flex;
  flex-direction: column;
  transition: width 0.2s ease;
  user-select: none;
}

.layout-sidebar.is-collapsed {
  width: 68px;
}

.sidebar-brand {
  height: 60px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 18px;
  border-bottom: 1px solid var(--ascp-sidebar-border);
  cursor: pointer;
}

.brand-logo {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  background: linear-gradient(135deg, #4f46e5 0%, #7c3aed 100%);
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  box-shadow: 0 4px 12px rgba(79, 70, 229, 0.35);
}

.brand-text {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  white-space: nowrap;
}

.brand-title {
  font-size: 16px;
  font-weight: 700;
  color: #f8fafc;
  letter-spacing: 0.04em;
  line-height: 1.2;
}

.brand-sub {
  font-size: 11px;
  color: #94a3b8;
  margin-top: 2px;
}

.sidebar-nav {
  flex: 1;
  padding: 14px 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
  overflow-y: auto;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 42px;
  padding: 0 12px;
  border-radius: 8px;
  color: var(--ascp-sidebar-text);
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.layout-sidebar.is-collapsed .nav-item {
  justify-content: center;
  padding: 0;
}

.nav-item:hover {
  color: #e2e8f0;
  background-color: rgba(255, 255, 255, 0.06);
}

.nav-item.is-active {
  color: var(--ascp-sidebar-text-active);
  background: linear-gradient(90deg, rgba(79, 70, 229, 0.9) 0%, rgba(99, 102, 241, 0.85) 100%);
  box-shadow: 0 2px 8px rgba(79, 70, 229, 0.3);
}

.nav-icon {
  flex-shrink: 0;
}

.sidebar-footer {
  padding: 12px 10px;
  border-top: 1px solid var(--ascp-sidebar-border);
}

.collapse-btn {
  width: 100%;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #94a3b8;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 0.15s ease, color 0.15s ease;
}

.collapse-btn:hover {
  background-color: rgba(255, 255, 255, 0.06);
  color: #f8fafc;
}

.layout-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.layout-header {
  height: 60px;
  flex-shrink: 0;
  background-color: var(--ascp-bg-surface);
  border-bottom: 1px solid var(--ascp-border);
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.user-trigger {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 8px;
  border-radius: 8px;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.user-trigger:hover {
  background-color: #f1f5f9;
}

.user-avatar {
  background: linear-gradient(135deg, #4f46e5 0%, #6366f1 100%);
  color: #ffffff;
  font-weight: 600;
  font-size: 13px;
}

.user-meta {
  display: flex;
  flex-direction: column;
  line-height: 1.25;
}

.user-name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.user-role {
  font-size: 11.5px;
  color: var(--ascp-text-muted);
}

.dropdown-user-header {
  padding: 8px 14px;
  min-width: 180px;
}

.dropdown-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--ascp-text-primary);
}

.dropdown-username {
  font-size: 12px;
  color: var(--ascp-text-muted);
  margin-top: 2px;
}

.dropdown-roles {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.layout-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}
</style>
