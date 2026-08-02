import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import {
  readStoredAdministratorAccessToken,
  useAdministratorSessionStore,
} from '@/stores/adminSession'

function testToken(actorType: string, identity: string, expiresInSeconds = 3600): string {
  const encode = (value: object) =>
    btoa(JSON.stringify(value))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '')
  return [
    encode({ alg: 'HS256', typ: 'JWT' }),
    encode({
      sub: identity,
      actor_type: actorType,
      exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
    }),
    'test-signature',
  ].join('.')
}

describe('administrator session store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('establishes and persists an administrator session', () => {
    const store = useAdministratorSessionStore()
    const token = testToken('administrator', '30001')

    store.establish({
      access_token: token,
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      administrator: { id: 30001, username: 'admin' },
    })

    expect(store.isAuthenticated).toBe(true)
    expect(store.administratorId).toBe(30001)
    expect(store.username).toBe('admin')
    expect(readStoredAdministratorAccessToken()).toBe(token)
  })

  it('rejects a student token returned as an administrator session', () => {
    const store = useAdministratorSessionStore()

    expect(() => store.establish({
      access_token: testToken('student', '30001'),
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      administrator: { id: 30001, username: 'admin' },
    })).toThrow('登录凭证不是管理员身份')
  })

  it('removes the administrator session on disconnect', () => {
    const store = useAdministratorSessionStore()
    store.establish({
      access_token: testToken('administrator', '30001'),
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      administrator: { id: 30001, username: 'admin' },
    })

    store.disconnect()

    expect(store.isAuthenticated).toBe(false)
    expect(readStoredAdministratorAccessToken()).toBe('')
  })
})
