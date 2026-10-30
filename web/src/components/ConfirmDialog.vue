<script setup lang="ts">
import { ref } from 'vue'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

const props = defineProps<{ title: string; message: string; confirmLabel: string; action: () => Promise<unknown> }>()
const open = defineModel<boolean>('open', { required: true })
const errorMessage = useErrorMessage()

const loading = ref(false)
const error = ref('')

async function confirm() {
  loading.value = true
  error.value = ''
  try {
    await props.action()
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="title" @update:open="error = ''">
    <p class="text-sm leading-relaxed text-slate-300">{{ message }}</p>
    <ErrorAlert v-if="error" class="mt-4" :message="error" />
    <template #footer>
      <BaseButton @click="open = false">{{ $t('common.cancel') }}</BaseButton>
      <BaseButton variant="danger" :loading="loading" @click="confirm">{{ confirmLabel }}</BaseButton>
    </template>
  </BaseDialog>
</template>
