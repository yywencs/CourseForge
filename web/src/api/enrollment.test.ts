import type { AxiosResponse } from 'axios'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  cancelWaitlist,
  dropEnrollment,
  joinWaitlist,
  listMyEnrollments,
  listMyWaitlist,
  queryApplication,
  queryWaitlist,
  selectCourse,
} from './enrollment'
import { http } from './http'
import type { ApiEnvelope } from '@/types/api'

function okResponse<T>(data: T): AxiosResponse<ApiEnvelope<T>> {
  return {
    data: { code: 0, info: 'success', data },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: { headers: {} } as AxiosResponse['config'],
  }
}

describe('enrollment API contract', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('submits a course selection', async () => {
    const request = {
      request_id: 'request-1',
      round_id: 9,
      teaching_class_id: 101,
    }
    const receipt = {
      application_id: 'application-1',
      state: 'created' as const,
      stream_recorded: false,
      mysql_persisted: false,
    }
    const post = vi.spyOn(http, 'post').mockResolvedValue(okResponse(receipt))

    await expect(selectCourse(request)).resolves.toEqual(receipt)
    expect(post).toHaveBeenCalledWith('/api/v1/enrollments', request)
  })

  it('queries an application', async () => {
    const get = vi.spyOn(http, 'get').mockResolvedValue(okResponse({}))

    await queryApplication('application-1')
    expect(get).toHaveBeenCalledWith(
      '/api/v1/enrollments/applications/application-1',
    )
  })

  it('lists the current student enrollments with pagination', async () => {
    const get = vi
      .spyOn(http, 'get')
      .mockResolvedValue(okResponse({ items: [], limit: 20, offset: 40, total: 0 }))

    await listMyEnrollments(202601, 20, 40)
    expect(get).toHaveBeenCalledWith('/api/v1/enrollments/me', {
      params: { term_id: 202601, limit: 20, offset: 40 },
    })
  })

  it('drops an enrollment', async () => {
    const remove = vi.spyOn(http, 'delete').mockResolvedValue(okResponse({}))

    await dropEnrollment('enrollment-1')
    expect(remove).toHaveBeenCalledWith('/api/v1/enrollments/enrollment-1')
  })

  it('joins the waitlist', async () => {
    const request = {
      request_id: 'request-2',
      round_id: 9,
      teaching_class_id: 102,
    }
    const post = vi.spyOn(http, 'post').mockResolvedValue(okResponse({}))

    await joinWaitlist(request)
    expect(post).toHaveBeenCalledWith(
      '/api/v1/enrollments/waitlist',
      request,
    )
  })

  it('queries one waitlist entry', async () => {
    const get = vi.spyOn(http, 'get').mockResolvedValue(okResponse({}))

    await queryWaitlist('waitlist-1')
    expect(get).toHaveBeenCalledWith('/api/v1/enrollments/waitlist/waitlist-1')
  })

  it('lists the current student waitlist with pagination', async () => {
    const get = vi
      .spyOn(http, 'get')
      .mockResolvedValue(okResponse({ items: [], limit: 30, offset: 10, total: 0 }))

    await listMyWaitlist(202601, 30, 10)
    expect(get).toHaveBeenCalledWith('/api/v1/enrollments/waitlist/me', {
      params: { term_id: 202601, limit: 30, offset: 10 },
    })
  })

  it('cancels a waitlist entry', async () => {
    const remove = vi.spyOn(http, 'delete').mockResolvedValue(okResponse({}))

    await cancelWaitlist('waitlist-1')
    expect(remove).toHaveBeenCalledWith(
      '/api/v1/enrollments/waitlist/waitlist-1',
    )
  })
})
