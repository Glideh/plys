<script setup lang="ts">
defineProps<{ loading?: boolean }>()
</script>

<template>
  <button
    type="submit"
    :disabled="loading"
    class="flex w-full items-center justify-center gap-2 rounded-lg bg-accent-500 px-4 py-2.5 font-semibold text-surface-950 shadow-lg shadow-accent-500/20 transition hover:bg-accent-400 disabled:cursor-wait disabled:opacity-70"
  >
    <span v-if="loading" class="size-4 animate-spin rounded-full border-2 border-surface-950/30 border-t-surface-950" aria-hidden="true" />
    <slot />
  </button>
</template>
