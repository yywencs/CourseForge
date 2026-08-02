interface CryptoRandomSource {
  randomUUID?: () => string
  getRandomValues: (array: Uint8Array) => Uint8Array
}

function hex(value: number): string {
  return value.toString(16).padStart(2, '0')
}

// createRequestId 生成选课与候补请求的幂等标识，并兼容非 HTTPS 页面。
export function createRequestId(
  source: CryptoRandomSource = globalThis.crypto,
): string {
  if (typeof source.randomUUID === 'function') {
    return source.randomUUID()
  }

  const bytes = source.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const value = Array.from(bytes, hex).join('')

  return [
    value.slice(0, 8),
    value.slice(8, 12),
    value.slice(12, 16),
    value.slice(16, 20),
    value.slice(20),
  ].join('-')
}
