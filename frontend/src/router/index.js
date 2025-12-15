import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/Login.vue')
  },
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('../views/Setup.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/',
    name: 'Upload',
    component: () => import('../views/Upload.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/files',
    name: 'Files',
    component: () => import('../views/Files.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/stats',
    name: 'Stats',
    component: () => import('../views/Stats.vue'),
    meta: { requiresAuth: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth) {
    if (!authStore.isAuthenticated) {
      // 尝试验证 token
      const isValid = await authStore.checkAuth()
      if (!isValid) {
        next('/login')
        return
      }
    }

    // 如果需要配置 R2 且不是去配置页面，重定向到配置页面
    if (authStore.needSetup && to.path !== '/setup') {
      next('/setup')
      return
    }

    // 允许已配置的用户访问 setup 页面（用于修改配置）
    // 不再重定向
  }

  next()
})

export default router
