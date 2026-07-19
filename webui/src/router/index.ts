import { createRouter, createWebHashHistory } from 'vue-router'
import { applicationBasePath } from '@/lib/base-url'
import DashboardView from '@/views/DashboardView.vue'

const router = createRouter({
  history: createWebHashHistory(applicationBasePath()),
  routes: [
    {
      path: '/',
      name: 'dashboard',
      component: DashboardView,
    },
    {
      path: '/recordings',
      name: 'recordings',
      component: () => import('@/views/RecordingsView.vue'),
    },
    {
      path: '/live-control',
      name: 'live-control',
      component: () => import('@/views/LiveControlView.vue'),
    },
    {
      path: '/config',
      name: 'config',
      component: () => import('@/views/ConfigView.vue'),
    },
    {
      path: '/system',
      name: 'system',
      component: () => import('@/views/SystemView.vue'),
    },
  ],
})

export default router
