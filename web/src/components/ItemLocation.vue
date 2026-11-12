<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

/** Where the files of an item sit on the server: the common folder, then each file relative to it. */
const props = defineProps<{ folder?: string; paths: string[] }>()
const { t } = useI18n()

const files = computed(() => {
  const prefix = props.folder ? props.folder.replace(/\/$/, '') + '/' : ''
  return props.paths.map((p) => (prefix && p.startsWith(prefix) ? p.slice(prefix.length) : p))
})
</script>

<template>
  <dl v-if="folder || files.length" class="grid gap-x-6 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
    <template v-if="folder">
      <dt class="text-slate-500">{{ t('catalog.folder') }}</dt>
      <dd class="select-all break-all font-mono text-xs leading-5 text-slate-300">{{ folder }}</dd>
    </template>
    <template v-if="files.length">
      <dt class="text-slate-500">{{ t('catalog.fileName', files.length) }}</dt>
      <dd class="space-y-1">
        <p v-for="f in files" :key="f" class="select-all break-all font-mono text-xs leading-5 text-slate-300">{{ f }}</p>
      </dd>
    </template>
  </dl>
</template>
