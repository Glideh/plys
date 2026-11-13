<script setup lang="ts">
import { watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()

watch(
  () => auth.user,
  (user) => {
    if (!user && !route.meta.public) router.replace({ name: 'login', query: { redirect: route.fullPath } })
  },
)
</script>

<template>
  <!-- Moving between videos must restart the player, which a same-route navigation would reuse.
       The layout stays alive while the player is open so that its cached lists survive a playback;
       the cache is dropped on logout so that the next user never sees the previous one's lists. -->
  <RouterView v-slot="{ Component, route: current }">
    <KeepAlive :include="auth.user ? ['AppLayout'] : []">
      <component :is="Component" :key="current.name === 'watch' ? String(current.params.id) : undefined" />
    </KeepAlive>
  </RouterView>
</template>
