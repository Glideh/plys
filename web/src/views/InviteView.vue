<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ApiError } from '@/api/client'
import { invitationToken, invitationsApi, type InvitationInfo } from '@/api/invitations'
import AuthCard from '@/components/AuthCard.vue'
import SubmitButton from '@/components/SubmitButton.vue'
import TextField from '@/components/TextField.vue'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { localeLabels, locales, setLocale, type Locale } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const MAX_AVATAR_BYTES = 2 * 1024 * 1024

const { t, d, locale: currentLocale } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const token = invitationToken(route.hash)
const info = ref<InvitationInfo | null>(null)
const checking = ref(true)
const invalid = ref(false)

const username = ref('')
const password = ref('')
const confirm = ref('')
const pin = ref('')
const locale = ref(currentLocale.value as Locale)
const avatar = ref<File | null>(null)
const avatarPreview = ref<string | null>(null)
const avatarInput = ref<HTMLInputElement | null>(null)
const loading = ref(false)
const error = ref('')

const subtitle = computed(() =>
  info.value?.serverName ? t('invite.subtitleNamed', { server: info.value.serverName }) : t('invite.subtitle'),
)

// The page is opened from a link holding a secret: it must not leak to other sites through the Referer header.
let referrerMeta: HTMLMetaElement | null = null

onMounted(async () => {
  referrerMeta = document.createElement('meta')
  referrerMeta.name = 'referrer'
  referrerMeta.content = 'no-referrer'
  document.head.appendChild(referrerMeta)

  if (!token) {
    invalid.value = true
    checking.value = false
    return
  }
  try {
    info.value = await invitationsApi.lookup(token)
    locale.value = info.value.locale
    setLocale(info.value.locale)
  } catch (err) {
    if (err instanceof ApiError && err.code === 'invitation_invalid') invalid.value = true
    else error.value = errorMessage(err)
  } finally {
    checking.value = false
  }
})

onUnmounted(() => {
  referrerMeta?.remove()
  if (avatarPreview.value) URL.revokeObjectURL(avatarPreview.value)
})

function onAvatarPicked(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (file.size > MAX_AVATAR_BYTES) {
    error.value = t('profile.avatarTooLarge')
    return
  }
  error.value = ''
  if (avatarPreview.value) URL.revokeObjectURL(avatarPreview.value)
  avatar.value = file
  avatarPreview.value = URL.createObjectURL(file)
}

function clearAvatar() {
  if (avatarPreview.value) URL.revokeObjectURL(avatarPreview.value)
  avatar.value = null
  avatarPreview.value = null
}

async function submit() {
  error.value = ''
  if (password.value !== confirm.value) {
    error.value = t('errors.passwordMismatch')
    return
  }
  loading.value = true
  try {
    await auth.acceptInvitation({
      token,
      pin: pin.value.trim(),
      username: username.value.trim(),
      password: password.value,
      locale: locale.value,
    })
  } catch (err) {
    if (err instanceof ApiError && err.code === 'invitation_invalid') invalid.value = true
    error.value = errorMessage(err)
    loading.value = false
    return
  }
  // The account exists from here on: a refused picture is retried from the profile.
  let next = 'home'
  if (avatar.value) {
    try {
      await auth.setAvatar(avatar.value)
    } catch {
      next = 'profile'
    }
  }
  loading.value = false
  await router.replace({ name: next })
}
</script>

<template>
  <AuthCard :title="t('invite.title')" :subtitle="subtitle">
    <div v-if="checking" class="h-40 animate-pulse rounded-xl bg-surface-800/60" />

    <div v-else-if="invalid" class="space-y-5">
      <p role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">
        {{ t('errors.invitation_invalid') }}
      </p>
      <RouterLink :to="{ name: 'login' }" class="block text-center text-sm text-accent-400 hover:underline">{{ t('invite.toLogin') }}</RouterLink>
    </div>

    <form v-else class="space-y-5" @submit.prevent="submit">
      <div>
        <label for="invite-pin" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('invite.pin') }}</label>
        <input
          id="invite-pin"
          v-model="pin"
          inputmode="numeric"
          autocomplete="one-time-code"
          pattern="[0-9]{4}"
          maxlength="4"
          required
          aria-describedby="invite-pin-hint"
          class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 font-mono text-lg tracking-[0.4em] text-slate-100 transition focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
        />
        <p id="invite-pin-hint" class="mt-1.5 text-xs text-slate-500">{{ t('invite.pinHint') }}</p>
      </div>

      <TextField v-model="username" :label="t('common.username')" :hint="t('setup.usernameHint')" autocomplete="username" required :maxlength="32" />
      <TextField
        v-model="password"
        :label="t('common.password')"
        :hint="t('setup.passwordHint')"
        type="password"
        autocomplete="new-password"
        required
        :minlength="8"
      />
      <TextField v-model="confirm" :label="t('setup.confirmPassword')" type="password" autocomplete="new-password" required />

      <div>
        <label for="invite-locale" class="mb-1.5 block text-sm font-medium text-slate-300">{{ t('common.language') }}</label>
        <select
          id="invite-locale"
          v-model="locale"
          class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
          @change="setLocale(locale)"
        >
          <option v-for="l in locales" :key="l" :value="l">{{ localeLabels[l] }}</option>
        </select>
      </div>

      <div class="flex items-center gap-4">
        <span class="grid size-14 shrink-0 place-items-center overflow-hidden rounded-full bg-surface-700 text-xl font-semibold uppercase">
          <img v-if="avatarPreview" :src="avatarPreview" alt="" class="size-full object-cover" />
          <template v-else>{{ username.trim().charAt(0) }}</template>
        </span>
        <div class="min-w-0 space-y-1">
          <p class="text-sm font-medium text-slate-300">{{ t('invite.avatar') }}</p>
          <p class="text-xs text-slate-500">{{ t('profile.avatarHelp') }}</p>
          <div class="flex gap-3 text-sm">
            <button type="button" class="text-accent-400 hover:underline" @click="avatarInput?.click()">{{ t('profile.avatarUpload') }}</button>
            <button v-if="avatar" type="button" class="text-slate-400 hover:underline" @click="clearAvatar">{{ t('profile.avatarRemove') }}</button>
          </div>
        </div>
        <input ref="avatarInput" type="file" accept="image/jpeg,image/png" class="hidden" @change="onAvatarPicked" />
      </div>

      <p v-if="info" class="text-xs text-slate-500">{{ t('invitations.expires', { date: d(new Date(info.expiresAt), 'long') }) }}</p>

      <p v-if="error" role="alert" class="rounded-lg border border-red-500/30 bg-red-500/10 px-3.5 py-2.5 text-sm text-red-300">
        {{ error }}
      </p>

      <SubmitButton :loading="loading">{{ t('invite.submit') }}</SubmitButton>
    </form>
  </AuthCard>
</template>
