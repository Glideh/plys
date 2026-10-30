<script setup lang="ts">
import { useId } from 'vue'

defineProps<{
  label: string
  type?: string
  autocomplete?: string
  hint?: string
  required?: boolean
  minlength?: number
  maxlength?: number
}>()

const model = defineModel<string>({ required: true })
const id = useId()
</script>

<template>
  <div>
    <label :for="id" class="mb-1.5 block text-sm font-medium text-slate-300">{{ label }}</label>
    <input
      :id="id"
      v-model="model"
      :type="type ?? 'text'"
      :autocomplete="autocomplete"
      :required="required"
      :minlength="minlength"
      :maxlength="maxlength"
      :aria-describedby="hint ? `${id}-hint` : undefined"
      class="block w-full rounded-lg border border-white/10 bg-surface-800 px-3.5 py-2.5 text-slate-100 placeholder-slate-500 transition focus:border-accent-500 focus:ring-2 focus:ring-accent-500/30 focus:outline-none"
    />
    <p v-if="hint" :id="`${id}-hint`" class="mt-1.5 text-xs text-slate-500">{{ hint }}</p>
  </div>
</template>
