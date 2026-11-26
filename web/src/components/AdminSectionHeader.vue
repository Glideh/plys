<script setup lang="ts">
/** Title, help text and actions shared by the admin pages, so every "add" button sits in the same place. */
defineProps<{ title: string; help?: string }>()
</script>

<template>
  <div class="mb-6 flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
    <div class="min-w-0 flex-1 basis-64">
      <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
        <h2 class="text-lg font-semibold">{{ title }}</h2>
        <slot name="status" />
      </div>
      <p v-if="help" class="mt-1 text-sm leading-relaxed text-slate-400">{{ help }}</p>
    </div>
    <div v-if="$slots.actions" class="flex flex-wrap gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
