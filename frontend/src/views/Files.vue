<template>
  <div class="files-page">
    <n-layout>
      <n-layout-header class="header">
        <div class="header-left">
          <div class="logo">
            <img src="/logo.png" alt="R2Box" class="logo-icon" />
            <span class="logo-text">R2Box</span>
          </div>
        </div>
        <n-space align="center" :size="16">
          <n-button quaternary @click="router.push('/')">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M9 16h6v-6h4l-7-7-7 7h4zm-4 2h14v2H5z"/></svg>
            </template>
            上传文件
          </n-button>
          <n-button quaternary @click="router.push('/stats')">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zM9 17H7v-7h2v7zm4 0h-2V7h2v10zm4 0h-2v-4h2v4z"/></svg>
            </template>
            存储统计
          </n-button>
          <n-button quaternary type="error" @click="handleLogout">退出</n-button>
        </n-space>
      </n-layout-header>

      <n-layout-content class="content">
        <n-card title="已上传文件">
          <template #header-extra>
            <n-button @click="loadFiles">
              <template #icon>
                <n-icon><svg viewBox="0 0 24 24"><path fill="currentColor" d="M17.65 6.35A7.958 7.958 0 0 0 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08A5.99 5.99 0 0 1 12 18c-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg></n-icon>
              </template>
              刷新
            </n-button>
          </template>

          <n-data-table
            :columns="columns"
            :data="filesStore.files"
            :loading="filesStore.loading"
            :pagination="pagination"
            :bordered="false"
          />
        </n-card>
      </n-layout-content>
    </n-layout>

    <!-- 文件信息弹窗 -->
    <n-modal v-model:show="showInfoModal" preset="card" title="文件信息" style="width: 500px; border-radius: 16px;">
      <template v-if="selectedFile">
        <n-descriptions bordered :column="1">
          <n-descriptions-item label="文件名">{{ selectedFile.filename }}</n-descriptions-item>
          <n-descriptions-item label="文件大小">{{ formatBytes(selectedFile.size) }}</n-descriptions-item>
          <n-descriptions-item label="上传时间">{{ new Date(selectedFile.created_at).toLocaleString('zh-CN') }}</n-descriptions-item>
          <n-descriptions-item label="剩余时间">{{ selectedFile.remaining_time }}</n-descriptions-item>
        </n-descriptions>

        <n-divider />

        <div class="link-group">
          <n-text depth="3" style="font-size: 12px;">短链接</n-text>
          <n-input-group>
            <n-input :value="getShortUrl(selectedFile)" readonly />
            <n-button type="primary" @click="copyUrl(getShortUrl(selectedFile), '短链接')">复制</n-button>
          </n-input-group>
        </div>

        <div class="link-group" style="margin-top: 12px;">
          <n-text depth="3" style="font-size: 12px;">直链</n-text>
          <n-input-group>
            <n-input :value="getDownloadUrl(selectedFile)" readonly />
            <n-button type="primary" @click="copyUrl(getDownloadUrl(selectedFile), '直链')">复制</n-button>
          </n-input-group>
        </div>
      </template>

      <template #footer>
        <n-space justify="end">
          <n-button @click="showInfoModal = false">关闭</n-button>
          <n-button type="primary" @click="handleDownload(selectedFile)">下载文件</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, h, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useFilesStore } from '../stores/files'
import {
  NLayout,
  NLayoutHeader,
  NLayoutContent,
  NCard,
  NButton,
  NSpace,
  NText,
  NTag,
  NIcon,
  NDataTable,
  NPopconfirm,
  NModal,
  NDescriptions,
  NDescriptionsItem,
  NDivider,
  NInput,
  NInputGroup,
  useMessage
} from 'naive-ui'

const router = useRouter()
const authStore = useAuthStore()
const filesStore = useFilesStore()
const message = useMessage()

const showInfoModal = ref(false)
const selectedFile = ref(null)

const pagination = ref({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: (page) => {
    pagination.value.page = page
    loadFiles()
  },
  onUpdatePageSize: (pageSize) => {
    pagination.value.pageSize = pageSize
    pagination.value.page = 1
    loadFiles()
  }
})

const columns = [
  {
    title: '文件名',
    key: 'filename',
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '文件大小',
    key: 'size',
    width: 100,
    render: (row) => formatBytes(row.size)
  },
  {
    title: '有效期',
    key: 'expires_in',
    width: 80,
    render: (row) => {
      if (row.expires_in === -30) return '30秒'
      return row.expires_in + '天'
    }
  },
  {
    title: '状态',
    key: 'upload_status',
    width: 100,
    render: (row) => {
      if (row.upload_status === 'deleted') {
        return h(NTag, { type: 'error', size: 'small' }, { default: () => '已过期' })
      }
      return h(NTag, { type: 'success', size: 'small' }, { default: () => '有效' })
    }
  },
  {
    title: '剩余时间',
    key: 'remaining_time',
    width: 180,
    render: (row) => row.upload_status === 'deleted' ? '-' : row.remaining_time
  },
  {
    title: '上传时间',
    key: 'created_at',
    width: 180,
    render: (row) => new Date(row.created_at).toLocaleString('zh-CN')
  },
  {
    title: '操作',
    key: 'actions',
    width: 140,
    render: (row) => {
      const isDeleted = row.upload_status === 'deleted'
      return h('div', { style: 'display: flex; gap: 8px;' }, [
        h(
          NButton,
          {
            size: 'small',
            type: 'info',
            disabled: isDeleted,
            onClick: () => showFileInfo(row)
          },
          { default: () => '详情' }
        ),
        h(
          NPopconfirm,
          {
            positiveText: '确定',
            negativeText: '取消',
            onPositiveClick: () => handleDelete(row.id)
          },
          {
            trigger: () => h(
              NButton,
              {
                size: 'small',
                type: 'error',
                disabled: isDeleted
              },
              { default: () => '删除' }
            ),
            default: () => '确定要删除这个文件吗？'
          }
        )
      ])
    }
  }
]

const formatBytes = (bytes) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i]
}

const getShortUrl = (file) => {
  return window.location.origin + '/s/' + file.short_code
}

const getDownloadUrl = (file) => {
  return file.download_url || (window.location.origin + `/api/files/${file.id}/download`)
}

const copyUrl = (url, type) => {
  navigator.clipboard.writeText(url)
  message.success(`${type}已复制到剪贴板`)
}

const showFileInfo = (row) => {
  selectedFile.value = row
  showInfoModal.value = true
}

const loadFiles = async () => {
  try {
    await filesStore.fetchFiles(pagination.value.page)
    pagination.value.itemCount = filesStore.total
  } catch (error) {
    message.error('加载文件列表失败')
  }
}

const handleDownload = (row) => {
  const downloadUrl = getDownloadUrl(row)
  window.open(downloadUrl, '_blank')
}

const handleDelete = async (fileId) => {
  try {
    await filesStore.deleteFile(fileId)
    message.success('文件已删除')
  } catch (error) {
    message.error('删除文件失败')
  }
}

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}

onMounted(() => {
  loadFiles()
})
</script>

<style scoped>
.files-page {
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
  max-width: 1200px;
  margin: 0 auto;
}

.link-group {
  margin-bottom: 4px;
}
</style>
