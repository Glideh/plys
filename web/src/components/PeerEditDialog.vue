<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { peersApi, type Peer } from '@/api/peers'
import { useErrorMessage } from '@/composables/useErrorMessage'
import BaseButton from './BaseButton.vue'
import BaseDialog from './BaseDialog.vue'
import ErrorAlert from './ErrorAlert.vue'
import TextField from './TextField.vue'

/** Edits the alias and the public URL of a paired server. */
const props = defineProps<{ peer: Peer | null }>()
const emit = defineEmits<{ saved: [] }>()
const open = defineModel<boolean>('open', { required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const alias = ref('')
const url = ref('')
const error = ref('')
const saving = ref(false)

watch(open, (isOpen) => {
  if (!isOpen || !props.peer) return
  alias.value = props.peer.alias ?? ''
  url.value = props.peer.url
  error.value = ''
})

async function submit() {
  const peer = props.peer
  if (!peer) return
  saving.value = true
  error.value = ''
  try {
    const nextUrl = url.value.trim()
    if (nextUrl !== peer.url) await peersApi.update(peer.id, nextUrl)
    const nextAlias = alias.value.trim() || null
    if (nextAlias !== (peer.alias ?? null)) await peersApi.setAlias(peer.id, nextAlias)
    emit('saved')
    open.value = false
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <BaseDialog v-model:open="open" :title="t('servers.editTitle', { name: peer?.displayName ?? '' })">
    <form v-if="peer" id="peer-edit-form" class="space-y-5" @submit.prevent="submit">
      <TextField v-model="alias" :label="t('aliases.label')" :hint="t('aliases.hint', { name: peer.name })" autocomplete="off" :maxlength="64" />
      <TextField v-model="url" :label="t('servers.urlLabel')" type="url" autocomplete="off" required />
      <ErrorAlert v-if="error" :message="error" />
    </form>
    <template #footer>
      <BaseButton @click="open = false">{{ t('common.cancel') }}</BaseButton>
      <BaseButton type="submit" form="peer-edit-form" variant="primary" :loading="saving" :disabled="!url.trim()">{{ t('common.save') }}</BaseButton>
    </template>
  </BaseDialog>
</template>
