import { defineStore } from 'pinia'
import { computed, shallowRef } from 'vue'

import { decodeJwtPayload, isJwtExpired } from '@/utils/jwt'
import type { AdministratorLoginResponse } from '@/types/auth'

const adminSessionStorageKey = 'courseforge.administrator-session'

export interface AdministratorSessionSnapshot {
  accessToken: string
  administratorId: number
  username: string
}

function emptySession(): AdministratorSessionSnapshot {
  return { accessToken: '', administratorId: 0, username: '' }
}

function loadSession(): AdministratorSessionSnapshot {
  if (typeof window === 'undefined') return emptySession()
  try {
    const raw = window.sessionStorage.getItem(adminSessionStorageKey)
    if (!raw) return emptySession()
    const parsed = JSON.parse(raw) as Partial<AdministratorSessionSnapshot>
    return {
      accessToken: parsed.accessToken?.trim() ?? '',
      administratorId: Number(parsed.administratorId) || 0,
      username: parsed.username?.trim() ?? '',
    }
  } catch {
    return emptySession()
  }
}

function persistSession(snapshot: AdministratorSessionSnapshot): void {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(adminSessionStorageKey, JSON.stringify(snapshot))
}

export function readStoredAdministratorAccessToken(): string {
  return loadSession().accessToken
}

export const useAdministratorSessionStore = defineStore('administrator-session', () => {
  const initial = loadSession()
  const accessToken = shallowRef(initial.accessToken)
  const administratorId = shallowRef(initial.administratorId)
  const username = shallowRef(initial.username)

  const isAuthenticated = computed(() => {
    if (!accessToken.value || administratorId.value <= 0 || isJwtExpired(accessToken.value)) {
      return false
    }
    try {
      const payload = decodeJwtPayload(accessToken.value)
      return payload.actor_type === 'administrator' && Number(payload.sub) === administratorId.value
    } catch {
      return false
    }
  })

  function establish(input: AdministratorLoginResponse): void {
    const token = input.access_token.trim()
    const payload = decodeJwtPayload(token)
    const parsedAdministratorId = Number(payload.sub)
    if (payload.actor_type !== 'administrator') {
      throw new Error('登录凭证不是管理员身份')
    }
    if (!Number.isSafeInteger(parsedAdministratorId) || parsedAdministratorId <= 0) {
      throw new Error('登录凭证中缺少有效的管理员身份')
    }
    if (isJwtExpired(token)) {
      throw new Error('登录已过期，请重新登录')
    }
    if (parsedAdministratorId !== input.administrator.id) {
      throw new Error('登录身份信息不一致')
    }

    accessToken.value = token
    administratorId.value = parsedAdministratorId
    username.value = input.administrator.username.trim()
    persistSession({
      accessToken: accessToken.value,
      administratorId: administratorId.value,
      username: username.value,
    })
  }

  function disconnect(): void {
    accessToken.value = ''
    administratorId.value = 0
    username.value = ''
    if (typeof window !== 'undefined') {
      window.sessionStorage.removeItem(adminSessionStorageKey)
    }
  }

  return {
    accessToken,
    administratorId,
    username,
    isAuthenticated,
    establish,
    disconnect,
  }
})
