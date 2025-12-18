<template>
  <div class="stats-page">
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
          <n-button quaternary @click="router.push('/files')">
            <template #icon>
              <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor"><path d="M10 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z"/></svg>
            </template>
            文件列表
          </n-button>
          <n-button quaternary type="error" @click="handleLogout">退出</n-button>
        </n-space>
      </n-layout-header>

      <n-layout-content class="content">
        <n-grid :cols="1" :x-gap="24" :y-gap="24">
          <n-gi>
            <n-card title="存储空间使用情况">
              <template #header-extra>
                <n-button @click="loadStats">
                  <template #icon>
                    <n-icon><svg viewBox="0 0 24 24"><path fill="currentColor" d="M17.65 6.35A7.958 7.958 0 0 0 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08A5.99 5.99 0 0 1 12 18c-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"/></svg></n-icon>
                  </template>
                  刷新
                </n-button>
              </template>

              <n-spin :show="loading">
                <n-space vertical size="large" v-if="stats">
                  <n-progress
                    type="circle"
                    :percentage="stats.usagePercent"
                    :stroke-width="12"
                    style="margin: 0 auto; display: block;"
                  >
                    <div style="text-align: center;">
                      <div style="font-size: 24px; font-weight: bold;">{{ Math.round(stats.usagePercent) }}%</div>
                      <div style="font-size: 12px; color: #999; margin-top: 4px;">已使用</div>
                    </div>
                  </n-progress>

                  <n-descriptions bordered :column="2">
                    <n-descriptions-item label="已用空间">
                      <n-text strong>{{ stats.usedSpaceFormatted }}</n-text>
                    </n-descriptions-item>
                    <n-descriptions-item label="总空间">
                      <n-text>{{ stats.totalSpaceFormatted }}</n-text>
                    </n-descriptions-item>
                    <n-descriptions-item label="文件数量">
                      <n-text strong>{{ stats.fileCount }}</n-text>
                    </n-descriptions-item>
                    <n-descriptions-item label="今日过期">
                      <n-text type="warning">{{ stats.expiringToday }}</n-text>
                    </n-descriptions-item>
                    <n-descriptions-item label="本周过期" :span="2">
                      <n-text type="info">{{ stats.expiringThisWeek }}</n-text>
                    </n-descriptions-item>
                  </n-descriptions>
                </n-space>
              </n-spin>
            </n-card>
          </n-gi>

          <n-gi>
            <n-card title="使用提示">
              <n-space vertical>
                <n-alert type="warning" title="用量说明">
                  显示的存储用量基于本地数据库统计，可能与 R2 实际用量存在差异。
                </n-alert>
                <n-alert type="info" title="存储空间">
                  当前使用 Cloudflare R2 免费层，总容量 10GB。超出后可能产生费用。
                </n-alert>
                <n-alert type="warning" title="文件过期">
                  文件会根据设置的过期时间自动删除，请及时下载重要文件。
                </n-alert>
                <n-alert type="success" title="流量说明">
                  R2 不收取出站流量费用，下载文件完全免费。
                </n-alert>
              </n-space>
            </n-card>
          </n-gi>
        </n-grid>
      </n-layout-content>
    </n-layout>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../services/api'
import {
  NLayout,
  NLayoutHeader,
  NLayoutContent,
  NCard,
  NGrid,
  NGi,
  NButton,
  NSpace,
  NText,
  NTag,
  NIcon,
  NSpin,
  NProgress,
  NDescriptions,
  NDescriptionsItem,
  NAlert,
  useMessage
} from 'naive-ui'

const router = useRouter()
const authStore = useAuthStore()
const message = useMessage()

const loading = ref(false)
const stats = ref(null)

const loadStats = async () => {
  loading.value = true
  try {
    stats.value = await api.getStats()
  } catch (error) {
    message.error('加载存储统计失败')
  } finally {
    loading.value = false
  }
}

const handleLogout = () => {
  authStore.logout()
  router.push('/login')
}

onMounted(() => {
  loadStats()
})
</script>

<style scoped>
.stats-page {
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
</style>
