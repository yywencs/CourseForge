import type { TeachingClassSummary } from '@/types/enrollment'

const catalog: Omit<TeachingClassSummary, 'roundId'>[] = [
  {
    id: 20001,
    courseId: 10001,
    courseCode: 'CS-304',
    courseName: '分布式系统设计',
    teacherName: '周屿教授',
    credits: 3.5,
    schedule: '周二 3–4 节',
    location: '格物楼 A308',
    capacity: 60,
    selectedCount: 42,
    tags: ['专业核心', '项目制'],
    introduction: '从一致性协议到消息可靠性，用一次完整工程实践理解分布式系统。',
    hasVideo: true,
    dayOfWeek: 2,
    startSection: 3,
    endSection: 4,
  },
  {
    id: 20002,
    courseId: 10002,
    courseCode: 'AI-217',
    courseName: '智能交互产品实践',
    teacherName: '许南乔副教授',
    credits: 2,
    schedule: '周四 7–8 节',
    location: '创新中心 C201',
    capacity: 40,
    selectedCount: 36,
    tags: ['跨专业', '工作坊'],
    introduction: '围绕真实校园场景，完成从用户研究、原型到可用产品的完整过程。',
    hasVideo: true,
    dayOfWeek: 4,
    startSection: 7,
    endSection: 8,
  },
  {
    id: 20003,
    courseId: 10003,
    courseCode: 'HUM-109',
    courseName: '影像叙事与当代文化',
    teacherName: '陈见微讲师',
    credits: 2,
    schedule: '周五 5–6 节',
    location: '人文馆 109',
    capacity: 80,
    selectedCount: 80,
    tags: ['通识选修', '研讨'],
    introduction: '以经典影像片段为入口，讨论媒介如何改变我们理解世界的方式。',
    hasVideo: false,
    dayOfWeek: 5,
    startSection: 5,
    endSection: 6,
  },
]

export function courseCatalog(roundId: number): TeachingClassSummary[] {
  return catalog.map((course) => ({ ...course, roundId }))
}

export function findCourse(
  teachingClassId: number,
  roundId = 0,
): TeachingClassSummary | undefined {
  const course = catalog.find((item) => item.id === teachingClassId)
  return course ? { ...course, roundId } : undefined
}

export function courseLabel(teachingClassId: number): string {
  return (
    findCourse(teachingClassId)?.courseName ??
    `教学班 #${teachingClassId}`
  )
}
