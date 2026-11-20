import axios, { type AxiosRequestConfig } from 'axios'
import { ElMessage } from 'element-plus'
import type { ApiResponse } from '@/types/api'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000
})

request.interceptors.request.use((config) => {
  const token = localStorage.getItem('ascp_token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

let isRedirectingToLogin = false

request.interceptors.response.use(
  (response) => {
    const res = response.data as ApiResponse
    if (res && typeof res.code === 'number' && res.code !== 0) {
      ElMessage.error(res.message || '请求失败')
      return Promise.reject(new Error(res.message || '请求失败'))
    }
    return response
  },
  async (error) => {
    const status = error.response?.status
    if (status === 401) {
      localStorage.removeItem('ascp_token')
      localStorage.removeItem('ascp_user')
      try {
        const { useAuthStore } = await import('@/stores/auth')
        const authStore = useAuthStore()
        authStore.clearAuth()
      } catch {
        // Store may not be initialized yet
      }
      if (!isRedirectingToLogin) {
        isRedirectingToLogin = true
        try {
          const { default: router } = await import('@/router')
          const currentPath = router.currentRoute.value.path
          if (currentPath !== '/login' && currentPath !== '/register') {
            await router.replace({
              path: '/login',
              query: { redirect: router.currentRoute.value.fullPath }
            })
          }
        } finally {
          isRedirectingToLogin = false
        }
      }
    }
    const msg = error.response?.data?.message || error.message || '网络异常'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)

export async function apiGet<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const res = await request.get<ApiResponse<T>>(url, config)
  return res.data.data
}

export async function apiPost<T, D = unknown>(
  url: string,
  data?: D,
  config?: AxiosRequestConfig
): Promise<T> {
  const res = await request.post<ApiResponse<T>>(url, data, config)
  return res.data.data
}

export async function apiPut<T, D = unknown>(
  url: string,
  data?: D,
  config?: AxiosRequestConfig
): Promise<T> {
  const res = await request.put<ApiResponse<T>>(url, data, config)
  return res.data.data
}

export async function apiDelete<T>(url: string, config?: AxiosRequestConfig): Promise<T> {
  const res = await request.delete<ApiResponse<T>>(url, config)
  return res.data.data
}

export default request
