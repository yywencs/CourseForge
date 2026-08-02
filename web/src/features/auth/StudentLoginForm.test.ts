import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import StudentLoginForm from './StudentLoginForm.vue'
import { login } from '@/api/auth'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
}))

function sessionToken(): string {
  const encode = (value: object) =>
    btoa(JSON.stringify(value))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '')
  return [
    encode({ alg: 'HS256', typ: 'JWT' }),
    encode({
      sub: '10001',
      actor_type: 'student',
      exp: Math.floor(Date.now() / 1000) + 3600,
    }),
    'test-signature',
  ].join('.')
}

describe('StudentLoginForm', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(login).mockReset()
  })

  it('submits student credentials and emits an authenticated session', async () => {
    vi.mocked(login).mockResolvedValue({
      access_token: sessionToken(),
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      student: {
        id: 10001,
        student_no: '2026001001',
        student_name: '林知夏',
      },
      selection_context: { term_id: 1, round_id: 2 },
    })
    const wrapper = mount(StudentLoginForm, {
      global: { plugins: [createPinia()] },
    })

    await wrapper.get('input[autocomplete="username"]').setValue('2026001001')
    await wrapper
      .get('input[autocomplete="current-password"]')
      .setValue('correct-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(login).toHaveBeenCalledWith({
      student_no: '2026001001',
      password: 'correct-password',
    })
    expect(wrapper.emitted('authenticated')).toHaveLength(1)
  })
})
