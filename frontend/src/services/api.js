import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000,
  withCredentials: true
})

// 请求拦截器
api.interceptors.request.use(
  config => {
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  error => Promise.reject(error)
)

// 响应拦截器
api.interceptors.response.use(
  response => response.data,
  error => {
    // 登录相关接口不自动跳转
    const isAuthApi = error.config?.url?.includes('/auth/')
    if (error.response?.status === 401 && !isAuthApi) {
      localStorage.removeItem('auth_token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default {
  // 认证
  login(password) {
    return api.post('/auth/login', { password })
  },

  getAuthStatus() {
    return api.get('/auth/status')
  },

  getPasswordStatus() {
    return api.get('/auth/password-status')
  },

  setupPassword(password) {
    return api.post('/auth/setup-password', { password })
  },

  // R2 配置
  getSetupStatus() {
    return api.get('/setup/status')
  },

  saveR2Config(config) {
    return api.post('/setup/config', config)
  },

  testR2Connection(config) {
    return api.post('/setup/test', config)
  },

  // 文件上传
  getUploadURL(data) {
    return api.post('/upload/presign', data)
  },

  initMultipartUpload(data) {
    return api.post('/upload/multipart/init', data)
  },

  getMultipartUploadURL(data) {
    return api.post('/upload/multipart/presign', data)
  },

  completeMultipartUpload(data) {
    return api.post('/upload/multipart/complete', data)
  },

  confirmUpload(fileId) {
    return api.post('/upload/confirm', { file_id: fileId })
  },

  // 文件管理
  getFiles(page = 1, limit = 20) {
    return api.get('/files', { params: { page, limit } })
  },

  deleteFile(fileId) {
    return api.delete(`/files/${fileId}`)
  },

  getDownloadURL(fileId) {
    return `/api/files/${fileId}/download`
  },

  // 存储统计
  getStats() {
    return api.get('/stats')
  },

  // 取消上传
  cancelUpload(data) {
    return api.post('/upload/cancel', data)
  },

  // 直接上传到 R2（使用预签名 URL，支持 AbortController）
  uploadToR2(url, file, onProgress, abortSignal) {
    return axios.put(url, file, {
      headers: {
        'Content-Type': file.type || 'application/octet-stream'
      },
      signal: abortSignal,
      onUploadProgress: progressEvent => {
        if (onProgress) {
          const percent = Math.round((progressEvent.loaded * 100) / progressEvent.total)
          onProgress(percent, progressEvent.loaded, progressEvent.total)
        }
      }
    })
  }
}
