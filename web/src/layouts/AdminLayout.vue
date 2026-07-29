<script setup lang="ts">
import {
  Activity,
  BookCopy,
  ChevronLeft,
  LayoutDashboard,
  Settings2,
} from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'

const navigation = [
  { to: '/admin', label: '教务概览', icon: LayoutDashboard, exact: true },
  { to: '/admin/courses', label: '课程与教学班', icon: BookCopy },
  { to: '/admin/enrollments', label: '选课申请监控', icon: Activity },
]
</script>

<template>
  <div class="admin-shell">
    <aside class="admin-sidebar">
      <div class="admin-brand">
        <span class="brand-mark brand-mark--light">CF</span>
        <span>
          <strong>CourseForge</strong>
          <small>教务工作台</small>
        </span>
      </div>

      <nav class="admin-nav" aria-label="教务端主导航">
        <RouterLink
          v-for="item in navigation"
          :key="item.to"
          :to="item.to"
          :exact-active-class="item.exact ? 'router-link-exact-active' : ''"
        >
          <component :is="item.icon" :size="18" />
          {{ item.label }}
        </RouterLink>
      </nav>

      <div class="admin-sidebar-bottom">
        <button type="button"><Settings2 :size="18" />系统设置</button>
        <RouterLink to="/student/courses">
          <ChevronLeft :size="18" />返回学生端
        </RouterLink>
      </div>
    </aside>

    <div class="admin-workspace">
      <header class="admin-topbar">
        <div>
          <small>2026—2027 学年 · 秋季学期</small>
          <strong>第一轮选课进行中</strong>
        </div>
        <span class="live-indicator"><i /> 系统运行正常</span>
      </header>
      <main class="admin-content">
        <RouterView />
      </main>
    </div>
  </div>
</template>
