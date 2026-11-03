import { describe, expect, it } from 'vitest'
import { avatarUrl } from '@/api/client'

describe('avatarUrl', () => {
  it('returns null when the user has no avatar', () => {
    expect(avatarUrl({ id: 'u1' })).toBeNull()
    expect(avatarUrl(null)).toBeNull()
    expect(avatarUrl(undefined)).toBeNull()
  })

  it('versions the URL with the avatar timestamp so replaced pictures are reloaded', () => {
    const at = '2024-05-01T10:00:00Z'
    expect(avatarUrl({ id: 'u1', avatarUpdatedAt: at })).toBe(`/api/v1/images/avatars/u1?v=${encodeURIComponent(at)}`)
  })
})
