import { defineStore } from 'pinia'
import api from '../services/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    isAuthenticated: false,
    needSetup: false,
    token: localStorage.getItem('auth_token') || ''
  }),

  actions: {
    async login(token) {
      try {
        const response = await api.login(token)
        if (response.success) {
          this.isAuthenticated = true
          this.needSetup = response.need_setup
          this.token = token
          localStorage.setItem('auth_token', token)
          return { success: true, needSetup: response.need_setup }
        }
        return { success: false, message: response.message }
      } catch (error) {
        return { success: false, message: error.response?.data?.error || '登录失败' }
      }
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
