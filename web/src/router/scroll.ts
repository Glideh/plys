import type { RouteLocationNormalized, RouterScrollBehavior } from 'vue-router'

type Position = { left: number; top: number }

/** Where the window goes after a navigation: `false` keeps the current scroll. */
export function scrollTarget(to: Pick<RouteLocationNormalized, 'path'>, from: Pick<RouteLocationNormalized, 'path'>, saved: Position | null): Position | false {
  // Back/forward: the position the page had when it was left.
  if (saved) return saved
  // Same page, only the query changed (catalog search, sort or filters kept in the URL).
  if (to.path === from.path) return false
  return { left: 0, top: 0 }
}

const RESTORE_TIMEOUT_MS = 2000

/** Resolves once the document is tall enough to scroll to `top`, or after a timeout, for pages still loading. */
function whenScrollable(top: number): Promise<void> {
  return new Promise((resolve) => {
    const started = performance.now()
    const check = () => {
      const reachable = document.documentElement.scrollHeight - window.innerHeight >= top
      if (reachable || performance.now() - started > RESTORE_TIMEOUT_MS) resolve()
      else requestAnimationFrame(check)
    }
    check()
  })
}

export const scrollBehavior: RouterScrollBehavior = async (to, from, savedPosition) => {
  const saved = savedPosition && 'top' in savedPosition ? { left: savedPosition.left ?? 0, top: savedPosition.top ?? 0 } : null
  const target = scrollTarget(to, from, saved)
  if (target && target.top > 0) await whenScrollable(target.top)
  return target
}
