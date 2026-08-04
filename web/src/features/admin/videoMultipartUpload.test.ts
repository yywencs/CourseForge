import { describe, expect, it, vi } from 'vitest'

import {
  missingPartNumbers,
  uploadPartWithRetry,
} from '@/features/admin/videoMultipartUpload'
import type { VideoUploadPartTicket } from '@/types/catalog'

const part: VideoUploadPartTicket = {
  part_number: 2,
  upload_url: 'https://objects.example/part-2',
  method: 'PUT',
}

describe('video multipart upload retries', () => {
  it('retries a transient failure three times with exponential backoff', async () => {
    const fetcher = vi.fn()
      .mockResolvedValueOnce(response(500))
      .mockRejectedValueOnce(new Error('network unavailable'))
      .mockResolvedValueOnce(response(429))
      .mockResolvedValueOnce(response(200))
    const wait = vi.fn().mockResolvedValue(undefined)

    await uploadPartWithRetry(new Blob(['part']), part, {
      fetcher,
      wait,
      refreshPartURL: vi.fn(),
    })

    expect(fetcher).toHaveBeenCalledTimes(4)
    expect(wait.mock.calls.map(([milliseconds]) => milliseconds)).toEqual([500, 1000, 2000])
  })

  it('stops after three automatic retries and requires manual continuation', async () => {
    const fetcher = vi.fn().mockResolvedValue(response(503))

    await expect(uploadPartWithRetry(new Blob(['part']), part, {
      fetcher,
      wait: vi.fn().mockResolvedValue(undefined),
      refreshPartURL: vi.fn(),
    })).rejects.toThrow('请点击继续上传')

    expect(fetcher).toHaveBeenCalledTimes(4)
  })

  it('refreshes an expired signed URL before retrying', async () => {
    const refreshed = { ...part, upload_url: 'https://objects.example/refreshed-part-2' }
    const fetcher = vi.fn()
      .mockResolvedValueOnce(response(403))
      .mockResolvedValueOnce(response(200))
    const refreshPartURL = vi.fn().mockResolvedValue(refreshed)

    await uploadPartWithRetry(new Blob(['part']), part, {
      fetcher,
      wait: vi.fn().mockResolvedValue(undefined),
      refreshPartURL,
    })

    expect(refreshPartURL).toHaveBeenCalledWith(2)
    expect(fetcher.mock.calls[1]?.[0]).toBe(refreshed.upload_url)
  })

  it('selects only parts that OSS has not recorded', () => {
    const parts = [1, 2, 3].map((partNumber) => ({
      ...part,
      part_number: partNumber,
    }))

    expect(missingPartNumbers(parts, new Set([1, 3]))).toEqual([2])
  })
})

function response(status: number): Response {
  return { ok: status >= 200 && status < 300, status } as Response
}
