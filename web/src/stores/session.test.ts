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
      student_id: studentId,
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

    store.connect({
      accessToken: token,
      studentName: '林知夏',
      studentNo: '2026001001',
      termId: 1,
      roundId: 2,
    })

    expect(store.isAuthenticated).toBe(true)
    expect(store.studentId).toBe(10001)
    expect(store.context).toEqual({ termId: 1, roundId: 2 })
    expect(readStoredAccessToken()).toBe(token)
  })

  it('rejects an expired token', () => {
    const store = useSessionStore()

    expect(() =>
      store.connect({
        accessToken: testToken('10001', -10),
        studentName: '',
        studentNo: '',
        termId: 1,
        roundId: 1,
      }),
    ).toThrow('JWT 已过期')
  })
})
