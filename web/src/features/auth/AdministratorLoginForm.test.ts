import { createPinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdministratorLoginForm from './AdministratorLoginForm.vue'
import { loginAdministrator } from '@/api/adminAuth'

vi.mock('@/api/adminAuth', () => ({
  loginAdministrator: vi.fn(),
}))

function administratorToken(): string {
  const encode = (value: object) =>
    btoa(JSON.stringify(value))
      .replaceAll('+', '-')
      .replaceAll('/', '_')
      .replaceAll('=', '')
  return [
    encode({ alg: 'HS256', typ: 'JWT' }),
    encode({
      sub: '30001',
      actor_type: 'administrator',
      exp: Math.floor(Date.now() / 1000) + 3600,
    }),
    'test-signature',
  ].join('.')
}

describe('AdministratorLoginForm', () => {
  beforeEach(() => {
    vi.mocked(loginAdministrator).mockReset()
  })

  it('submits administrator credentials and emits an authenticated session', async () => {
    vi.mocked(loginAdministrator).mockResolvedValue({
      access_token: administratorToken(),
      token_type: 'Bearer',
      expires_at: new Date(Date.now() + 3_600_000).toISOString(),
      administrator: { id: 30001, username: 'admin' },
    })
    const wrapper = mount(AdministratorLoginForm, {
      global: { plugins: [createPinia()] },
    })

    await wrapper.get('input[autocomplete="username"]').setValue('admin')
    await wrapper
      .get('input[autocomplete="current-password"]')
      .setValue('correct-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(loginAdministrator).toHaveBeenCalledWith({
      username: 'admin',
      password: 'correct-password',
    })
    expect(wrapper.emitted('authenticated')).toHaveLength(1)
  })

  it('shows the authentication error without emitting success', async () => {
    vi.mocked(loginAdministrator).mockRejectedValue(new Error('用户名或密码错误'))
    const wrapper = mount(AdministratorLoginForm, {
      global: { plugins: [createPinia()] },
    })

    await wrapper.get('input[autocomplete="username"]').setValue('admin')
    await wrapper.get('input[autocomplete="current-password"]').setValue('wrong-password')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('[role="alert"]').text()).toBe('用户名或密码错误')
    expect(wrapper.emitted('authenticated')).toBeUndefined()
  })
})
