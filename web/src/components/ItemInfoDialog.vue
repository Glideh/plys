<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'
import ItemLocation from './ItemLocation.vue'

/** Extra details about an item, kept out of the item page. Only admins receive the file locations. */
defineProps<{ folder?: string; paths: string[] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('catalog.details')" wide>
    <h3 class="mb-3 text-sm font-semibold text-slate-200">{{ t('catalog.location') }}</h3>
    <ItemLocation :folder="folder" :paths="paths" />
  </BaseDialog>
</template>
