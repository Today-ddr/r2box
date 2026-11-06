<template>
  <div class="upload-page">
    <n-layout>
      <n-layout-header class="header">
        <div class="header-left">
          <div class="logo">
            <img src="/logo.png" alt="R2Box" class="logo-icon" />
            <span class="logo-text">R2Box</span>
          </div>
        </div>
        <n-space align="center" :size="16">
          <n-button quaternary @click="router.push('/files')">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>
            </template>
            文件列表
          </n-button>
          <n-button quaternary @click="router.push('/stats')">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM9 17H7v-7h2v7zm4 0h-2V7h2v10zm4 0h-2v-4h2v4z"/></svg>
            </template>
            存储统计
          </n-button>
          <n-button quaternary @click="showConfigModal = true">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19.14 12.94c.04-.31.06-.63.06-.94 0-.31-.02-.63-.06-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/></svg>
            </template>
            R2 配置
          </n-button>
          <n-button quaternary tag="a" href="https://github.com/Today-ddr/r2box" target="_blank">
            <template #icon>
              <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
            </template>
          </n-button>
          <VersionBadge />
          <n-button quaternary type="error" @click="handleLogout">退出</n-button>
        </n-space>
      </n-layout-header>

      <n-layout-content class="content">
        <!-- 右上角悬浮存储用量指示器 -->
        <div class="storage-widget" v-if="storageStats">
          <n-tooltip trigger="hover">
            <template #trigger>
              <div class="storage-ring-container">
                <n-progress
                  type="circle"
                  :percentage="Math.min(storageStats.usagePercent, 100)"
                  :stroke-width="8"
                  :color="getStorageColor(storageStats.usagePercent)"
                  :rail-color="'#e5e7eb'"
                  :show-indicator="false"
                  style="width: 36px; height: 36px;"
                />
                <div class="storage-ring-icon">
                  <svg viewBox="0 0 24 24" width="14" height="14">
                    <path fill="currentColor" d="M2 20h20v-4H2v4zm2-3h2v2H4v-2zM2 4v4h20V4H2zm4 3H4V5h2v2zm-4 7h20v-4H2v4zm2-3h2v2H4v-2z"/>
                  </svg>
                </div>
              </div>
            </template>
            <div style="text-align: center;">
              <div style="font-weight: 600; margin-bottom: 4px;">存储空间</div>
              <div>{{ storageStats.usedSpaceFormatted }} / {{ storageStats.totalSpaceFormatted }}</div>
              <div v-if="storageStats.inFlightSpace > 0" style="color: #f59e0b; font-size: 12px; margin-top: 2px;">上传中 {{ storageStats.inFlightFormatted }}</div>
              <div style="color: #999; font-size: 12px; margin-top: 2px;">已使用 {{ Math.round(storageStats.usagePercent) }}%</div>
            </div>
          </n-tooltip>
        </div>

        <n-grid :cols="1" :x-gap="24" :y-gap="24">
          <n-gi>
            <n-card title="上传文件">
              <n-upload
                ref="uploadRef"
                :custom-request="handleUpload"
                :max="1"
                :show-file-list="false"
                @before-upload="beforeUpload"
              >
                <n-upload-dragger>
                  <div style="margin-bottom: 12px;">
                    <n-icon size="48" :depth="3">
                      <svg viewBox="0 0 24 24"><path fill="currentColor" d="M9 16h6v-6h4l-7-7-7 7h4zm-4 2h14v2H5z"/></svg>
                    </n-icon>
                  </div>
                  <n-text style="font-size: 16px;">
                    点击或拖拽文件到此区域上传
                  </n-text>
                  <n-p depth="3" style="margin: 8px 0 0 0;">
                    支持单个文件上传，最大 5GB
                  </n-p>
                </n-upload-dragger>
              </n-upload>

              <n-divider />

              <n-form-item label="过期时间">
                <n-radio-group v-model:value="expiresIn">
                  <n-space>
                    <n-radio :value="1">1天</n-radio>
                    <n-radio :value="3">3天</n-radio>
                    <n-radio :value="7">7天</n-radio>
                    <n-radio :value="30">30天</n-radio>
                  </n-space>
                </n-radio-group>
              </n-form-item>

              <n-alert v-if="isUploading" type="info" style="margin-top: 16px;">
                <template #header>
                  <div style="display: flex; justify-content: space-between; align-items: center;">
                    <span>上传中: {{ currentFile?.name }}</span>
                    <n-button
                      size="small"
                      type="error"
                      :loading="isCancelling"
                      @click="handleCancelUpload"
                    >
                      {{ isCancelling ? '取消中...' : '取消上传' }}
                    </n-button>
                  </div>
                </template>
                <n-progress
                  type="line"
                  :percentage="displayProgress"
                  :indicator-placement="'inside'"
                  processing
                />
                <div class="upload-stats">
                  <span>{{ formatBytes(uploadedSize) }} / {{ formatBytes(totalSize) }}</span>
                  <span>{{ uploadSpeed }}</span>
                  <span>剩余 {{ remainingTime }}</span>
                </div>
              </n-alert>

              <n-alert v-if="uploadResult" :type="uploadResult.success ? 'success' : 'error'" style="margin-top: 16px;">
                <template #header>
                  {{ uploadResult.success ? '上传成功！' : '上传失败' }}
                </template>
                <div v-if="uploadResult.success">
                  <div class="file-info" style="margin-bottom: 12px;">
                    <n-text strong style="font-size: 15px; word-break: break-all;">📄 {{ uploadResult.filename }}</n-text>
                  </div>
                  <div class="upload-summary">
                    <n-tag type="info" size="small">{{ uploadResult.fileSize }}</n-tag>
                    <n-tag type="success" size="small">{{ uploadResult.avgSpeed }}</n-tag>
                    <n-tag type="warning" size="small">{{ uploadResult.duration }}</n-tag>
                  </div>
                  <n-p style="margin-top: 8px; margin-bottom: 8px;">文件将在 {{ expiresIn }} 天后自动删除</n-p>
                  <div class="link-group">
                    <n-text depth="3" style="font-size: 12px;">短链接</n-text>
                    <n-input-group>
                      <n-input :value="uploadResult.shortUrl" readonly />
                      <n-button type="primary" @click="copyShortUrl">复制</n-button>
                    </n-input-group>
                  </div>
                  <div class="link-group" style="margin-top: 12px;">
                    <n-text depth="3" style="font-size: 12px;">直链</n-text>
                    <n-input-group>
                      <n-input :value="uploadResult.downloadUrl" readonly />
                      <n-button @click="copyDownloadUrl">复制</n-button>
                    </n-input-group>
                  </div>
                </div>
                <div v-else>
                  {{ uploadResult.message }}
                </div>
              </n-alert>
            </n-card>
          </n-gi>
        </n-grid>
      </n-layout-content>
    </n-layout>

    <!-- R2 配置弹窗 -->
    <n-modal v-model:show="showConfigModal" preset="card" title="R2 存储配置" style="width: 600px; border-radius: 20px;">
      <n-form
        ref="configFormRef"
        :model="configForm"
        :rules="configRules"
        label-placement="left"
        label-width="140"
      >
        <n-form-item label="R2 端点 URL" path="endpoint">
          <n-input
            v-model:value="configForm.endpoint"
            placeholder="https://xxxxxxxx.r2.cloudflarestorage.com"
          />
        </n-form-item>

        <n-form-item label="Access Key ID" path="access_key_id">
          <n-input
            v-model:value="configForm.access_key_id"
            placeholder="R2 访问密钥 ID"
          />
        </n-form-item>

        <n-form-item label="Secret Access Key" path="secret_access_key">
          <n-input
            v-model:value="configForm.secret_access_key"
            type="password"
            placeholder="R2 访问密钥（留空则不修改）"
            show-password-on="click"
          />
        </n-form-item>

        <n-form-item label="Bucket Name" path="bucket_name">
          <n-input
            v-model:value="configForm.bucket_name"
            placeholder="存储桶名称"
          />
        </n-form-item>
      </n-form>

      <n-alert v-if="configTestResult" :type="configTestResult.success ? 'success' : 'error'" :title="configTestResult.message" style="margin-bottom: 16px;" />

      <template #footer>
        <n-space justify="end">
          <n-button @click="showConfigModal = false">取消</n-button>
          <n-button type="info" :loading="configTesting" @click="handleTestConfig">测试连接</n-button>
          <n-button type="primary" :loading="configSaving" :disabled="!configTestPassed" @click="handleSaveConfig">保存配置</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../services/api'
import VersionBadge from '../components/VersionBadge.vue'
import {
  NLayout,
  NLayoutHeader,
  NLayoutContent,
  NCard,
  NGrid,
  NGi,
  NUpload,
  NUploadDragger,
  NButton,
  NSpace,
  NText,
  NP,
  NIcon,
  NTag,
  NDivider,
  NFormItem,
  NRadioGroup,
  NRadio,
  NProgress,
  NAlert,
  NInput,
  NInputGroup,
  NModal,
  NForm,
  NTooltip,
  useMessage
} from 'naive-ui'

const router = useRouter()
const authStore = useAuthStore()
const message = useMessage()

const appVersion = __APP_VERSION__

const uploadRef = ref(null)
const expiresIn = ref(7)
const uploadProgress = ref(0)
const currentFile = ref(null)
const uploadResult = ref(null)

// 存储统计
const storageStats = ref(null)
const storageLoading = ref(false)

// 上传统计
const isUploading = ref(false)
const uploadedSize = ref(0)
const totalSize = ref(0)
const uploadSpeed = ref('0 B/s')
const remainingTime = ref('计算中...')
const displayProgress = ref(0) // 用于平滑显示的进度
let uploadStartTime = 0
let lastUpdateTime = 0
let lastLoaded = 0
let animationFrame = null

// 上传取消相关
let abortController = null
const currentFileId = ref(null)
const currentUploadId = ref(null)
const isCancelling = ref(false)

const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return (bytes / Math.pow(k, i)).toFixed(2) + ' ' + sizes[i]
}

// 根据存储使用百分比返回对应颜色
const getStorageColor = (percent) => {
  if (percent > 95) return '#ef4444' // 危险 - 红色
  if (percent > 80) return '#f59e0b' // 警告 - 橙色
  return '#0070f3' // 正常 - 蓝色
}

const formatDuration = (seconds) => {
  if (seconds < 60) {
    return `${seconds.toFixed(1)} 秒`
  } else if (seconds < 3600) {
    const mins = Math.floor(seconds / 60)
    const secs = Math.round(seconds % 60)
    return `${mins} 分 ${secs} 秒`
  } else {
    const hours = Math.floor(seconds / 3600)
    const mins = Math.round((seconds % 3600) / 60)
    return `${hours} 小时 ${mins} 分`
  }
}

// 平滑动画更新进度
const animateProgress = () => {
  const target = uploadProgress.value
  const current = displayProgress.value
  const diff = target - current

  if (Math.abs(diff) > 0.5) {
    // 使用 easing 函数平滑过渡，取整确保不显示小数
    displayProgress.value = Math.round(current + diff * 0.2)
    animationFrame = requestAnimationFrame(animateProgress)
  } else {
    displayProgress.value = Math.round(target)
    animationFrame = null
  }
}

const updateUploadStats = (loaded, total) => {
  const now = Date.now()
  uploadedSize.value = loaded
  totalSize.value = total

  // 计算精确进度（保留整数）
  const exactProgress = Math.round((loaded / total) * 100)
  uploadProgress.value = Math.min(exactProgress, 100)

  // 当上传完成时直接设置100%，跳过动画延迟
  if (loaded >= total) {
    displayProgress.value = 100
    if (animationFrame) {
      cancelAnimationFrame(animationFrame)
      animationFrame = null
    }
    return
  }

  // 启动平滑动画（如果没有在运行）
  if (!animationFrame && isUploading.value) {
    animationFrame = requestAnimationFrame(animateProgress)
  }

  // 计算速度
  const elapsed = (now - uploadStartTime) / 1000
  if (elapsed > 0.5) {
    const avgSpeed = loaded / elapsed
    uploadSpeed.value = formatBytes(avgSpeed) + '/s'

    // 计算剩余时间
    if (avgSpeed > 0) {
      const remaining = (total - loaded) / avgSpeed
      if (remaining < 60) {
        remainingTime.value = Math.round(remaining) + ' 秒'
      } else if (remaining < 3600) {
        remainingTime.value = Math.round(remaining / 60) + ' 分钟'
      } else {
        remainingTime.value = (remaining / 3600).toFixed(1) + ' 小时'
      }
    }
  }
}

// R2 配置弹窗
const showConfigModal = ref(false)
const configFormRef = ref(null)
const configForm = ref({
  endpoint: '',
  access_key_id: '',
  secret_access_key: '',
  bucket_name: ''
})
const configRules = {
  endpoint: { required: true, message: '请输入 R2 端点 URL', trigger: 'blur' },
  bucket_name: { required: true, message: '请输入 Bucket Name', trigger: 'blur' }
}
const configTesting = ref(false)
const configSaving = ref(false)
const configTestPassed = ref(false)
const configTestResult = ref(null)

const MAX_FILE_SIZE = 5 * 1024 * 1024 * 1024 // 5GB

// 加载存储统计
const loadStorageStats = async () => {
  storageLoading.value = true
  try {
    storageStats.value = await api.getStats()
  } catch (error) {
    console.error('加载存储统计失败:', error)
  } finally {
    storageLoading.value = false
  }
}

// 加载已有配置
onMounted(async () => {
  // 并行加载配置和存储统计
  loadStorageStats()

  try {
    const status = await api.getSetupStatus()
    if (status.configured && status.config) {
      configForm.value.endpoint = status.config.endpoint || ''
      configForm.value.bucket_name = status.config.bucket_name || ''
    }
  } catch (error) {
    console.error('加载配置失败:', error)
  }
})

const beforeUpload = ({ file }) => {
  if (file.file.size > MAX_FILE_SIZE) {
    message.error('文件大小超过 5GB 限制')
    return false
  }
  return true
}

// 取消上传
const handleCancelUpload = async () => {
  if (!isUploading.value || isCancelling.value) return

  isCancelling.value = true

  try {
    // 1. 中断前端请求
    if (abortController) {
      abortController.abort()
      abortController = null
    }

    // 2. 调用后端清理 R2 数据
    if (currentFileId.value) {
      await api.cancelUpload({
        file_id: currentFileId.value,
        upload_id: currentUploadId.value || ''
      })
    }

    message.warning('上传已取消')
    uploadResult.value = {
      success: false,
      message: '上传已被用户取消'
    }
  } catch (error) {
    // 忽略 AbortError
    if (error.name !== 'AbortError' && error.code !== 'ERR_CANCELED') {
      console.error('取消上传时出错:', error)
    }
  } finally {
    // 重置状态
    isUploading.value = false
    isCancelling.value = false
    currentFileId.value = null
    currentUploadId.value = null
    if (animationFrame) {
      cancelAnimationFrame(animationFrame)
      animationFrame = null
    }
  }
}

const handleUpload = async ({ file }) => {
  currentFile.value = file
  uploadProgress.value = 0
  displayProgress.value = 0
  uploadResult.value = null

  // 初始化 AbortController
  abortController = new AbortController()
  currentFileId.value = null
  currentUploadId.value = null

  // 立即显示上传状态
  isUploading.value = true
  uploadedSize.value = 0
  totalSize.value = file.file.size
  uploadSpeed.value = '准备中...'
  remainingTime.value = '计算中...'
  uploadStartTime = Date.now()
  lastUpdateTime = Date.now()
  lastLoaded = 0
  animationFrame = null

  try {
    // 清除上传组件状态以允许连续上传
    uploadRef.value?.clear()
    const fileSize = file.file.size

    // 小文件直接上传
    if (fileSize < 100 * 1024 * 1024) {
      await uploadSmallFile(file)
    } else {
      // 大文件分片上传
      await uploadLargeFile(file)
    }
  } catch (error) {
    // 检查是否是取消操作
    if (error.name === 'AbortError' || error.code === 'ERR_CANCELED') {
      // 取消操作已在 handleCancelUpload 中处理
      return
    }
    console.error('上传错误:', error)
    uploadResult.value = {
      success: false,
      message: error.response?.data?.error || error.message || '上传失败'
    }
    // 上传失败时清理 R2 数据
    if (currentFileId.value) {
      try {
        await api.cancelUpload({
          file_id: currentFileId.value,
          upload_id: currentUploadId.value || ''
        })
      } catch (cleanupError) {
        console.error('清理失败上传数据时出错:', cleanupError)
      }
    }
  } finally {
    isUploading.value = false
    abortController = null
    currentFileId.value = null
    currentUploadId.value = null
    // 取消动画帧
    if (animationFrame) {
      cancelAnimationFrame(animationFrame)
      animationFrame = null
    }
  }
}

const uploadSmallFile = async (file) => {
  // 获取预签名 URL
  const response = await api.getUploadURL({
    filename: file.name,
    content_type: file.type || 'application/octet-stream',
    size: file.file.size,
    expires_in: expiresIn.value
  })

  // 保存 file_id 用于取消操作
  currentFileId.value = response.file_id

  // 直接上传到 R2（传入 abort signal）
  await api.uploadToR2(response.upload_url, file.file, (percent, loaded, total) => {
    updateUploadStats(loaded, total)
  }, abortController?.signal)

  // 计算上传统计
  const uploadEndTime = Date.now()
  const duration = (uploadEndTime - uploadStartTime) / 1000
  const avgSpeed = file.file.size / duration

  uploadProgress.value = 100
  displayProgress.value = 100
  // 确认上传完成并获取直链
  const confirmResult = await api.confirmUpload(response.file_id)

  // 判断 download_url 是否为完整 URL（R2 直链）
  const downloadUrl = confirmResult.download_url?.startsWith('http')
    ? confirmResult.download_url
    : window.location.origin + (confirmResult.download_url || response.download_url)

  uploadResult.value = {
    success: true,
    filename: file.name,
    downloadUrl: downloadUrl,
    shortUrl: window.location.origin + (confirmResult.short_url || response.short_url),
    fileSize: formatBytes(file.file.size),
    avgSpeed: formatBytes(avgSpeed) + '/s',
    duration: formatDuration(duration)
  }

  message.success('文件上传成功！')
}

const uploadLargeFile = async (file) => {
  // 初始化分片上传
  const initResponse = await api.initMultipartUpload({
    filename: file.name,
    content_type: file.type || 'application/octet-stream',
    size: file.file.size,
    expires_in: expiresIn.value
  })

  const { file_id, upload_id, part_size, total_parts } = initResponse

  // 保存 file_id 和 upload_id 用于取消操作
  currentFileId.value = file_id
  currentUploadId.value = upload_id

  const CONCURRENCY = 3 // 并发数
  let completedBytes = 0
  const partProgress = new Array(total_parts).fill(0) // 每个分片的进度
  let isCancelled = false

  // 更新总进度
  const updateTotalProgress = () => {
    const totalLoaded = partProgress.reduce((a, b) => a + b, 0)
    updateUploadStats(totalLoaded, file.file.size)
  }

  // 检查是否已取消
  const checkCancelled = () => {
    if (abortController?.signal?.aborted) {
      isCancelled = true
      const error = new Error('上传已取消')
      error.name = 'AbortError'
      throw error
    }
  }

  // 上传单个分片（带重试和实时进度）
  const uploadPart = async (partIndex) => {
    checkCancelled()

    const partNumber = partIndex + 1
    const start = partIndex * part_size
    const end = Math.min(start + part_size, file.file.size)
    const chunk = file.file.slice(start, end)

    for (let attempt = 1; attempt <= 3; attempt++) {
      try {
        checkCancelled()

        // 获取分片预签名 URL
        const presignResponse = await api.getMultipartUploadURL({
          file_id,
          upload_id,
          part_number: partNumber
        })

        checkCancelled()

        // 上传分片（带实时进度和 abort signal）
        const uploadResponse = await api.uploadToR2(presignResponse.upload_url, chunk, (percent, loaded) => {
          partProgress[partIndex] = loaded
          updateTotalProgress()
        }, abortController?.signal)

        // 获取 ETag
        let etag = uploadResponse.headers?.etag || ''
        if (!etag) {
          throw new Error(`分片 ${partNumber} 未返回 ETag`)
        }
        if (!etag.startsWith('"')) {
          etag = `"${etag}"`
        }

        // 确保进度完整
        partProgress[partIndex] = end - start
        updateTotalProgress()

        return { part_number: partNumber, etag }
      } catch (err) {
        // 如果是取消操作，直接抛出
        if (err.name === 'AbortError' || err.code === 'ERR_CANCELED') {
          throw err
        }
        if (attempt === 3) throw err
        await new Promise(r => setTimeout(r, 1000 * attempt))
      }
    }
  }

  // 并发上传所有分片（使用 Promise 池）
  const uploadedParts = []
  let currentIndex = 0

  const uploadNext = async () => {
    while (currentIndex < total_parts && !isCancelled) {
      checkCancelled()
      const partIndex = currentIndex++
      try {
        const result = await uploadPart(partIndex)
        uploadedParts.push(result)
      } catch (err) {
        throw err
      }
    }
  }

  // 启动并发 workers
  const workers = []
  for (let i = 0; i < Math.min(CONCURRENCY, total_parts); i++) {
    workers.push(uploadNext())
  }

  await Promise.all(workers)

  // 按 part_number 排序
  const validParts = uploadedParts
    .filter(p => p && p.etag)
    .sort((a, b) => a.part_number - b.part_number)

  if (validParts.length !== total_parts) {
    throw new Error(`分片上传不完整: ${validParts.length}/${total_parts}`)
  }

  // 完成分片上传
  const completeResponse = await api.completeMultipartUpload({
    file_id,
    upload_id,
    parts: validParts
  })

  // 计算上传统计
  const uploadEndTime = Date.now()
  const duration = (uploadEndTime - uploadStartTime) / 1000
  const avgSpeed = file.file.size / duration

  uploadProgress.value = 100
  displayProgress.value = 100

  // 判断 download_url 是否为完整 URL（R2 直链）
  const downloadUrl = completeResponse.download_url?.startsWith('http')
    ? completeResponse.download_url
    : window.location.origin + completeResponse.download_url

  uploadResult.value = {
    success: true,
    filename: file.name,
    downloadUrl: downloadUrl,
    shortUrl: window.location.origin + completeResponse.short_url,
    fileSize: formatBytes(file.file.size),
    avgSpeed: formatBytes(avgSpeed) + '/s',
    duration: formatDuration(duration)
  }

  message.success('文件上传成功！')
}

const copyShortUrl = () => {
  if (uploadResult.value?.shortUrl) {
    navigator.clipboard.writeText(uploadResult.value.shortUrl)
    message.success('短链接已复制到剪贴板')
  }
}

const copyDownloadUrl = () => {
  if (uploadResult.value?.downloadUrl) {
    navigator.clipboard.writeText(uploadResult.value.downloadUrl)
    message.success('完整链接已复制到剪贴板')
  }
}

// R2 配置相关
const handleTestConfig = async () => {
  if (!configForm.value.endpoint || !configForm.value.bucket_name) {
    message.error('请填写必填字段')
    return
  }
  if (!configForm.value.access_key_id || !configForm.value.secret_access_key) {
    message.error('测试连接需要填写 Access Key')
    return
  }

  configTesting.value = true
  configTestResult.value = null

  try {
    const result = await api.testR2Connection({
      endpoint: configForm.value.endpoint,
      access_key_id: configForm.value.access_key_id,
      secret_access_key: configForm.value.secret_access_key,
      bucket_name: configForm.value.bucket_name
    })

    configTestResult.value = result
    if (result.success) {
      configTestPassed.value = true
      message.success('连接测试成功！')
    } else {
      configTestPassed.value = false
      message.error(result.message)
    }
  } catch (error) {
    configTestResult.value = {
      success: false,
      message: error.response?.data?.message || '连接测试失败'
    }
    message.error('连接测试失败')
  } finally {
    configTesting.value = false
  }
}

const handleSaveConfig = async () => {
  configSaving.value = true

  try {
    const result = await api.saveR2Config({
      endpoint: configForm.value.endpoint,
      access_key_id: configForm.value.access_key_id,
      secret_access_key: configForm.value.secret_access_key,
      bucket_name: configForm.value.bucket_name
    })

    if (result.success) {
      message.success('配置保存成功！')
      showConfigModal.value = false
      configTestPassed.value = false
      configTestResult.value = null
    }
  } catch (error) {
    message.error('保存配置失败')
  } finally {
    configSaving.value = false
  }
}

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.upload-page {
  min-height: 100vh;
  background: #fafafa;
}

.header {
  height: 64px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #eaeaea;
}

.header-left {
  display: flex;
  align-items: center;
}

.logo {
  display: flex;
  align-items: center;
  gap: 8px;
}

.logo-icon {
  width: 28px;
  height: 28px;
  border-radius: 6px;
}

.logo-text {
  font-size: 22px;
  font-weight: 700;
  color: #333;
}

.content {
  padding: 32px;
  max-width: 800px;
  margin: 0 auto;
}

.upload-stats {
  display: flex;
  justify-content: space-between;
  margin-top: 12px;
  font-size: 13px;
  color: #666;
}

.link-group {
  margin-bottom: 4px;
}

.upload-summary {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

/* 进度条动画 - 更平滑 */
:deep(.n-progress-graph-line-fill) {
  transition: width 0.1s linear !important;
}

:deep(.n-progress-graph-line-indicator) {
  transition: left 0.1s linear !important;
}

:deep(.n-progress-graph-line) {
  background: #eaeaea;
  border-radius: 10px;
  overflow: hidden;
}

:deep(.n-progress-graph-line-fill) {
  background: linear-gradient(90deg, #0070f3, #00a8ff);
  border-radius: 10px;
}

:deep(.n-progress-graph-line-rail) {
  border-radius: 10px;
  overflow: hidden;
}

/* 右上角悬浮存储用量指示器 */
.storage-widget {
  position: fixed;
  top: 80px;
  right: 24px;
  z-index: 100;
}

.storage-ring-container {
  position: relative;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  border-radius: 50%;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.storage-ring-container:hover {
  transform: scale(1.08);
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
}

.storage-ring-icon {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #6b7280;
}

/* 圆环进度条样式优化 */
.storage-ring-container :deep(.n-progress-graph-circle-fill) {
  transition: stroke-dashoffset 0.3s ease, stroke 0.3s ease;
}

.version-tag {
  font-size: 12px;
  color: #6b7280;
  background: #f3f4f6;
  padding: 2px 8px;
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
