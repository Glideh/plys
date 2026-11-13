<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/** Says that some servers of the federation were left out of an aggregated answer, and which ones when known. */
const props = defineProps<{ count: number; names?: string[] }>()
const { t } = useI18n()

const message = computed(() => {
  const names = props.names ?? []
  if (!names.length) return t('filters.partial', props.count)
  return t('filters.partialNamed', { n: props.count, names: names.join(', ') }, props.count)
})
</script>

<template>
  <p v-if="count" class="text-sm text-amber-300" role="status">{{ message }}</p>
</template>
