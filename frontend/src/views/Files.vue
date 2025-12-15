<template>
  <div class="files-page">
    <n-layout>
      <n-layout-header class="header">
        <div class="header-left">
          <div class="logo">
            <span class="logo-icon">📦</span>
            <span class="logo-text">R2Box</span>
          </div>
        </div>
        <n-space align="center" :size="16">
          <n-button quaternary @click="router.push('/')">📤 上传文件</n-button>
          <n-button quaternary @click="router.push('/stats')">📊 存储统计</n-button>
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
  useMessage
} from 'naive-ui'

const router = useRouter()
const authStore = useAuthStore()
const filesStore = useFilesStore()
const message = useMessage()

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
    width: 180,
    render: (row) => {
      const isDeleted = row.upload_status === 'deleted'
      return h('div', { style: 'display: flex; gap: 8px;' }, [
        h(
          NButton,
          {
            size: 'small',
            type: 'primary',
            disabled: isDeleted,
            onClick: () => handleDownload(row)
          },
          { default: () => '下载' }
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

const loadFiles = async () => {
  try {
    await filesStore.fetchFiles(pagination.value.page)
    pagination.value.itemCount = filesStore.total
  } catch (error) {
    message.error('加载文件列表失败')
  }
}

const handleDownload = (row) => {
  // 优先使用后端返回的 R2 直链
  const downloadUrl = row.download_url || (window.location.origin + `/api/files/${row.id}/download`)
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
  font-size: 28px;
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
</style>
