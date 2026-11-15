<script setup lang="ts">
import { computed } from 'vue'
import { encodeQr } from '@/utils/qr'

const props = defineProps<{ value: string; label: string }>()

// A 4-module light margin around the symbol, as scanners expect.
const QUIET = 4

const qr = computed(() => encodeQr(props.value))
const path = computed(() => {
  const parts: string[] = []
  qr.value.modules.forEach((row, y) =>
    row.forEach((dark, x) => {
      if (dark) parts.push(`M${x + QUIET} ${y + QUIET}h1v1h-1z`)
    }),
  )
  return parts.join('')
})
const extent = computed(() => qr.value.size + QUIET * 2)
</script>

<template>
  <svg :viewBox="`0 0 ${extent} ${extent}`" role="img" :aria-label="label" shape-rendering="crispEdges" class="rounded-lg">
    <rect :width="extent" :height="extent" fill="#fff" />
    <path :d="path" fill="#000" />
  </svg>
</template>
