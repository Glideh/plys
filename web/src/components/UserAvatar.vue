<script setup lang="ts">
import { computed } from 'vue'
import { avatarUrl, type User } from '@/api/client'

const props = defineProps<{ user: Pick<User, 'id' | 'displayName' | 'avatarUpdatedAt'> | null | undefined }>()

const src = computed(() => avatarUrl(props.user))
const initial = computed(() => props.user?.displayName.charAt(0) ?? '')
</script>

<template>
  <span class="grid place-items-center overflow-hidden rounded-full bg-surface-700 font-semibold uppercase">
    <img v-if="src" :src="src" alt="" class="size-full object-cover" />
    <template v-else>{{ initial }}</template>
  </span>
</template>
