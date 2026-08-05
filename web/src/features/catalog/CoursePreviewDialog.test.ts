import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CoursePreviewDialog from './CoursePreviewDialog.vue'
import { listDanmakuSegment, publishDanmaku } from '@/api/danmaku'
import type { HistoricalDanmaku } from '@/types/danmaku'

const realtimeMock = vi.hoisted(() => ({
  options: undefined as undefined | { onDanmaku: (item: HistoricalDanmaku) => void },
  start: vi.fn(),
  stop: vi.fn(),
}))

vi.mock('@/api/danmaku', () => ({
  listDanmakuSegment: vi.fn(),
  publishDanmaku: vi.fn(),
}))

vi.mock('@/utils/requestId', () => ({
  createRequestId: () => 'ec40a0ec-572c-4af5-9067-65f702fa666c',
}))

vi.mock('@/features/catalog/danmakuRealtime', () => ({
  createDanmakuRealtimeClient: vi.fn((options) => {
    realtimeMock.options = options
    return { start: realtimeMock.start, stop: realtimeMock.stop }
  }),
}))

const course = {
  id: 3,
  roundId: 2,
  courseId: 5,
  courseCode: 'CS-101',
  courseName: '程序设计',
  teacherName: '林老师',
  credits: 3,
  schedule: '周一 1–2 节',
  location: '教学楼 A101',
  capacity: 30,
  selectedCount: 10,
  tags: [],
  introduction: '课程介绍',
  hasVideo: true,
  videoId: 7,
  videoUrl: 'https://objects.example/preview.mp4?signature=test',
}

describe('CoursePreviewDialog danmaku composer', () => {
  beforeEach(() => {
    window.sessionStorage.clear()
    realtimeMock.options = undefined
    realtimeMock.start.mockReset()
    realtimeMock.stop.mockReset()
    vi.mocked(listDanmakuSegment).mockReset()
    vi.mocked(listDanmakuSegment).mockImplementation(async (_videoID, segmentIndex) => ({
      segment_index: segmentIndex,
      start_ms: (segmentIndex - 1) * 60_000,
      end_ms: segmentIndex * 60_000,
      items: [],
    }))
    vi.mocked(publishDanmaku).mockReset()
  })

  it('publishes at the current playback position', async () => {
    vi.mocked(publishDanmaku).mockResolvedValue({
      id: 9,
      video_id: 7,
      student_id: 1001,
      client_msg_id: 'ec40a0ec-572c-4af5-9067-65f702fa666c',
      video_time_ms: 12_345,
      content: '这里讲得很清楚',
      status: 'visible',
      create_time: '2026-08-04T10:00:00Z',
    })
    const wrapper = mount(CoursePreviewDialog, {
      props: { modelValue: false, course },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    const player = wrapper.get('video').element
    player.currentTime = 12.345
    await wrapper.get('#danmaku-content').setValue(' 这里讲得很清楚 ')

    await wrapper.get('.danmaku-composer').trigger('submit')
    await vi.mocked(publishDanmaku).mock.results[0]?.value
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(publishDanmaku).toHaveBeenCalledTimes(1)

    expect(publishDanmaku).toHaveBeenCalledWith(7, {
      client_msg_id: 'ec40a0ec-572c-4af5-9067-65f702fa666c',
      video_time_ms: 12_345,
      content: '这里讲得很清楚',
    })
  })

  it('reuses the same idempotency request when retrying a failed send', async () => {
    vi.mocked(publishDanmaku)
      .mockRejectedValueOnce(new Error('网络中断'))
      .mockResolvedValueOnce({
        id: 10,
        video_id: 7,
        student_id: 1001,
        client_msg_id: 'ec40a0ec-572c-4af5-9067-65f702fa666c',
        video_time_ms: 8_000,
        content: '重试这条弹幕',
        status: 'visible',
        create_time: '2026-08-04T10:00:00Z',
      })
    const wrapper = mount(CoursePreviewDialog, {
      props: { modelValue: false, course },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    wrapper.get('video').element.currentTime = 8
    await wrapper.get('#danmaku-content').setValue('重试这条弹幕')

    await wrapper.get('.danmaku-composer').trigger('submit')
    await expect(vi.mocked(publishDanmaku).mock.results[0]?.value).rejects.toThrow('网络中断')
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(publishDanmaku).toHaveBeenCalledTimes(1)
    expect(wrapper.get('#danmaku-content').element).toHaveProperty('value', '重试这条弹幕')

    wrapper.get('video').element.currentTime = 20
    await wrapper.get('.danmaku-composer').trigger('submit')
    await vi.mocked(publishDanmaku).mock.results[1]?.value
    await flushPromises()
    await wrapper.vm.$nextTick()
    expect(publishDanmaku).toHaveBeenCalledTimes(2)

    expect(vi.mocked(publishDanmaku).mock.calls[1]).toEqual(
      vi.mocked(publishDanmaku).mock.calls[0],
    )
  })

  it('loads, schedules and switches fixed history segments after seeking', async () => {
    vi.mocked(listDanmakuSegment).mockImplementation(async (_videoID, segmentIndex) => ({
      segment_index: segmentIndex,
      start_ms: (segmentIndex - 1) * 60_000,
      end_ms: segmentIndex * 60_000,
      items: segmentIndex === 1
        ? [{ id: 1, video_time_ms: 1_000, content: '第一段弹幕', create_time: '2026-08-04T10:00:00Z' }]
        : segmentIndex === 3
          ? [{ id: 3, video_time_ms: 125_200, content: '快进后的弹幕', create_time: '2026-08-04T10:01:00Z' }]
          : [],
    }))
    const wrapper = mount(CoursePreviewDialog, {
      props: { modelValue: false, course },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    const player = wrapper.get('video').element
    let currentTime = 0
    Object.defineProperty(player, 'currentTime', {
      configurable: true,
      get: () => currentTime,
      set: (value: number) => { currentTime = value },
    })

    currentTime = 0
    await wrapper.get('video').trigger('loadedmetadata')
    await flushPromises()
    expect(listDanmakuSegment).toHaveBeenCalledWith(7, 1)

    currentTime = 1.1
    await wrapper.get('video').trigger('timeupdate')
    expect(wrapper.get('.danmaku-item').text()).toBe('第一段弹幕')

    currentTime = 125
    await wrapper.get('video').trigger('seeked')
    await flushPromises()
    expect(listDanmakuSegment).toHaveBeenCalledWith(7, 3)
    expect(listDanmakuSegment).toHaveBeenCalledWith(7, 4)

    currentTime = 125.3
    await wrapper.get('video').trigger('timeupdate')
    await wrapper.vm.$nextTick()
    await vi.waitFor(() => expect(wrapper.get('.danmaku-item').text()).toBe('快进后的弹幕'))
    wrapper.unmount()
  })

  it('starts realtime reception and displays a danmaku near the current position', async () => {
    window.sessionStorage.setItem('courseforge.student-session', JSON.stringify({
      accessToken: 'student-token',
      studentId: 1001,
    }))
    const wrapper = mount(CoursePreviewDialog, {
      props: { modelValue: false, course },
      global: { stubs: { teleport: true } },
    })

    await wrapper.setProps({ modelValue: true })
    await flushPromises()
    expect(realtimeMock.start).toHaveBeenCalledOnce()

    wrapper.get('video').element.currentTime = 12.5
    realtimeMock.options?.onDanmaku({
      id: 21,
      video_time_ms: 12_345,
      content: '刚刚收到的实时弹幕',
      create_time: '2026-08-05T08:01:00.000Z',
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.get('.danmaku-item').text()).toBe('刚刚收到的实时弹幕')
    await wrapper.setProps({ modelValue: false })
    await flushPromises()
    expect(realtimeMock.stop).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('pauses and resumes active danmaku animations with video playback', async () => {
    const wrapper = mount(CoursePreviewDialog, {
      props: { modelValue: false, course },
      global: { stubs: { teleport: true } },
    })
    await wrapper.setProps({ modelValue: true })
    await flushPromises()

    const layer = wrapper.get('.danmaku-layer')
    const player = wrapper.get('video').element
    expect(layer.classes()).toContain('is-paused')

    await player.play()
    await wrapper.vm.$nextTick()
    expect(wrapper.get('.danmaku-layer').classes()).not.toContain('is-paused')

    player.pause()
    await wrapper.vm.$nextTick()
    expect(wrapper.get('.danmaku-layer').classes()).toContain('is-paused')

    await player.play()
    await wrapper.vm.$nextTick()
    expect(wrapper.get('.danmaku-layer').classes()).not.toContain('is-paused')
    wrapper.unmount()
  })
})
