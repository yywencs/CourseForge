export type ApplicationState =
  | 'created'
  | 'reserved'
  | 'selected'
  | 'rejected'
  | 'cancelled'

export type EnrollmentState = 'enrolled' | 'dropped' | 'completed'

export type WaitlistState =
  | 'waiting'
  | 'promoting'
  | 'promoted'
  | 'cancelled'

export interface FailureReason {
  code: string
  message: string
}

export interface TeachingClassSummary {
  id: number
  roundId: number
  courseId: number
  courseCode: string
  courseName: string
  teacherName: string
  credits: number
  schedule: string
  location: string
  capacity: number
  selectedCount: number
  tags: string[]
  introduction: string
  hasVideo: boolean
  videoId?: number
  videoUrl?: string
  dayOfWeek?: number
  startSection?: number
  endSection?: number
}

export interface SelectCourseRequest {
  request_id: string
  round_id: number
  teaching_class_id: number
}

export interface SelectionReceipt {
  application_id: string
  state: ApplicationState
  broker_confirmed: boolean
  mysql_persisted: boolean
}

export interface SelectionApplication {
  application_id: string
  request_id: string
  round_id: number
  term_id: number
  course_id: number
  teaching_class_id: number
  credits: string
  state: ApplicationState
  failure?: FailureReason
  applied_at: string
  completed_at?: string
  broker_confirmed: boolean
  mysql_persisted: boolean
}

export interface StudentEnrollment {
  enrollment_id: string
  application_id: string
  round_id: number
  term_id: number
  course_id: number
  teaching_class_id: number
  credits: string
  state: EnrollmentState
  enrolled_at: string
  dropped_at?: string
}

export interface DropEnrollmentReceipt {
  enrollment_id: string
  state: EnrollmentState
  mysql_persisted: boolean
  redis_released: boolean
}

export interface JoinWaitlistRequest {
  request_id: string
  round_id: number
  teaching_class_id: number
}

export interface WaitlistEntry {
  waitlist_id: string
  request_id: string
  round_id: number
  term_id: number
  course_id: number
  teaching_class_id: number
  credits: string
  state: WaitlistState
  failure?: FailureReason
  position: number
  joined_at: string
  promoted_at?: string
  cancelled_at?: string
}

export interface PageResult<T> {
  items: T[]
  limit: number
  offset: number
  total: number
}

export interface TrackedApplication {
  applicationId: string
  requestId: string
  teachingClassId: number
  courseName: string
  courseCode: string
  state: ApplicationState
  brokerConfirmed: boolean
  mysqlPersisted: boolean
  submittedAt: string
  failure?: FailureReason
}

export interface PendingSelection {
  requestId: string
  teachingClassId: number
  roundId: number
  courseName: string
  createdAt: string
}

export interface PendingWaitlist {
  requestId: string
  teachingClassId: number
  roundId: number
  courseName: string
  createdAt: string
}
