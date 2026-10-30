import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { safeRedirect } from './redirect'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    admin?: boolean
  }
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/setup', name: 'setup', component: () => import('@/views/SetupView.vue'), meta: { public: true } },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    {
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      children: [
        { path: '', name: 'home', component: () => import('@/views/HomeView.vue') },
        {
          path: 'admin',
          component: () => import('@/layouts/AdminLayout.vue'),
          meta: { admin: true },
          children: [
            { path: '', redirect: { name: 'admin-libraries' } },
            { path: 'libraries', name: 'admin-libraries', component: () => import('@/views/admin/AdminLibrariesView.vue') },
            { path: 'users', name: 'admin-users', component: () => import('@/views/admin/AdminUsersView.vue') },
          ],
        },
      ],
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  const auth = useAuthStore()
  try {
    await auth.init()
  } catch {
    // API unreachable: the login view displays the error on submit.
    return to.name === 'login' ? true : { name: 'login' }
  }

  if (auth.needsSetup) return to.name === 'setup' ? true : { name: 'setup' }
  if (to.name === 'setup') return { name: auth.user ? 'home' : 'login' }
  if (!auth.user && !to.meta.public) return { name: 'login', query: { redirect: to.fullPath } }
  if (auth.user && to.name === 'login') return safeRedirect(to.query.redirect)
  if (to.meta.admin && !auth.isAdmin) return { name: 'home' }
  return true
})
