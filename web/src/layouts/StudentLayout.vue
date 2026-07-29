<script setup lang="ts">
import { BookOpen, CalendarDays, GraduationCap, ShieldCheck } from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const navigation = [
  { to: '/student/courses', label: '选课中心', icon: BookOpen },
  { to: '/student/enrollments', label: '我的选课', icon: GraduationCap },
  { to: '/student/schedule', label: '本学期课表', icon: CalendarDays },
]
</script>

<template>
  <div class="student-shell">
    <header class="student-header">
      <RouterLink class="brand" to="/student/courses" aria-label="CourseForge 首页">
        <span class="brand-mark">CF</span>
        <span>
          <strong>CourseForge</strong>
          <small>让每一次选择都有确定结果</small>
        </span>
      </RouterLink>

      <nav class="student-nav" aria-label="学生端主导航">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to">
          <component :is="item.icon" :size="17" />
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="student-profile">
        <span class="avatar">{{ session.initials }}</span>
        <span>
          <strong>{{ session.studentName }}</strong>
          <small>{{ session.studentNo }}</small>
        </span>
      </div>
    </header>

    <main>
      <RouterView />
    </main>

    <footer class="student-footer">
      <span><ShieldCheck :size="15" /> 原子占用名额，重复提交不会重复选课</span>
      <RouterLink to="/admin">进入教务端</RouterLink>
    </footer>
  </div>
</template>
