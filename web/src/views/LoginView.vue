<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AuthCard from '@/components/AuthCard.vue'
import SubmitButton from '@/components/SubmitButton.vue'
import TextField from '@/components/TextField.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { safeRedirect } from '@/router/redirect'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const errorMessage = useErrorMessage()

const username = ref('')
const password = ref('')
const loading = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(username.value.trim(), password.value)
    await router.replace(safeRedirect(route.query.redirect))
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <AuthCard :title="t('login.title')" :subtitle="t('login.subtitle')">
    <form class="space-y-5" @submit.prevent="submit">
      <TextField v-model="username" :label="t('common.username')" autocomplete="username" required />
      <TextField v-model="password" :label="t('common.password')" type="password" autocomplete="current-password" required />

      <p v-if="error" role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">
        {{ error }}
      </p>

      <SubmitButton :loading="loading">{{ t('login.submit') }}</SubmitButton>
    </form>
  </AuthCard>
</template>
