<script setup lang="ts">
import { Plus } from '@lucide/vue'
import BaseButton from './BaseButton.vue'

/** The primary "add" button of the admin pages: same look for libraries, users and servers. */
defineProps<{ label: string }>()
</script>

<template>
  <BaseButton variant="primary">
    <Plus class="-ml-1 size-4" :stroke-width="2.5" aria-hidden="true" />
    {{ label }}
  </BaseButton>
</template>
