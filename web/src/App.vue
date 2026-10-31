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
  <!-- Moving between videos must restart the player, which a same-route navigation would reuse. -->
  <RouterView v-slot="{ Component, route: current }">
    <component :is="Component" :key="current.name === 'watch' ? String(current.params.id) : undefined" />
  </RouterView>
</template>
