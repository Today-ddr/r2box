import { defineStore } from 'pinia'
import api from '../services/api'

export const useFilesStore = defineStore('files', {
  state: () => ({
    files: [],
    total: 0,
    page: 1,
    limit: 20,
    loading: false
  }),

  actions: {
    async fetchFiles(page = 1) {
      this.loading = true
      try {
        const response = await api.getFiles(page, this.limit)
        this.files = response.files || []
        this.total = response.total
        this.page = response.page
        this.loading = false
      } catch (error) {
        this.loading = false
        throw error
      }
    },

    async deleteFile(fileId) {
      try {
        await api.deleteFile(fileId)
        // 重新加载当前页
        await this.fetchFiles(this.page)
      } catch (error) {
        throw error
      }
    }
  }
})
