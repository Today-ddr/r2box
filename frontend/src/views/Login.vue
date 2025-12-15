<template>
  <div class="login-container">
    <div class="login-card">
      <div class="logo">
        <span class="logo-icon">📦</span>
        <span class="logo-text">R2Box</span>
      </div>
      <p class="subtitle">轻量级临时文件分享</p>

      <n-form ref="formRef" :model="formValue" :rules="rules" style="margin-top: 32px;">
        <n-form-item label="访问口令" path="token">
          <n-input
            v-model:value="formValue.token"
            type="password"
            placeholder="请输入访问口令"
            show-password-on="click"
            size="large"
            @keyup.enter="handleLogin"
          />
        </n-form-item>

        <n-button
          type="primary"
          block
          size="large"
          :loading="loading"
          @click="handleLogin"
          style="margin-top: 8px;"
        >
          登录
        </n-button>

        <n-alert v-if="errorMessage" type="error" :title="errorMessage" style="margin-top: 16px;" />
      </n-form>

      <p class="footer-text">首次登录后需要配置 Cloudflare R2 存储</p>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { NForm, NFormItem, NInput, NButton, NAlert } from 'naive-ui'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const formValue = ref({
  token: ''
})

const rules = {
  token: {
    required: true,
    message: '请输入访问口令',
    trigger: 'blur'
  }
}

const loading = ref(false)
const errorMessage = ref('')

const handleLogin = async () => {
  try {
    await formRef.value?.validate()
    loading.value = true
    errorMessage.value = ''

    const result = await authStore.login(formValue.value.token)

    if (result.success) {
      if (result.needSetup) {
        router.push('/setup')
      } else {
        router.push('/')
      }
    } else {
      errorMessage.value = result.message || '登录失败'
    }
  } catch (error) {
    console.error('登录错误:', error)
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
  font-size: 48px;
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
</style>
