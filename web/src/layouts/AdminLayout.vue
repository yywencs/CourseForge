<script setup lang="ts">
import {
  Activity,
  BookCopy,
  ChevronLeft,
  LayoutDashboard,
} from '@lucide/vue'
import { RouterLink, RouterView } from 'vue-router'

const navigation = [
  { to: '/admin', label: '运行概览', icon: LayoutDashboard },
  { to: '/admin/courses', label: '课程能力', icon: BookCopy },
  { to: '/admin/enrollments', label: '选课能力', icon: Activity },
]
</script>

<template>
  <div class="admin-shell">
    <aside class="admin-sidebar">
      <RouterLink class="admin-brand" to="/admin">
        <span class="admin-brand__mark">CF</span>
        <span><strong>CourseForge</strong><small>教务运行台</small></span>
      </RouterLink>
      <nav class="admin-nav" aria-label="教务端主导航">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to">
          <component :is="item.icon" :size="18" />
          {{ item.label }}
        </RouterLink>
      </nav>
      <RouterLink class="back-link" to="/student/courses">
        <ChevronLeft :size="17" />
        返回学生端
      </RouterLink>
    </aside>

    <div class="admin-workspace">
      <header class="admin-topbar">
        <div>
          <small>CourseForge operations</small>
          <strong>后端能力对照面板</strong>
        </div>
        <span><i /> 每 15 秒自动探测</span>
      </header>
      <main class="admin-content"><RouterView /></main>
    </div>
  </div>
</template>

<style scoped>
.admin-shell {
  display: grid;
  min-height: 100vh;
  grid-template-columns: 238px 1fr;
  background: #edf3f0;
}

.admin-sidebar {
  position: sticky;
  top: 0;
  display: flex;
  height: 100vh;
  flex-direction: column;
  padding: 24px 17px;
  color: white;
  background:
    linear-gradient(160deg, rgba(45, 139, 112, 0.12), transparent 42%),
    #0b392f;
}

.admin-brand {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 0 7px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.12);
}

.admin-brand__mark {
  display: grid;
  width: 39px;
  height: 39px;
  place-items: center;
  border-radius: 12px 12px 12px 4px;
  color: #0b493a;
  background: #ace8d8;
  font-family: var(--font-display);
  font-size: 13px;
  font-weight: 800;
}

.admin-brand > span:last-child {
  display: grid;
  gap: 2px;
}

.admin-brand strong {
  font-family: var(--font-display);
}

.admin-brand small {
  color: rgba(255, 255, 255, 0.5);
  font-size: 10px;
}

.admin-nav {
  display: grid;
  gap: 5px;
  margin-top: 22px;
}

.admin-nav a,
.back-link {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 11px;
  border-radius: 9px;
  color: rgba(255, 255, 255, 0.62);
  font-size: 12px;
}

.admin-nav a.router-link-exact-active {
  color: white;
  background: rgba(175, 234, 216, 0.13);
}

.back-link {
  margin-top: auto;
}

.admin-workspace {
  min-width: 0;
}

.admin-topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 70px;
  padding: 11px 32px;
  border-bottom: 1px solid #d5e0db;
  background: rgba(250, 253, 252, 0.82);
  backdrop-filter: blur(14px);
}

.admin-topbar > div {
  display: grid;
  gap: 3px;
}

.admin-topbar small {
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 9px;
}

.admin-topbar strong {
  font-size: 12px;
}

.admin-topbar > span {
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 10px;
}

.admin-topbar i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--success);
  box-shadow: 0 0 0 4px rgba(26, 146, 110, 0.12);
}

.admin-content {
  padding: 32px;
}

@media (max-width: 800px) {
  .admin-shell {
    grid-template-columns: 74px 1fr;
  }

  .admin-brand > span:last-child,
  .admin-nav a,
  .back-link {
    font-size: 0;
  }

  .admin-nav a,
  .back-link {
    justify-content: center;
  }
}

@media (max-width: 580px) {
  .admin-shell {
    display: block;
  }

  .admin-sidebar {
    position: static;
    width: 100%;
    height: auto;
  }

  .admin-nav {
    grid-template-columns: repeat(3, 1fr);
  }

  .admin-nav a {
    font-size: 10px;
  }

  .back-link {
    display: none;
  }

  .admin-content,
  .admin-topbar {
    padding-inline: 15px;
  }
}
</style>
