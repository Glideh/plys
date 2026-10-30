<script setup lang="ts">
withDefaults(
  defineProps<{ variant?: 'primary' | 'secondary' | 'danger'; type?: 'button' | 'submit'; loading?: boolean; disabled?: boolean }>(),
  { variant: 'secondary', type: 'button' },
)

const variants = {
  primary: 'bg-accent-500 text-surface-950 hover:bg-accent-400 shadow-lg shadow-accent-500/20',
  secondary: 'bg-white/5 text-slate-200 hover:bg-white/10',
  danger: 'bg-red-500/90 text-white hover:bg-red-500',
}
</script>

<template>
  <button
    :type="type"
    :disabled="loading || disabled"
    :aria-busy="loading"
    :class="variants[variant]"
    class="inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50"
  >
    <span v-if="loading" class="size-4 animate-spin rounded-full border-2 border-current/30 border-t-current" aria-hidden="true" />
    <slot />
  </button>
</template>
