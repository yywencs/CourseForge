export interface Course {
  id: number
  course_code: string
  course_name: string
  credits: number
  introduction: string
  tags: readonly string[]
  create_time?: string
  update_time?: string
}

export interface ClassSchedule {
  id?: number
  teaching_class_id?: number
  day_of_week: number
  start_week: number
  end_week: number
  start_section: number
  end_section: number
}

export type TeachingClassState = 'planned' | 'open' | 'closed' | 'cancelled'

export interface TeachingClass {
  id: number
  class_code: string
  term_id: number
  course_id: number
  course_code: string
  course_name: string
  credits: number
  introduction: string
  tags: readonly string[]
  teacher_name: string
  location: string
  capacity: number
  selected_count: number
  minimum_grade_year?: number
  maximum_grade_year?: number
  state: TeachingClassState
  schedules: readonly ClassSchedule[]
  create_time?: string
  update_time?: string
}

export type SelectionRoundState = 'planned' | 'open' | 'closed'

export interface SelectionRound {
  id: number
  term_id: number
  round_code: string
  round_name: string
  start_time: string
  end_time: string
  state: SelectionRoundState
  class_count: number
  create_time?: string
  update_time?: string
}

export interface RoundClassBinding {
  id: number
  round_id: number
  teaching_class_id: number
  class_code: string
  course_name: string
  state: 'open' | 'closed'
  create_time: string
}

export type CourseDraft = Omit<Course, 'id' | 'create_time' | 'update_time'>
export type TeachingClassDraft = Pick<
  TeachingClass,
  | 'class_code'
  | 'term_id'
  | 'course_id'
  | 'teacher_name'
  | 'location'
  | 'capacity'
  | 'minimum_grade_year'
  | 'maximum_grade_year'
  | 'schedules'
>
export type SelectionRoundDraft = Pick<
  SelectionRound,
  'term_id' | 'round_code' | 'round_name' | 'start_time' | 'end_time'
>
