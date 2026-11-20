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
    redirect: '/marketplace',
    meta: { requiresAuth: true },
    children: [
      {
        path: 'marketplace',
        name: 'marketplace',
        component: () => import('@/views/MarketplaceView.vue'),
        meta: { title: '能力广场', requiresAuth: true }
      },
      {
        path: 'assets',
        name: 'assets',
        component: () => import('@/views/AssetsView.vue'),
        meta: { title: '资产工作台', requiresAuth: true }
      },
      {
        path: 'domains',
        name: 'domains',
        component: () => import('@/views/DomainsView.vue'),
        meta: { title: '业务域管理', requiresAuth: true }
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
    redirect: '/marketplace'
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
    return { path: '/marketplace' }
  }

  return true
})

export default router
