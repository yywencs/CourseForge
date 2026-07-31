import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import {
  readStoredAccessToken,
  useSessionStore,
} from '@/stores/session'

function testToken(studentId: string, expiresInSeconds = 3600): string {
  const encode = (value: object) =>
    btoa(JSON.stringify(value))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '')
  return [
    encode({ alg: 'HS256', typ: 'JWT' }),
    encode({
      sub: studentId,
      exp: Math.floor(Date.now() / 1000) + expiresInSeconds,
    }),
    'test-signature',
  ].join('.')
}

describe('session store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('extracts the student identity and persists only the active session', () => {
    const store = useSessionStore()
    const token = testToken('10001')

    store.establish({
      access_token: token,
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      student: {
        id: 10001,
        student_name: '林知夏',
        student_no: '2026001001',
      },
      selection_context: { term_id: 1, round_id: 2 },
    })

    expect(store.isAuthenticated).toBe(true)
    expect(store.studentId).toBe(10001)
    expect(store.context).toEqual({ termId: 1, roundId: 2 })
    expect(readStoredAccessToken()).toBe(token)
  })

  it('rejects an expired token', () => {
    const store = useSessionStore()

    expect(() =>
      store.establish({
        access_token: testToken('10001', -10),
        token_type: 'Bearer',
        expires_at: new Date(Date.now() - 10_000).toISOString(),
        student: {
          id: 10001,
          student_name: '林知夏',
          student_no: '2026001001',
        },
        selection_context: { term_id: 1, round_id: 1 },
      }),
    ).toThrow('JWT 已过期')
  })
})
