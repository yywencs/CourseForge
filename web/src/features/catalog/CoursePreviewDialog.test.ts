import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import CoursePreviewDialog from './CoursePreviewDialog.vue'
import { publishDanmaku } from '@/api/danmaku'

vi.mock('@/api/danmaku', () => ({
  publishDanmaku: vi.fn(),
}))

vi.mock('@/utils/requestId', () => ({
  createRequestId: () => 'ec40a0ec-572c-4af5-9067-65f702fa666c',
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
      .mockResolvedValueOnce({} as never)
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
})
