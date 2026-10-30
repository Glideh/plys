import { api, type User } from './client'

export type Role = User['role']

export const usersApi = {
  list: () => api<{ items: User[] }>('/users').then((r) => r.items),
  create: (body: { username: string; password: string; role: Role; locale: User['locale'] }) =>
    api<User>('/users', { method: 'POST', body }),
  update: (id: string, body: { role?: Role; locale?: User['locale']; password?: string }) =>
    api<User>(`/users/${id}`, { method: 'PATCH', body }),
  remove: (id: string) => api<void>(`/users/${id}`, { method: 'DELETE' }),
}
