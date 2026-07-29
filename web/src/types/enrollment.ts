export type EnrollmentState =
  | 'created'
  | 'reserved'
  | 'processing'
  | 'selected'
  | 'rejected'
  | 'cancelled'

export interface TeachingClassSummary {
  id: number
  roundId: number
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
}

export interface SelectCourseRequest {
  request_id: string
  round_id: number
  student_id: number
  teaching_class_id: number
  source: 'web' | 'mobile' | 'admin'
}

export interface SelectionReceipt {
  application_id: string
  state: EnrollmentState
  broker_confirmed: boolean
  mysql_persisted: boolean
}
