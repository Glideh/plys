import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, refreshSession, setAccessToken } from './client'

const session = { accessToken: 'new-token', tokenType: 'Bearer', expiresAt: '', user: { id: '1' } }

function json(status: number, body?: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), { status })
}

describe('api client', () => {
  const fetchMock = vi.fn<typeof fetch>()

  beforeEach(() => {
    vi.stubGlobal('fetch', fetchMock)
    setAccessToken('old-token')
  })

  afterEach(() => {
    fetchMock.mockReset()
    vi.unstubAllGlobals()
  })

  it('refreshes once and retries after a 401', async () => {
    fetchMock
      .mockResolvedValueOnce(json(401))
      .mockResolvedValueOnce(json(200, session))
      .mockResolvedValueOnce(json(200, { ok: true }))

    await expect(api('/me')).resolves.toEqual({ ok: true })
    const retried = fetchMock.mock.calls[2]![1]!.headers as Record<string, string>
    expect(retried.Authorization).toBe('Bearer new-token')
  })

  it('shares a single in-flight refresh between concurrent callers', async () => {
    fetchMock.mockResolvedValue(json(200, session))
    const [a, b] = await Promise.all([refreshSession(), refreshSession()])
    expect(a).toEqual(session)
    expect(b).toBe(a)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('maps error payloads to ApiError', async () => {
    fetchMock.mockResolvedValueOnce(json(409, { error: { code: 'username_taken', message: 'taken' } }))
    const err = await api('/users', { method: 'POST', body: {} }).catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 409, code: 'username_taken' })
  })

  it('does not refresh for unauthenticated endpoints', async () => {
    fetchMock.mockResolvedValueOnce(json(401, { error: { code: 'invalid_credentials', message: 'no' } }))
    await expect(api('/auth/login', { method: 'POST', body: {}, auth: false })).rejects.toMatchObject({
      code: 'invalid_credentials',
    })
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})
