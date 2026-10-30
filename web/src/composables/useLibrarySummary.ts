import { useI18n } from 'vue-i18n'
import type { ItemKind, Library } from '@/api/libraries'

const shownKinds: Record<Library['kind'], ItemKind[]> = {
  movies: ['movie'],
  series: ['series', 'episode'],
  other: ['video'],
}

/** Formats item counts as "12 séries · 340 épisodes". */
export function useLibrarySummary() {
  const { t } = useI18n()
  return (lib: Library) =>
    shownKinds[lib.kind].map((k) => t(`libraries.counts.${k}`, lib.counts?.[k] ?? 0)).join(' · ')
}
