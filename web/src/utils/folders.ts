/** Number of folders the picker renders at once; "show more" adds another page. */
export const FOLDER_PAGE_SIZE = 100

function fold(value: string): string {
  return value.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

/** Keeps the entries whose name contains the query, ignoring case and accents. An empty query keeps everything. */
export function filterFolders<T extends { name: string }>(entries: T[], query: string): T[] {
  const needle = fold(query.trim())
  if (!needle) return entries
  return entries.filter((e) => fold(e.name).includes(needle))
}
