<template>
  <div class="login-container">
    <div class="login-card">
      <div class="logo">
        <img src="/logo.png" alt="R2Box" class="logo-icon" />
        <span class="logo-text">R2Box</span>
      </div>
      <p class="subtitle">轻量级临时文件分享</p>

      <n-spin :show="checking">
        <n-form ref="formRef" :model="formValue" :rules="rules" style="margin-top: 32px;">
          <n-form-item :label="isSetup ? '设置密码' : '密码'" path="password">
            <n-input
              v-model:value="formValue.password"
              type="password"
              :placeholder="isSetup ? '请设置访问密码' : '请输入密码'"
              show-password-on="click"
              size="large"
              @keyup.enter="handleSubmit"
            />
          </n-form-item>

          <n-form-item v-if="isSetup" label="确认密码" path="confirmPassword">
            <n-input
              v-model:value="formValue.confirmPassword"
              type="password"
              placeholder="请再次输入密码"
              show-password-on="click"
              size="large"
              @keyup.enter="handleSubmit"
            />
          </n-form-item>

          <n-button
            type="primary"
            block
            size="large"
            :loading="loading"
            @click="handleSubmit"
            style="margin-top: 8px;"
          >
            {{ isSetup ? '设置密码' : '登录' }}
          </n-button>

          <n-alert v-if="errorMessage" type="error" :title="errorMessage" style="margin-top: 16px;" />
        </n-form>
      </n-spin>

      <p class="footer-text">{{ isSetup ? '首次使用，请设置访问密码' : '首次登录后需要配置 Cloudflare R2 存储' }}</p>
      <div class="footer-links">
        <a class="github-link" href="https://github.com/Today-ddr/r2box" target="_blank">
          <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
          GitHub
        </a>
        <span class="version-tag">v{{ appVersion }}</span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import api from '../services/api'
import { NForm, NFormItem, NInput, NButton, NAlert, NSpin } from 'naive-ui'

// Version injected by Vite at build time
const appVersion = __APP_VERSION__

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const formValue = ref({
  password: '',
  confirmPassword: ''
})

const isSetup = ref(false)
const checking = ref(true)
const loading = ref(false)
const errorMessage = ref('')

const rules = computed(() => {
  const baseRules = {
    password: {
      required: true,
      message: isSetup.value ? '请设置密码' : '请输入密码',
      trigger: 'blur'
    }
  }

  if (isSetup.value) {
    baseRules.confirmPassword = {
      required: true,
      message: '请确认密码',
      trigger: 'blur'
    }
  }

  return baseRules
})

onMounted(async () => {
  try {
    const status = await api.getPasswordStatus()
    isSetup.value = !status.password_set
  } catch (error) {
    console.error('检查密码状态失败:', error)
  } finally {
    checking.value = false
  }
})

const handleSubmit = async () => {
  try {
    await formRef.value?.validate()

    if (isSetup.value && formValue.value.password !== formValue.value.confirmPassword) {
      errorMessage.value = '两次输入的密码不一致'
      return
    }


    loading.value = true
    errorMessage.value = ''

    let result
    if (isSetup.value) {
      result = await api.setupPassword(formValue.value.password)
      if (result.success) {
        // 设置密码成功后，更新 authStore 状态
        const hash = await authStore.hashPassword(formValue.value.password)
        authStore.isAuthenticated = true
        authStore.token = hash
        localStorage.setItem('auth_token', hash)
      }
    } else {
      result = await authStore.login(formValue.value.password)
    }

    if (result.success) {
      if (result.need_setup || result.needSetup) {
        router.push('/setup')
      } else {
        router.push('/')
      }
    } else {
      errorMessage.value = result.message || '操作失败'
    }
  } catch (error) {
    errorMessage.value = error.response?.data?.error || '操作失败'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 100vh;
  background: #fafafa;
  background-image:
    radial-gradient(circle at 25% 25%, rgba(0, 112, 243, 0.03) 0%, transparent 50%),
    radial-gradient(circle at 75% 75%, rgba(0, 112, 243, 0.03) 0%, transparent 50%);
}

.login-card {
  width: 100%;
  max-width: 400px;
  padding: 48px 40px;
  background: #fff;
  border-radius: 24px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
  border: 1px solid #eaeaea;
}

.logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.logo-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
}

.logo-text {
  font-size: 36px;
  font-weight: 700;
  color: #111;
}

.subtitle {
  text-align: center;
  color: #666;
  margin-top: 8px;
  font-size: 14px;
}

.footer-text {
  text-align: center;
  color: #999;
  font-size: 12px;
  margin-top: 24px;
}

.footer-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  margin-top: 12px;
}

.github-link {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #666;
  font-size: 12px;
  text-decoration: none;
}

.github-link:hover {
  color: #111;
}

.version-tag {
  font-size: 11px;
  color: #999;
  background: #f5f5f5;
  padding: 2px 8px;
  border-radius: 10px;
  font-family: 'SF Mono', Monaco, 'Courier New', monospace;
}
</style>
