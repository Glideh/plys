<script setup lang="ts">
defineProps<{ message: string }>()
</script>

<template>
  <p role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">{{ message }}</p>
</template>
