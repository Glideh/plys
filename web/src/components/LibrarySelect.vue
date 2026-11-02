<script setup lang="ts">
import type { Library } from '@/api/libraries'

defineProps<{ modelValue: string | null; libraries: Library[]; allLabel?: string; label: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string | null] }>()

function onChange(event: Event) {
  emit('update:modelValue', (event.target as HTMLSelectElement).value || null)
}
</script>

<template>
  <div class="inline-flex max-w-full items-center gap-2">
    <select
      :value="modelValue ?? ''"
      :aria-label="label"
      class="min-w-0 cursor-pointer appearance-none rounded-lg bg-transparent text-3xl font-bold tracking-tight text-slate-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-accent-500/40"
      @change="onChange"
    >
      <option v-if="allLabel !== undefined" value="" class="bg-surface-800">{{ allLabel }}</option>
      <option v-for="lib in libraries" :key="lib.id" :value="lib.id" class="bg-surface-800">{{ lib.name }}</option>
    </select>
  </div>
</template>
