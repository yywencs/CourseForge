import type { TeachingClass } from '@/types/catalog'
import type { TeachingClassSummary } from '@/types/enrollment'
import { shallowRef } from 'vue'

const cachedCatalog = shallowRef<TeachingClassSummary[]>([])

const weekdays = ['一', '二', '三', '四', '五', '六', '日']

function formatSchedule(item: TeachingClass): string {
  if (!item.schedules.length) return '时间待公布'
  return item.schedules
    .map((schedule) =>
      `周${weekdays[schedule.day_of_week - 1] ?? schedule.day_of_week} ${schedule.start_section}–${schedule.end_section} 节`,
    )
    .join('；')
}

export function toCourseSummary(item: TeachingClass, roundId: number): TeachingClassSummary {
  const primarySchedule = item.schedules[0]
  return {
    id: item.id,
    roundId,
    courseId: item.course_id,
    courseCode: item.course_code,
    courseName: item.course_name,
    teacherName: item.teacher_name || '教师待公布',
    credits: item.credits,
    schedule: formatSchedule(item),
    location: item.location || '地点待公布',
    capacity: item.capacity,
    selectedCount: item.selected_count,
    tags: [...item.tags],
    introduction: item.introduction || '课程介绍待补充。',
    hasVideo: false,
    dayOfWeek: primarySchedule?.day_of_week,
    startSection: primarySchedule?.start_section,
    endSection: primarySchedule?.end_section,
  }
}

export function replaceCourseCatalog(items: TeachingClass[], roundId: number): TeachingClassSummary[] {
  cachedCatalog.value = items.map((item) => toCourseSummary(item, roundId))
  return cachedCatalog.value
}

export function courseCatalog(roundId: number): TeachingClassSummary[] {
  return cachedCatalog.value.map((course) => ({ ...course, roundId }))
}

export function findCourse(
  teachingClassId: number,
  roundId = 0,
): TeachingClassSummary | undefined {
  const course = cachedCatalog.value.find((item) => item.id === teachingClassId)
  return course ? { ...course, roundId: roundId || course.roundId } : undefined
}

export function courseLabel(teachingClassId: number): string {
  return findCourse(teachingClassId)?.courseName ?? `教学班 #${teachingClassId}`
}
