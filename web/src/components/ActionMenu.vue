<script setup lang="ts">
import { Ellipsis } from '@lucide/vue'
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseButton from './BaseButton.vue'

export interface ActionMenuItem {
  label: string
  action: () => unknown
  danger?: boolean
  disabled?: boolean
}

/** A "⋯" button opening the secondary actions of a row, so rows keep a single visible button at most. */
defineProps<{ items: ActionMenuItem[]; label?: string }>()
const { t } = useI18n()

const open = ref(false)
const root = ref<HTMLElement | null>(null)

function closeOnOutsideClick(event: PointerEvent) {
  if (root.value && !root.value.contains(event.target as Node)) open.value = false
}

function closeOnEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') open.value = false
}

function run(item: ActionMenuItem) {
  open.value = false
  item.action()
}

onMounted(() => {
  document.addEventListener('pointerdown', closeOnOutsideClick)
  document.addEventListener('keydown', closeOnEscape)
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', closeOnOutsideClick)
  document.removeEventListener('keydown', closeOnEscape)
})
</script>

<template>
  <div ref="root" class="relative">
    <BaseButton
      class="!px-2.5"
      aria-haspopup="menu"
      :aria-expanded="open"
      :aria-label="label ?? t('common.actions')"
      :title="label ?? t('common.actions')"
      @click="open = !open"
    >
      <Ellipsis class="size-5" aria-hidden="true" />
    </BaseButton>
    <div v-if="open" role="menu" class="absolute right-0 z-20 mt-2 w-56 rounded-lg bg-surface-800 p-1 shadow-lg ring-1 ring-white/10">
      <button
        v-for="item in items"
        :key="item.label"
        type="button"
        role="menuitem"
        :disabled="item.disabled"
        :class="item.danger ? 'text-red-300 hover:bg-red-500/15' : 'text-slate-300 hover:bg-white/5 hover:text-slate-100'"
        class="block w-full rounded-md px-3 py-2 text-left text-sm transition disabled:cursor-not-allowed disabled:opacity-50"
        @click="run(item)"
      >
        {{ item.label }}
      </button>
    </div>
  </div>
</template>
