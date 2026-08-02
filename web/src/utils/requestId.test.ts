import { describe, expect, it, vi } from 'vitest'

import { createRequestId } from './requestId'

describe('createRequestId', () => {
  it('uses the native UUID generator when it is available', () => {
    const nativeId = '018f4f14-8123-7abc-8123-0123456789ab'
    const getRandomValues = vi.fn((bytes: Uint8Array) => bytes)

    expect(createRequestId({
      randomUUID: () => nativeId,
      getRandomValues,
    })).toBe(nativeId)
    expect(getRandomValues).not.toHaveBeenCalled()
  })

  it('creates an RFC 4122 version 4 UUID without randomUUID', () => {
    const getRandomValues = (bytes: Uint8Array): Uint8Array => {
      bytes.forEach((_, index) => {
        bytes[index] = index
      })
      return bytes
    }

    expect(createRequestId({ getRandomValues })).toBe(
      '00010203-0405-4607-8809-0a0b0c0d0e0f',
    )
  })
})
