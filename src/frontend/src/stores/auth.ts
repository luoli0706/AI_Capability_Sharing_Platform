import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import * as authApi from '@/api/auth'
import type { LoginPayload, UserInfo } from '@/types/api'

const TOKEN_KEY = 'ascp_token'
const USER_KEY = 'ascp_user'

function readStoredUser(): UserInfo | null {
  try {
    const raw = localStorage.getItem(USER_KEY)
    return raw ? (JSON.parse(raw) as UserInfo) : null
  } catch {
    return null
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem(TOKEN_KEY) || '')
  const user = ref<UserInfo | null>(readStoredUser())
  const loadingProfile = ref(false)

  const isAuthenticated = computed(() => Boolean(token.value))

  function setToken(newToken: string) {
    token.value = newToken
    if (newToken) {
      localStorage.setItem(TOKEN_KEY, newToken)
    } else {
      localStorage.removeItem(TOKEN_KEY)
    }
  }

  function setUser(newUser: UserInfo | null) {
    user.value = newUser
    if (newUser) {
      localStorage.setItem(USER_KEY, JSON.stringify(newUser))
    } else {
      localStorage.removeItem(USER_KEY)
    }
  }

  function clearAuth() {
    setToken('')
    setUser(null)
  }

  async function login(payload: LoginPayload) {
    const res = await authApi.login(payload)
    setToken(res.token)
    setUser(res.user)
    return res
  }

  async function fetchProfile() {
    if (!token.value) return null
    loadingProfile.value = true
    try {
      const profile = await authApi.getProfile()
      setUser(profile)
      return profile
    } finally {
      loadingProfile.value = false
    }
  }

  async function logout() {
    try {
      if (token.value) {
        await authApi.logout()
      }
    } catch {
      // Ignore network/401 errors during logout cleanup
    } finally {
      clearAuth()
    }
  }

  return {
    token,
    user,
    loadingProfile,
    isAuthenticated,
    setToken,
    setUser,
    clearAuth,
    login,
    fetchProfile,
    logout
  }
})
