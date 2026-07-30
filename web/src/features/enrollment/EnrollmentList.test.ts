import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import EnrollmentList from '@/features/enrollment/EnrollmentList.vue'
import type { StudentEnrollment } from '@/types/enrollment'

function record(state: StudentEnrollment['state']): StudentEnrollment {
  return {
    enrollment_id: `enrollment-${state}`,
    application_id: 'application-1',
    round_id: 1,
    term_id: 1,
    course_id: 10001,
    teaching_class_id: 20001,
    credits: '3.5',
    state,
    enrolled_at: '2026-07-30T10:00:00Z',
    dropped_at: state === 'dropped' ? '2026-07-30T11:00:00Z' : undefined,
  }
}

describe('EnrollmentList', () => {
  it('emits a drop request only for an active enrollment', async () => {
    const enrolled = record('enrolled')
    const wrapper = mount(EnrollmentList, {
      props: { items: [enrolled, record('dropped')] },
    })

    expect(wrapper.findAll('button')).toHaveLength(1)
    await wrapper.get('button').trigger('click')

    expect(wrapper.emitted('drop')).toEqual([[enrolled]])
  })
})
