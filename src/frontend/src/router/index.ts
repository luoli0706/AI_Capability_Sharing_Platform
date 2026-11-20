import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { title: '登录', guestOnly: true }
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('@/views/RegisterView.vue'),
    meta: { title: '注册', guestOnly: true }
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    redirect: '/domains',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'domains',
        name: 'domains',
        component: () => import('@/views/DomainsView.vue'),
        meta: { title: '域大厅', requiresAuth: true }
      },
      {
        path: 'organizations',
        name: 'organizations',
        component: () => import('@/views/OrganizationsView.vue'),
        meta: { title: '组织中心', requiresAuth: true }
      },
      {
        path: 'domains/:domainId/marketplace',
        name: 'domain-marketplace',
        component: () => import('@/views/MarketplaceView.vue'),
        meta: { title: '域内能力广场', requiresAuth: true, domainScoped: true }
      },
      {
        path: 'domains/:domainId/assets',
        name: 'domain-assets',
        component: () => import('@/views/AssetsView.vue'),
        meta: { title: '域内资产工作台', requiresAuth: true, domainScoped: true }
      },
      {
        path: 'domains/:domainId/knowledge-bases',
        name: 'domain-kbs',
        component: () => import('@/views/KnowledgeBasesView.vue'),
        meta: { title: '域内知识库', requiresAuth: true, domainScoped: true }
      },
      {
        path: 'domains/:domainId/members',
        name: 'domain-members',
        component: () => import('@/views/DomainMembersView.vue'),
        meta: { title: '域成员管理', requiresAuth: true, domainScoped: true }
      },
      {
        path: 'approvals',
        name: 'approvals',
        component: () => import('@/views/ApprovalsView.vue'),
        meta: { title: '审批中心', requiresAuth: true }
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('@/views/AuditLogsView.vue'),
        meta: { title: '日志中心', requiresAuth: true }
      },
      {
        path: 'profile',
        name: 'profile',
        component: () => import('@/views/ProfileView.vue'),
        meta: { title: '个人中心', requiresAuth: true }
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/domains'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to) => {
  const authStore = useAuthStore()
  const pageTitle = to.meta.title ? `${to.meta.title} - ASCP` : 'AI 能力共享平台 (ASCP)'
  document.title = pageTitle

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return {
      path: '/login',
      query: { redirect: to.fullPath }
    }
  }

  if (to.meta.guestOnly && authStore.isAuthenticated) {
    return { path: '/domains' }
  }

  return true
})

export default router
