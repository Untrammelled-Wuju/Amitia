import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '@/views/HomeView.vue'
import { useAuthStore } from '@/stores/authStore'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/LoginView.vue'),
      meta: { title: '登录' },
    },
    {
      path: '/',
      name: 'home',
      component: HomeView,
      meta: { requiresAuth: true },
      children: [
        {
          path: '',
          name: '工作台',
          component: () => import('@/views/pages/workbench/workbench.vue'),
          meta: { title: '工作台', requiresAuth: true },
        },
        {
          path: '/desktop',
          name: '桌面端更新',
          component: () => import('@/views/ReleaseManagementView.vue'),
          props: { product: 'desktop' },
          meta: { title: '桌面端更新', icon: 'Monitor', requiresAuth: true },
        },
        {
          path: '/android',
          name: '手机端更新',
          component: () => import('@/views/ReleaseManagementView.vue'),
          props: { product: 'android' },
          meta: { title: '手机端更新', icon: 'Cellphone', requiresAuth: true },
        },
        {
          path: '/audit',
          name: '发布审计',
          component: () => import('@/views/AuditLogsView.vue'),
          meta: { title: '发布审计', icon: 'Document', requiresAuth: true },
        },
        {
          path: '/feedback',
          name: '用户反馈',
          component: () => import('@/views/UserFeedbackView.vue'),
          meta: { title: '用户反馈', icon: 'ChatDotSquare', requiresAuth: true },
        },
        {
          path: '/settings',
          name: '发布设置',
          component: () => import('@/views/SettingsView.vue'),
          meta: { title: '发布设置', icon: 'Setting', requiresAuth: true },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      redirect: '/',
    },
  ],
})

router.beforeEach(async (to) => {
  const authStore = useAuthStore()
  await authStore.initializeAuth()
  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && authStore.isAuthenticated) {
    return { name: '工作台' }
  }
  return true
})

export default router
