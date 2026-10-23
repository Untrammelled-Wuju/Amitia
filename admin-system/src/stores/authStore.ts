import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { setCsrfToken } from '@/api/client'
import * as api from '@/api/releases'
import logo from '@/assets/images/logo.jpg'
interface User {
  id: number
  username: string
  avatar: string
  role: 'admin' | 'publisher'
}

export const useAuthStore = defineStore(
  'auth',
  () => {
    const user = ref<User | null>(null)
    const initialized = ref(false)
    const isAuthenticated = computed(() => Boolean(user.value))

    const applyUser = (payload: { user: { id: number; username: string; role: 'admin' | 'publisher' } }) => {
      user.value = {
        ...payload.user,
        avatar: logo,
      }
    }

    const login = async (username: string, password: string) => {
      const payload = await api.login(username, password)
      setCsrfToken(payload.csrfToken)
      applyUser(payload)
      initialized.value = true
    }

    const updateUserInfo = (_token: string, newUserInfo: User) => {
      user.value = newUserInfo
    }

    const logout = async () => {
      try {
        await api.logout()
      } finally {
        user.value = null
        initialized.value = true
        setCsrfToken('')
        window.location.href = '/login'
      }
    }

    const initializeAuth = async () => {
      if (initialized.value) return
      try {
        const payload = await api.loadMe()
        setCsrfToken(payload.csrfToken)
        applyUser(payload)
      } catch {
        user.value = null
        setCsrfToken('')
      } finally {
        initialized.value = true
      }
    }
    return { isAuthenticated, user, initialized, login, updateUserInfo, logout, initializeAuth }
  },
)
