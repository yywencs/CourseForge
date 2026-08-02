export interface JwtPayload {
  sub?: string | number
  actor_type?: string
  iss?: string
  aud?: string | string[]
  iat?: number
  exp?: number
  [key: string]: unknown
}

function decodeBase64Url(value: string): string {
  const normalized = value.replaceAll('-', '+').replaceAll('_', '/')
  const padding = '='.repeat((4 - (normalized.length % 4)) % 4)
  return decodeURIComponent(
    Array.from(atob(normalized + padding))
      .map((character) => `%${character.charCodeAt(0).toString(16).padStart(2, '0')}`)
      .join(''),
  )
}

export function decodeJwtPayload(token: string): JwtPayload {
  const parts = token.trim().split('.')
  if (parts.length !== 3 || !parts[1]) {
    throw new Error('登录凭证格式不正确')
  }
  try {
    return JSON.parse(decodeBase64Url(parts[1])) as JwtPayload
  } catch {
    throw new Error('登录凭证无法解析')
  }
}

export function isJwtExpired(token: string, now = Date.now()): boolean {
  try {
    const payload = decodeJwtPayload(token)
    return typeof payload.exp === 'number' && payload.exp * 1000 <= now
  } catch {
    return true
  }
}
