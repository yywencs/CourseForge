import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { timestampDate } from '@bufbuild/protobuf/wkt'

import {
  AuthenticateRequestSchema,
  ClientFrameSchema,
  RealtimeErrorCode,
  ServerFrameSchema,
} from '@/gen/courseforge/danmaku/v1/danmaku_pb'
import type { HistoricalDanmaku } from '@/types/danmaku'
import { createRequestId } from '@/utils/requestId'

const reconnectDelaysMS = [1_000, 2_000, 4_000] as const

export type DanmakuRealtimeStatus =
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'unavailable'

interface WebSocketConnection {
  binaryType: BinaryType
  readyState: number
  onopen: ((event: Event) => void) | null
  onmessage: ((event: MessageEvent) => void) | null
  onerror: ((event: Event) => void) | null
  onclose: ((event: CloseEvent) => void) | null
  send(data: ArrayBufferView | ArrayBuffer | Blob | string): void
  close(code?: number, reason?: string): void
}

export interface DanmakuRealtimeClient {
  start(): void
  stop(): void
}

export interface DanmakuRealtimeOptions {
  videoId: number
  accessToken: string
  onDanmaku: (item: HistoricalDanmaku) => void
  onStatus?: (status: DanmakuRealtimeStatus) => void
  onError?: (message: string) => void
  url?: string
  webSocketFactory?: (url: string) => WebSocketConnection
  requestIdFactory?: () => string
}

// createDanmakuRealtimeClient 创建一个只负责当前视频的实时弹幕连接。
export function createDanmakuRealtimeClient(
  options: DanmakuRealtimeOptions,
): DanmakuRealtimeClient {
  return new ReconnectingDanmakuClient(options)
}

// buildDanmakuRealtimeURL 根据 HTTP API 地址推导同一服务的 WebSocket 地址。
export function buildDanmakuRealtimeURL(
  videoId: number,
  apiBaseURL = import.meta.env.VITE_API_BASE_URL || window.location.origin,
): string {
  const base = new URL(apiBaseURL || window.location.origin, window.location.origin)
  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  base.pathname = `/api/v1/course-videos/${videoId}/danmakus/realtime`
  base.search = ''
  base.hash = ''
  return base.toString()
}

class ReconnectingDanmakuClient implements DanmakuRealtimeClient {
  private readonly options: DanmakuRealtimeOptions
  private readonly url: string
  private readonly webSocketFactory: (url: string) => WebSocketConnection
  private readonly requestIdFactory: () => string
  private socket?: WebSocketConnection
  private reconnectTimer?: number
  private reconnectAttempt = 0
  private stopped = true

  constructor(options: DanmakuRealtimeOptions) {
    this.options = options
    this.url = options.url ?? buildDanmakuRealtimeURL(options.videoId)
    this.webSocketFactory = options.webSocketFactory ?? ((url) => new WebSocket(url))
    this.requestIdFactory = options.requestIdFactory ?? createRequestId
  }

  start(): void {
    if (!this.stopped) return
    this.stopped = false
    this.reconnectAttempt = 0
    this.connect()
  }

  stop(): void {
    this.stopped = true
    if (this.reconnectTimer !== undefined) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = undefined
    }
    const socket = this.socket
    this.socket = undefined
    if (socket && socket.readyState < 2) {
      try {
        socket.close(1000, 'video preview closed')
      } catch {
        // CONNECTING 状态下部分实现会拒绝 close，后续 close 事件仍不会触发重连。
      }
    }
  }

  private connect(): void {
    if (this.stopped) return
    this.options.onStatus?.(this.reconnectAttempt === 0 ? 'connecting' : 'reconnecting')

    let socket: WebSocketConnection
    try {
      socket = this.webSocketFactory(this.url)
    } catch {
      this.scheduleReconnect()
      return
    }
    this.socket = socket
    socket.binaryType = 'arraybuffer'
    socket.onopen = () => this.authenticate(socket)
    socket.onmessage = (event) => { void this.handleMessage(socket, event.data) }
    socket.onerror = () => {
      // 浏览器不会暴露具体握手错误，统一等待 close 后执行重连。
    }
    socket.onclose = () => {
      if (this.socket === socket) this.socket = undefined
      if (!this.stopped) this.scheduleReconnect()
    }
  }

  private authenticate(socket: WebSocketConnection): void {
    if (this.socket !== socket || this.stopped) return
    const authenticate = create(AuthenticateRequestSchema, {
      accessToken: this.options.accessToken,
    })
    const frame = create(ClientFrameSchema, {
      requestId: this.requestIdFactory(),
      payload: { case: 'authenticate', value: authenticate },
    })
    try {
      socket.send(toBinary(ClientFrameSchema, frame))
    } catch {
      try {
        socket.close()
      } catch {
        if (this.socket === socket) this.socket = undefined
        this.scheduleReconnect()
      }
    }
  }

  private async handleMessage(socket: WebSocketConnection, data: unknown): Promise<void> {
    if (this.socket !== socket || this.stopped) return
    let bytes: Uint8Array
    if (data instanceof ArrayBuffer) {
      bytes = new Uint8Array(data)
    } else if (data instanceof Blob) {
      bytes = new Uint8Array(await data.arrayBuffer())
    } else if (ArrayBuffer.isView(data)) {
      bytes = new Uint8Array(data.buffer, data.byteOffset, data.byteLength)
    } else {
      this.options.onError?.('实时弹幕返回了不支持的数据格式')
      return
    }
    if (this.socket !== socket || this.stopped) return

    let frame
    try {
      frame = fromBinary(ServerFrameSchema, bytes)
    } catch {
      this.options.onError?.('实时弹幕消息解析失败')
      return
    }
    switch (frame.payload.case) {
      case 'connectionReady':
        if (toSafeNumber(frame.payload.value.videoId) !== this.options.videoId) {
          this.failPermanently(socket, '实时弹幕连接的视频不匹配')
          return
        }
        this.reconnectAttempt = 0
        this.options.onStatus?.('connected')
        return
      case 'danmakuPublished': {
        const published = frame.payload.value
        const id = toSafeNumber(published.id)
        const videoId = toSafeNumber(published.videoId)
        const videoTimeMS = toSafeNumber(published.videoTimeMs)
        if (id === undefined || videoId !== this.options.videoId ||
          videoTimeMS === undefined || !published.createTime) {
          this.options.onError?.('实时弹幕包含无效字段')
          return
        }
        this.options.onDanmaku({
          id,
          video_time_ms: videoTimeMS,
          content: published.content,
          create_time: timestampDate(published.createTime).toISOString(),
        })
        return
      }
      case 'error': {
        const failure = frame.payload.value
        if (!failure.retryable || failure.code === RealtimeErrorCode.UNAUTHENTICATED ||
          failure.code === RealtimeErrorCode.FORBIDDEN ||
          failure.code === RealtimeErrorCode.VIDEO_NOT_PLAYABLE) {
          this.failPermanently(socket, failure.message || '实时弹幕连接不可用')
        } else {
          this.options.onError?.(failure.message || '实时弹幕服务返回错误')
          socket.close()
        }
        return
      }
      default:
        this.options.onError?.('实时弹幕返回了未知消息')
    }
  }

  private failPermanently(socket: WebSocketConnection, message: string): void {
    this.stopped = true
    this.options.onError?.(message)
    this.options.onStatus?.('unavailable')
    try {
      socket.close(1008, 'realtime unavailable')
    } catch {
      // 连接可能已经被服务端关闭，不影响永久停止重连。
    }
  }

  private scheduleReconnect(): void {
    if (this.stopped || this.reconnectTimer !== undefined) return
    const delay = reconnectDelaysMS[this.reconnectAttempt]
    if (delay === undefined) {
      this.stopped = true
      this.options.onStatus?.('unavailable')
      return
    }
    this.reconnectAttempt += 1
    this.options.onStatus?.('reconnecting')
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = undefined
      this.connect()
    }, delay)
  }
}

function toSafeNumber(value: bigint): number | undefined {
  if (value < 0n || value > BigInt(Number.MAX_SAFE_INTEGER)) return undefined
  return Number(value)
}
