import { http, unwrap } from '@/api/http'
import type {
  Danmaku,
  DanmakuSegment,
  PublishDanmakuRequest,
} from '@/types/danmaku'

export function listDanmakuSegment(
  videoId: number,
  segmentIndex: number,
): Promise<DanmakuSegment> {
  return unwrap(http.get(`/api/v1/course-videos/${videoId}/danmakus`, {
    params: { segment_index: segmentIndex },
  }))
}

export function publishDanmaku(
  videoId: number,
  request: PublishDanmakuRequest,
): Promise<Danmaku> {
  return unwrap(http.post(`/api/v1/course-videos/${videoId}/danmakus`, request))
}
