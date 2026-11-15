<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from '@/components/BaseButton.vue'
import BaseDialog from '@/components/BaseDialog.vue'
import ErrorAlert from '@/components/ErrorAlert.vue'
import TextField from '@/components/TextField.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'

/** Edits the alias an admin gives a user or a paired server; an empty alias brings the original name back. */
const props = defineProps<{ name: string; alias: string | null; save: (alias: string | null) => Promise<void> }>()
const open = defineModel<boolean>('open', { required: true })

const { t } = useI18n()
const errorMessage = useErrorMessage()
const value = ref('')
const error = ref('')
const saving = ref(false)

watch(open, (isOpen) => {
  if (!isOpen) return
  value.value = props.alias ?? ''
  error.value = ''
})

async function submit() {
  saving.value = true
  error.value = ''
  try {
    await props.save(value.value.trim() || null)
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('aliases.title', { name })">
    <form id="alias-form" class="space-y-5" @submit.prevent="submit">
      <TextField v-model="value" :label="t('aliases.label')" :hint="t('aliases.hint', { name })" autocomplete="off" :maxlength="64" />
      <ErrorAlert v-if="error" :message="error" />
    </form>
    <template #footer>
      <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" form="alias-form" variant="primary" :loading="saving">{{ t('common.save') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
