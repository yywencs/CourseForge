import { create, fromBinary, toBinary } from '@bufbuild/protobuf'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  ClientFrameSchema,
  ConnectionReadySchema,
  DanmakuPublishedSchema,
  ServerFrameSchema,
} from '@/gen/courseforge/danmaku/v1/danmaku_pb'

import { buildDanmakuRealtimeURL, createDanmakuRealtimeClient } from './danmakuRealtime'

class FakeWebSocket {
  binaryType: BinaryType = 'blob'
  readyState: number = WebSocket.CONNECTING
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  readonly sent: Array<ArrayBufferView | ArrayBuffer | Blob | string> = []

  send(data: ArrayBufferView | ArrayBuffer | Blob | string): void {
    this.sent.push(data)
  }

  open(): void {
    this.readyState = WebSocket.OPEN
    this.onopen?.(new Event('open'))
  }

  receive(data: Uint8Array): void {
    this.onmessage?.({ data } as MessageEvent)
  }

  close(): void {
    if (this.readyState === WebSocket.CLOSED) return
    this.readyState = WebSocket.CLOSED
    this.onclose?.(new CloseEvent('close'))
  }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('danmaku realtime client', () => {
  it('authenticates with protobuf and converts published frames', async () => {
    const socket = new FakeWebSocket()
    const statuses: string[] = []
    const received = vi.fn()
    const client = createDanmakuRealtimeClient({
      videoId: 7,
      accessToken: 'student-token',
      onDanmaku: received,
      onStatus: (status) => statuses.push(status),
      url: 'ws://courseforge.test/realtime',
      webSocketFactory: () => socket,
      requestIdFactory: () => 'request-1',
    })

    client.start()
    socket.open()

    const authenticationBytes = socket.sent[0]
    expect(ArrayBuffer.isView(authenticationBytes)).toBe(true)
    const authentication = fromBinary(ClientFrameSchema, authenticationBytes as Uint8Array)
    expect(authentication.requestId).toBe('request-1')
    expect(authentication.payload.case).toBe('authenticate')
    if (authentication.payload.case === 'authenticate') {
      expect(authentication.payload.value.accessToken).toBe('student-token')
    }

    socket.receive(toBinary(ServerFrameSchema, create(ServerFrameSchema, {
      requestId: 'request-1',
      payload: {
        case: 'connectionReady',
        value: create(ConnectionReadySchema, {
          videoId: 7n,
          connectedAt: timestampFromDate(new Date('2026-08-05T08:00:00Z')),
        }),
      },
    })))
    socket.receive(toBinary(ServerFrameSchema, create(ServerFrameSchema, {
      payload: {
        case: 'danmakuPublished',
        value: create(DanmakuPublishedSchema, {
          id: 19n,
          videoId: 7n,
          clientMessageId: 'message-1',
          videoTimeMs: 12_345n,
          content: '实时弹幕',
          createTime: timestampFromDate(new Date('2026-08-05T08:01:00Z')),
        }),
      },
    })))
    await Promise.resolve()

    expect(statuses).toEqual(['connecting', 'connected'])
    expect(received).toHaveBeenCalledWith({
      id: 19,
      video_time_ms: 12_345,
      content: '实时弹幕',
      create_time: '2026-08-05T08:01:00.000Z',
    })
    client.stop()
  })

  it('reconnects with exponential backoff and stops after three retries', () => {
    vi.useFakeTimers()
    const sockets: FakeWebSocket[] = []
    const statuses: string[] = []
    const client = createDanmakuRealtimeClient({
      videoId: 7,
      accessToken: 'student-token',
      onDanmaku: vi.fn(),
      onStatus: (status) => statuses.push(status),
      url: 'ws://courseforge.test/realtime',
      webSocketFactory: () => {
        const socket = new FakeWebSocket()
        sockets.push(socket)
        return socket
      },
    })

    client.start()
    sockets[0]?.close()
    vi.advanceTimersByTime(1_000)
    sockets[1]?.close()
    vi.advanceTimersByTime(2_000)
    sockets[2]?.close()
    vi.advanceTimersByTime(4_000)
    sockets[3]?.close()

    expect(sockets).toHaveLength(4)
    expect(statuses.at(-1)).toBe('unavailable')
  })

  it('derives the websocket endpoint from the configured API origin', () => {
    expect(buildDanmakuRealtimeURL(7, 'https://api.courseforge.test/base')).toBe(
      'wss://api.courseforge.test/api/v1/course-videos/7/danmakus/realtime',
    )
  })
})
