<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { Search, SlidersHorizontal, Sparkles } from '@lucide/vue'
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'

import CourseCard from '@/components/CourseCard.vue'
import { selectCourse } from '@/api/enrollment'
import { useSessionStore } from '@/stores/session'
import type { TeachingClassSummary } from '@/types/enrollment'

const session = useSessionStore()
const keyword = ref('')
const activeCourseId = ref<number | null>(null)

// 课程查询接口尚未实现，框架阶段使用可替换的演示数据。
const courses: TeachingClassSummary[] = [
  {
    id: 20001,
    roundId: 1,
    courseCode: 'CS-304',
    courseName: '分布式系统设计',
    teacherName: '周屿教授',
    credits: 3.5,
    schedule: '周二 3–4节',
    location: '格物楼 A308',
    capacity: 60,
    selectedCount: 42,
    tags: ['专业核心', '项目制'],
    introduction: '从一致性协议到消息可靠性，用一次完整工程实践理解分布式系统。',
    hasVideo: true,
  },
  {
    id: 20002,
    roundId: 1,
    courseCode: 'AI-217',
    courseName: '智能交互产品实践',
    teacherName: '许南乔副教授',
    credits: 2,
    schedule: '周四 7–8节',
    location: '创新中心 C201',
    capacity: 40,
    selectedCount: 36,
    tags: ['跨专业', '工作坊'],
    introduction: '围绕真实校园场景，完成从用户研究、原型到可用产品的完整过程。',
    hasVideo: true,
  },
  {
    id: 20003,
    roundId: 1,
    courseCode: 'HUM-109',
    courseName: '影像叙事与当代文化',
    teacherName: '陈见微讲师',
    credits: 2,
    schedule: '周五 5–6节',
    location: '人文馆 109',
    capacity: 80,
    selectedCount: 80,
    tags: ['通识选修', '研讨'],
    introduction: '以经典影像片段为入口，讨论媒介如何改变我们理解世界的方式。',
    hasVideo: false,
  },
]

const filteredCourses = computed(() => {
  const normalized = keyword.value.trim().toLowerCase()
  if (!normalized) return courses
  return courses.filter((course) =>
    [course.courseCode, course.courseName, course.teacherName]
      .join(' ')
      .toLowerCase()
      .includes(normalized),
  )
})

const selectionMutation = useMutation({
  mutationFn: (course: TeachingClassSummary) => {
    activeCourseId.value = course.id
    return selectCourse({
      request_id: crypto.randomUUID(),
      round_id: course.roundId,
      student_id: session.studentId,
      teaching_class_id: course.id,
      source: 'web',
    })
  },
  onSuccess: (receipt) => {
    ElMessage.success(`选课成功，申请单 ${receipt.application_id}`)
  },
  onError: (error: Error) => {
    ElMessage.error(error.message || '选课提交失败')
  },
  onSettled: () => {
    activeCourseId.value = null
  },
})
</script>

<template>
  <section class="student-page">
    <div class="catalog-hero">
      <div class="round-kicker"><Sparkles :size="15" /> 第一轮选课 · 剩余 2 天 14 小时</div>
      <h1>找到真正想投入的课程。</h1>
      <p>2026—2027 学年秋季学期，共开放 128 个教学班。</p>

      <label class="course-search">
        <Search :size="20" />
        <input v-model="keyword" type="search" placeholder="搜索课程、教师或课程代码" />
        <button type="button"><SlidersHorizontal :size="18" />筛选</button>
      </label>
    </div>

    <div class="catalog-heading">
      <div>
        <span>为你推荐</span>
        <h2>本轮开放课程</h2>
      </div>
      <p>以下为前端框架演示数据，课程查询接口接入后自动替换。</p>
    </div>

    <div class="course-grid">
      <CourseCard
        v-for="course in filteredCourses"
        :key="course.id"
        :course="course"
        :submitting="selectionMutation.isPending.value && activeCourseId === course.id"
        @select="selectionMutation.mutate"
      />
    </div>
  </section>
</template>
