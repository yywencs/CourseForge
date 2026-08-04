import type { AxiosResponse } from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { listDanmakuSegment, publishDanmaku } from './danmaku'
import { http } from './http'
import type { ApiEnvelope } from '@/types/api'
import type { Danmaku } from '@/types/danmaku'

function okResponse<T>(data: T): AxiosResponse<ApiEnvelope<T>> {
  return {
    data: { code: 0, info: 'success', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} } as AxiosResponse['config'],
  }
}

describe('danmaku API contract', () => {
  afterEach(() => vi.restoreAllMocks())

  it('publishes a message at the supplied video position', async () => {
    const request = {
      client_msg_id: 'ec40a0ec-572c-4af5-9067-65f702fa666c',
      video_time_ms: 12_345,
      content: '这里讲得很清楚',
    }
    const response: Danmaku = {
      id: 9,
      video_id: 7,
      student_id: 1001,
      ...request,
      status: 'visible',
      create_time: '2026-08-04T10:00:00Z',
    }
    const post = vi.spyOn(http, 'post').mockResolvedValue(okResponse(response))

    await expect(publishDanmaku(7, request)).resolves.toEqual(response)
    expect(post).toHaveBeenCalledWith('/api/v1/course-videos/7/danmakus', request)
  })

  it('loads a fixed history segment', async () => {
    const response = {
      segment_index: 3,
      start_ms: 120_000,
      end_ms: 180_000,
      items: [{
        id: 9,
        video_time_ms: 125_000,
        content: '重点来了',
        create_time: '2026-08-04T10:00:00Z',
      }],
    }
    const get = vi.spyOn(http, 'get').mockResolvedValue(okResponse(response))

    await expect(listDanmakuSegment(7, 3)).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/api/v1/course-videos/7/danmakus', {
      params: { segment_index: 3 },
    })
  })
})
