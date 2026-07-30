export interface JwtPayload {
  student_id?: string | number
  user_id?: string | number
  sub?: string | number
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

function encodeBase64Url(value: Uint8Array): string {
  let binary = ''
  value.forEach((byte) => {
    binary += String.fromCharCode(byte)
  })
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replaceAll('=', '')
}

function encodeJson(value: object): string {
  return encodeBase64Url(new TextEncoder().encode(JSON.stringify(value)))
}

export function decodeJwtPayload(token: string): JwtPayload {
  const parts = token.trim().split('.')
  if (parts.length !== 3 || !parts[1]) {
    throw new Error('JWT 格式不正确')
  }
  try {
    return JSON.parse(decodeBase64Url(parts[1])) as JwtPayload
  } catch {
    throw new Error('JWT 载荷无法解析')
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

export async function createDevelopmentJwt(input: {
  studentId: number
  signingKey: string
  issuer: string
  audience: string
  ttlSeconds?: number
}): Promise<string> {
  if (!import.meta.env.DEV) {
    throw new Error('开发令牌只能在 Vite 开发模式生成')
  }
  if (
    !Number.isSafeInteger(input.studentId) ||
    input.studentId <= 0 ||
    input.signingKey.length < 32
  ) {
    throw new Error('学生 ID 必须有效，开发签名密钥至少需要 32 个字符')
  }
  const now = Math.floor(Date.now() / 1000)
  const header = encodeJson({ alg: 'HS256', typ: 'JWT' })
  const payload = encodeJson({
    student_id: String(input.studentId),
    sub: String(input.studentId),
    iss: input.issuer,
    aud: input.audience,
    iat: now,
    exp: now + (input.ttlSeconds ?? 7200),
  })
  const content = `${header}.${payload}`
  const key = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(input.signingKey),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign'],
  )
  const signature = await crypto.subtle.sign(
    'HMAC',
    key,
    new TextEncoder().encode(content),
  )
  return `${content}.${encodeBase64Url(new Uint8Array(signature))}`
}
