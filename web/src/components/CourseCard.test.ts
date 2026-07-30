import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import CourseCard from '@/components/CourseCard.vue'
import type { TeachingClassSummary } from '@/types/enrollment'

function course(selectedCount = 3): TeachingClassSummary {
  return {
    id: 20001,
    roundId: 1,
    courseId: 10001,
    courseCode: 'CS-304',
    courseName: '分布式系统设计',
    teacherName: '周屿',
    credits: 3.5,
    schedule: '周二 3–4 节',
    location: '格物楼',
    capacity: 10,
    selectedCount,
    tags: ['专业核心'],
    introduction: '课程介绍',
    hasVideo: false,
  }
}

describe('CourseCard', () => {
  it('submits a normal selection when seats remain', async () => {
    const target = course()
    const wrapper = mount(CourseCard, { props: { course: target } })

    await wrapper.get('[data-testid="course-primary-action"]').trigger('click')

    expect(wrapper.emitted('select')).toEqual([[target]])
    expect(wrapper.emitted('joinWaitlist')).toBeUndefined()
  })

  it('joins the waitlist when the class is full', async () => {
    const target = course(10)
    const wrapper = mount(CourseCard, { props: { course: target } })

    expect(wrapper.text()).toContain('加入候补')
    await wrapper.get('[data-testid="course-primary-action"]').trigger('click')

    expect(wrapper.emitted('joinWaitlist')).toEqual([[target]])
    expect(wrapper.emitted('select')).toBeUndefined()
  })

  it('disables repeat actions for an already selected class', () => {
    const wrapper = mount(CourseCard, {
      props: { course: course(), selected: true },
    })

    expect(
      wrapper.get<HTMLButtonElement>('[data-testid="course-primary-action"]')
        .element.disabled,
    ).toBe(true)
    expect(wrapper.text()).toContain('已在选课记录中')
  })
})
