<script setup lang="ts">
import {
  BookOpen,
  CalendarDays,
  GraduationCap,
  ShieldCheck,
  UserCog,
} from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const navigation = [
  { to: '/student/courses', label: '选课中心', icon: BookOpen },
  { to: '/student/enrollments', label: '我的选课', icon: GraduationCap },
  { to: '/student/schedule', label: '本学期课表', icon: CalendarDays },
  { to: '/student/account', label: '身份设置', icon: UserCog },
]
</script>

<template>
  <div class="student-shell">
    <header class="student-header">
      <RouterLink class="brand" to="/student/courses" aria-label="CourseForge 首页">
        <span class="brand-mark">CF</span>
        <span>
          <strong>CourseForge</strong>
          <small>学生选课台</small>
        </span>
      </RouterLink>

      <nav class="student-nav" aria-label="学生端主导航">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to">
          <component :is="item.icon" :size="16" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>

      <RouterLink class="student-profile" to="/student/account">
        <span class="avatar">{{ session.initials }}</span>
        <span>
          <strong>{{ session.studentName }}</strong>
          <small>{{ session.studentNo }}</small>
        </span>
      </RouterLink>
    </header>

    <main>
      <RouterView />
    </main>

    <footer class="student-footer">
      <span><ShieldCheck :size="15" /> 身份来自 JWT，名额由 Redis 原子锁定</span>
      <RouterLink to="/admin">查看教务运行状态</RouterLink>
    </footer>
  </div>
</template>

<style scoped>
.student-shell {
  min-height: 100vh;
}

.student-header {
  position: sticky;
  z-index: 40;
  top: 0;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  min-height: 74px;
  padding: 11px clamp(22px, 5vw, 76px);
  border-bottom: 1px solid rgba(198, 215, 208, 0.8);
  background: rgba(247, 251, 249, 0.9);
  backdrop-filter: blur(18px);
}

.brand,
.student-profile {
  display: flex;
  align-items: center;
  gap: 11px;
}

.brand > span:last-child,
.student-profile > span:last-child {
  display: grid;
  gap: 2px;
}

.brand strong {
  font-family: var(--font-display);
  font-size: 19px;
}

.brand small,
.student-profile small {
  color: var(--muted);
  font-size: 10px;
}

.brand-mark {
  display: grid;
  width: 39px;
  height: 39px;
  place-items: center;
  border-radius: 12px 12px 12px 4px;
  color: white;
  background: var(--brand);
  font-family: var(--font-display);
  font-size: 13px;
  font-weight: 800;
}

.student-nav {
  display: flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--line);
  border-radius: 13px;
  background: rgba(231, 240, 236, 0.72);
}

.student-nav a {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 9px 12px;
  border-radius: 9px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 650;
}

.student-nav a.router-link-active {
  color: var(--ink);
  background: white;
  box-shadow: 0 4px 13px rgba(12, 57, 45, 0.08);
}

.student-profile {
  justify-self: end;
}

.student-profile strong {
  font-size: 12px;
}

.avatar {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 50%;
  color: #155846;
  background: #cceee4;
  font-size: 12px;
  font-weight: 800;
}

.student-footer {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 22px clamp(22px, 5vw, 76px);
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: 11px;
}

.student-footer span {
  display: flex;
  align-items: center;
  gap: 6px;
}

.student-footer a {
  color: var(--brand);
  font-weight: 750;
}

@media (max-width: 1060px) {
  .student-header {
    grid-template-columns: 1fr auto;
  }

  .student-nav {
    position: fixed;
    z-index: 50;
    right: 14px;
    bottom: 14px;
    left: 14px;
    justify-content: center;
    box-shadow: 0 18px 45px rgba(11, 58, 46, 0.2);
  }

  .student-nav a {
    flex: 1;
    justify-content: center;
  }

  .student-profile {
    display: flex;
  }

  .student-footer {
    padding-bottom: 86px;
  }
}

@media (max-width: 650px) {
  .student-header {
    padding-inline: 16px;
  }

  .brand small,
  .student-profile > span:last-child {
    display: none;
  }

  .student-nav a {
    flex-direction: column;
    gap: 2px;
    padding: 7px 4px;
    font-size: 9px;
  }

  .student-footer {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
