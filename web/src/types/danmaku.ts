export type DanmakuStatus = 'visible' | 'hidden' | 'deleted'

export interface PublishDanmakuRequest {
  client_msg_id: string
  video_time_ms: number
  content: string
}

export interface Danmaku {
  id: number
  video_id: number
  student_id: number
  client_msg_id: string
  video_time_ms: number
  content: string
  status: DanmakuStatus
  create_time: string
}
