<script setup lang="ts">
import {
  BookOpen,
  CalendarDays,
  GraduationCap,
  Radio,
  UserCog,
} from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'

import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const navigation = [
  { to: '/student/courses', label: '选课中心', shortLabel: '选课', icon: BookOpen },
  { to: '/student/enrollments', label: '我的选课', shortLabel: '记录', icon: GraduationCap },
  { to: '/student/schedule', label: '本学期课表', shortLabel: '课表', icon: CalendarDays },
  { to: '/student/account', label: '身份设置', shortLabel: '我的', icon: UserCog },
]
</script>

<template>
  <div class="student-shell">
    <aside class="student-rail">
      <RouterLink class="brand" to="/student/courses" aria-label="CourseForge 选课中心">
        <span class="brand-mark">C</span>
        <span class="brand-copy">
          <strong>CourseForge</strong>
          <small>学生选课系统</small>
        </span>
      </RouterLink>

      <nav class="student-nav" aria-label="学生端主导航">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to">
          <component :is="item.icon" :size="20" stroke-width="1.8" />
          <span>{{ item.label }}</span>
          <small>{{ item.shortLabel }}</small>
        </RouterLink>
      </nav>

      <div class="media-ready" title="课程视频和直播入口将在选课中心逐步开放">
        <Radio :size="18" />
        <span><strong>课程现场</strong><small>视频 · 直播</small></span>
      </div>

      <RouterLink class="student-profile" to="/student/account">
        <span class="avatar">{{ session.initials }}</span>
        <span>
          <strong>{{ session.studentName }}</strong>
          <small>{{ session.studentNo }}</small>
        </span>
      </RouterLink>
    </aside>

    <div class="student-stage">
      <header class="mobile-header">
        <RouterLink to="/student/courses"><strong>CourseForge</strong></RouterLink>
        <span>学期 {{ session.context.termId }} · 第 {{ session.context.roundId }} 轮</span>
      </header>
      <main><RouterView /></main>
      <footer class="student-footer">
        <span>CourseForge · 学生选课系统</span>
        <RouterLink to="/admin">进入教务端</RouterLink>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.student-shell {
  display: grid;
  min-height: 100vh;
  grid-template-columns: 230px minmax(0, 1fr);
}

.student-rail {
  position: sticky;
  z-index: 40;
  top: 0;
  display: flex;
  height: 100vh;
  flex-direction: column;
  padding: 22px 16px;
  border-right: 1px solid var(--line);
  color: var(--ink);
  background: var(--surface);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 8px 22px;
  border-bottom: 1px solid var(--line-soft);
}

.brand-mark {
  display: grid;
  width: 42px;
  height: 52px;
  place-items: center;
  color: white;
  background: var(--brand);
  font-family: var(--font-display);
  font-size: 27px;
  font-variation-settings: "wdth" 72, "wght" 720;
  line-height: 1;
}

.brand-copy,
.student-profile > span:last-child,
.media-ready > span {
  display: grid;
  gap: 2px;
}

.brand strong {
  font-family: var(--font-display);
  font-size: 17px;
  font-variation-settings: "wdth" 78, "wght" 680;
  letter-spacing: -0.03em;
}

.brand small,
.student-profile small,
.media-ready small {
  color: var(--muted);
  font-size: 10px;
}

.student-nav {
  display: grid;
  gap: 4px;
  margin-top: 25px;
}

.student-nav a {
  position: relative;
  display: grid;
  grid-template-columns: 26px 1fr;
  align-items: center;
  gap: 9px;
  min-height: 46px;
  padding: 10px 11px;
  color: var(--muted);
  font-size: 13px;
  font-weight: 620;
  transition: color 160ms ease, background 160ms ease;
}

.student-nav a small {
  display: none;
}

.student-nav a::before {
  position: absolute;
  top: 8px;
  bottom: 8px;
  left: -16px;
  width: 4px;
  background: transparent;
  content: "";
}

.student-nav a:hover {
  color: var(--ink);
  background: var(--surface-muted);
}

.student-nav a.router-link-active {
  color: var(--brand);
  background: var(--brand-pale);
}

.student-nav a.router-link-active::before {
  background: var(--brand);
}

.media-ready {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: auto;
  padding: 14px 11px;
  border-top: 1px solid var(--line-soft);
  border-bottom: 1px solid var(--line-soft);
  color: var(--signal);
}

.media-ready strong {
  color: var(--ink);
  font-size: 11px;
}

.student-profile {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 8px 2px;
}

.student-profile strong {
  font-size: 12px;
}

.avatar {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 auto;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: var(--brand);
  font-size: 11px;
  font-weight: 800;
}

.student-stage {
  min-width: 0;
}

.mobile-header {
  display: none;
}

.student-footer {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 24px 32px;
  border-top: 1px solid var(--line);
  color: var(--muted);
  font-size: 11px;
}

.student-footer a {
  color: var(--brand);
  font-weight: 750;
}

@media (max-width: 1050px) {
  .student-shell {
    grid-template-columns: 76px minmax(0, 1fr);
  }

  .student-rail {
    padding-inline: 11px;
  }

  .brand {
    justify-content: center;
    padding-inline: 0;
  }

  .brand-copy,
  .student-nav a > span,
  .media-ready > span,
  .student-profile > span:last-child {
    display: none;
  }

  .student-nav a {
    grid-template-columns: 1fr;
    justify-items: center;
    padding-inline: 4px;
  }

  .student-nav a small {
    display: block;
    color: inherit;
    font-size: 9px;
  }

  .student-nav a::before {
    left: -11px;
  }

  .media-ready,
  .student-profile {
    justify-content: center;
  }
}

@media (max-width: 680px) {
  .student-shell {
    display: block;
  }

  .student-rail {
    position: fixed;
    z-index: 60;
    top: auto;
    right: 0;
    bottom: 0;
    left: 0;
    display: block;
    width: auto;
    height: 68px;
    padding: 6px 10px max(6px, env(safe-area-inset-bottom));
    border-top: 1px solid var(--line);
  }

  .brand,
  .media-ready,
  .student-profile {
    display: none;
  }

  .student-nav {
    grid-template-columns: repeat(4, 1fr);
    gap: 2px;
    margin: 0;
  }

  .student-nav a {
    min-height: 50px;
    gap: 2px;
  }

  .student-nav a::before {
    top: -6px;
    right: 18px;
    bottom: auto;
    left: 18px;
    width: auto;
    height: 3px;
  }

  .student-nav a.router-link-active {
    background: transparent;
  }

  .mobile-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    min-height: 58px;
    padding: 12px 15px;
    border-bottom: 1px solid var(--line);
    background: var(--surface);
  }

  .mobile-header strong {
    font-family: var(--font-display);
    font-size: 18px;
    font-variation-settings: "wdth" 70, "wght" 780;
  }

  .mobile-header > span {
    color: var(--muted);
    font-size: 10px;
  }

  .student-footer {
    padding-bottom: 92px;
  }
}
</style>
