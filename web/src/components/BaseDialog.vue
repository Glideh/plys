<script setup lang="ts">
import { useTemplateRef, watch } from 'vue'

defineProps<{ title: string; wide?: boolean }>()
const open = defineModel<boolean>('open', { required: true })
const dialog = useTemplateRef<HTMLDialogElement>('dialog')

watch(
  [open, dialog],
  ([isOpen, el]) => {
    if (!el) return
    if (isOpen && !el.open) el.showModal()
    if (!isOpen && el.open) el.close()
  },
  { immediate: true, flush: 'post' },
)
</script>

<template>
  <dialog
    ref="dialog"
    :class="wide ? 'max-w-2xl' : 'max-w-md'"
    class="m-auto w-[calc(100%-2rem)] rounded-2xl border border-white/10 bg-surface-900 p-0 text-slate-100 shadow-2xl shadow-black/60 backdrop:bg-black/70 backdrop:backdrop-blur-sm"
    @close="open = false"
  >
    <div v-if="open" class="flex max-h-[85vh] flex-col">
      <header class="flex items-center justify-between border-b border-white/5 px-6 py-4">
        <h2 class="text-lg font-semibold">{{ title }}</h2>
        <button
          type="button"
          class="grid size-8 place-items-center rounded-lg text-slate-400 transition hover:bg-white/5 hover:text-slate-100"
          :aria-label="$t('common.close')"
          @click="open = false"
        >
          <svg viewBox="0 0 20 20" fill="currentColor" class="size-5" aria-hidden="true">
            <path d="M6.28 5.22a.75.75 0 0 0-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 1 0 1.06 1.06L10 11.06l3.72 3.72a.75.75 0 1 0 1.06-1.06L11.06 10l3.72-3.72a.75.75 0 0 0-1.06-1.06L10 8.94 6.28 5.22Z" />
          </svg>
        </button>
      </header>
      <div class="overflow-y-auto px-6 py-5">
        <slot />
      </div>
      <footer v-if="$slots.footer" class="flex justify-end gap-3 border-t border-white/5 px-6 py-4">
        <slot name="footer" />
      </footer>
    </div>
  </dialog>
</template>
