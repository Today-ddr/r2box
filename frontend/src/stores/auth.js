import { defineStore } from 'pinia'
import api from '../services/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    isAuthenticated: false,
    needSetup: false,
    token: localStorage.getItem('auth_token') || ''
  }),

  actions: {
    async login(password) {
      try {
        const response = await api.login(password)
        if (response.success) {
          this.isAuthenticated = true
          this.needSetup = response.need_setup
          // Cookie 已由后端设置，这里存储用于 API 请求
          const hash = await this.hashPassword(password)
          this.token = hash
          localStorage.setItem('auth_token', hash)
          return { success: true, needSetup: response.need_setup }
        }
        return { success: false, message: response.message }
      } catch (error) {
        return { success: false, message: error.response?.data?.message || error.response?.data?.error || '登录失败' }
      }
    },

    async hashPassword(password) {
      const encoder = new TextEncoder()
      const data = encoder.encode(password)
      const hashBuffer = await crypto.subtle.digest('SHA-256', data)
      const hashArray = Array.from(new Uint8Array(hashBuffer))
      return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
    },

    async checkAuth() {
      if (!this.token) {
        return false
      }
      try {
        const response = await api.getAuthStatus()
        this.isAuthenticated = response.authenticated
        this.needSetup = response.need_setup
        return true
      } catch (error) {
        this.logout()
        return false
      }
    },

    logout() {
      this.isAuthenticated = false
      this.token = ''
      localStorage.removeItem('auth_token')
    }
  }
})
