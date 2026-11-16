<script setup lang="ts">
defineProps<{ title: string; subtitle: string }>()
</script>

<template>
  <main class="relative flex min-h-full items-center justify-center overflow-hidden px-4 py-12">
    <div
      aria-hidden="true"
      class="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top,color-mix(in_oklch,var(--color-accent-500)_22%,transparent),transparent_55%),radial-gradient(ellipse_at_bottom_right,rgba(99,102,241,0.12),transparent_50%)]"
    />
    <div class="relative w-full max-w-md">
      <div class="mb-8 flex items-center justify-center gap-3">
        <span class="grid size-11 place-items-center rounded-xl bg-accent-500 text-xl font-black text-surface-950 shadow-lg shadow-accent-500/30">
          ▶
        </span>
        <span class="text-2xl font-bold tracking-tight">{{ $t('app.name') }}</span>
      </div>
      <section class="rounded-2xl border border-white/5 bg-surface-900/80 p-8 shadow-2xl shadow-black/50 backdrop-blur">
        <h1 class="text-2xl font-semibold">{{ title }}</h1>
        <p class="mt-2 text-sm leading-relaxed text-slate-400">{{ subtitle }}</p>
        <div class="mt-8">
          <slot />
        </div>
      </section>
    </div>
  </main>
</template>
