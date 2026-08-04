import { http, unwrap } from '@/api/http'
import type { Danmaku, PublishDanmakuRequest } from '@/types/danmaku'

export function publishDanmaku(
  videoId: number,
  request: PublishDanmakuRequest,
): Promise<Danmaku> {
  return unwrap(http.post(`/api/v1/course-videos/${videoId}/danmakus`, request))
}
