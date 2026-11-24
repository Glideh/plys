import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { libraryKinds, type LibraryKind } from '@/api/libraries'
import { safeRedirect } from './redirect'
import { scrollBehavior } from './scroll'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    admin?: boolean
  }
}

export const router = createRouter({
  history: createWebHistory(),
  scrollBehavior,
  routes: [
    { path: '/setup', name: 'setup', component: () => import('@/views/SetupView.vue'), meta: { public: true } },
    { path: '/invite', name: 'invite', component: () => import('@/views/InviteView.vue'), meta: { public: true } },
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue'), meta: { public: true } },
    { path: '/link', name: 'link', component: () => import('@/views/LinkDeviceView.vue') },
    { path: '/setup/metadata', name: 'setup-metadata', component: () => import('@/views/SetupMetadataView.vue'), meta: { admin: true } },
    { path: '/watch/:id', name: 'watch', component: () => import('@/views/PlayerView.vue'), props: true },
    {
      path: '/',
      component: () => import('@/layouts/AppLayout.vue'),
      children: [
        { path: '', name: 'home', component: () => import('@/views/HomeView.vue') },
        { path: 'libraries/:id', name: 'library', component: () => import('@/views/LibraryView.vue'), props: true },
        {
          path: 'kinds/:kind',
          name: 'kind',
          component: () => import('@/views/KindView.vue'),
          props: true,
          beforeEnter: (to) => (libraryKinds.includes(to.params.kind as LibraryKind) ? true : { name: 'home' }),
        },
        { path: 'items/:id', name: 'item', component: () => import('@/views/ItemView.vue'), props: true },
        { path: 'profile', name: 'profile', component: () => import('@/views/ProfileView.vue') },
        {
          path: 'admin',
          component: () => import('@/layouts/AdminLayout.vue'),
          meta: { admin: true },
          children: [
            { path: '', redirect: { name: 'admin-libraries' } },
            { path: 'libraries', name: 'admin-libraries', component: () => import('@/views/admin/AdminLibrariesView.vue') },
            { path: 'users', name: 'admin-users', component: () => import('@/views/admin/AdminUsersView.vue') },
            { path: 'servers', name: 'admin-servers', component: () => import('@/views/admin/AdminServersView.vue') },
            { path: 'settings', name: 'admin-settings', component: () => import('@/views/admin/AdminSettingsView.vue') },
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
