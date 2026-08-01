<script setup lang="ts">
import { Activity, BookCopy, ChevronLeft, LayoutDashboard } from '@lucide/vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'

const route = useRoute()
const navigation = [
  { to: '/admin', label: '运行概览', icon: LayoutDashboard },
  { to: '/admin/courses', label: '课程管理', icon: BookCopy },
  { to: '/admin/enrollments', label: '选课管理', icon: Activity },
]
</script>

<template>
  <div class="admin-shell">
    <aside class="admin-sidebar">
      <RouterLink class="admin-brand" to="/admin">
        <span class="admin-brand__mark">C</span>
        <span><strong>CourseForge</strong><small>教务管理台</small></span>
      </RouterLink>
      <nav class="admin-nav" aria-label="教务端主导航">
        <RouterLink v-for="item in navigation" :key="item.to" :to="item.to">
          <component :is="item.icon" :size="19" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </nav>
      <RouterLink class="back-link" to="/student/courses"><ChevronLeft :size="17" />返回学生端</RouterLink>
    </aside>

    <div class="admin-workspace">
      <header class="admin-topbar">
        <div><span>教务管理台</span><strong>{{ route.meta.title }}</strong></div>
        <div class="admin-operator"><span>教务值班</span><i>CF</i></div>
      </header>
      <main class="admin-content"><RouterView /></main>
    </div>
  </div>
</template>

<style scoped>
.admin-shell {
  display: grid;
  min-height: 100vh;
  grid-template-columns: 238px minmax(0, 1fr);
  background: var(--canvas);
}

.admin-sidebar {
  position: sticky;
  top: 0;
  display: flex;
  height: 100vh;
  flex-direction: column;
  padding: 22px 16px;
  border-right: 1px solid var(--line);
  color: var(--ink);
  background: var(--surface);
}

.admin-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 7px 22px;
  border-bottom: 1px solid var(--line-soft);
}

.admin-brand__mark {
  display: grid;
  width: 42px;
  height: 52px;
  place-items: center;
  color: white;
  background: var(--brand);
  font-family: var(--font-display);
  font-size: 27px;
  font-variation-settings: "wdth" 72, "wght" 720;
}

.admin-brand > span:last-child {
  display: grid;
  gap: 2px;
}

.admin-brand strong {
  font-family: var(--font-display);
  font-size: 17px;
  font-variation-settings: "wdth" 78, "wght" 680;
}

.admin-brand small {
  color: var(--muted);
  font-size: 10px;
}

.admin-nav {
  display: grid;
  gap: 4px;
  margin-top: 24px;
}

.admin-nav a,
.back-link {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 46px;
  padding: 10px 11px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 650;
}

.admin-nav a:hover,
.back-link:hover {
  color: var(--ink);
  background: var(--surface-muted);
}

.admin-nav a.router-link-exact-active {
  color: var(--brand);
  background: var(--brand-pale);
}

.back-link {
  margin-top: auto;
  border-top: 1px solid var(--line-soft);
}

.admin-workspace {
  min-width: 0;
}

.admin-topbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  min-height: 72px;
  padding: 12px 32px;
  border-bottom: 1px solid var(--line);
  background: var(--surface);
}

.admin-topbar > div:first-child {
  display: flex;
  align-items: baseline;
  gap: 12px;
}

.admin-topbar span {
  color: var(--muted);
  font-size: 10px;
}

.admin-topbar strong {
  font-family: var(--font-display);
  font-size: 19px;
  font-variation-settings: "wdth" 74, "wght" 760;
}

.admin-operator {
  display: flex;
  align-items: center;
  gap: 9px;
}

.admin-operator i {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 50%;
  color: white;
  background: var(--brand);
  font-size: 10px;
  font-style: normal;
  font-weight: 800;
}

.admin-content {
  width: min(1280px, 100%);
  padding: 38px 40px 70px;
}

@media (max-width: 800px) {
  .admin-shell {
    grid-template-columns: 76px minmax(0, 1fr);
  }

  .admin-brand {
    justify-content: center;
    padding-inline: 0;
  }

  .admin-brand > span:last-child,
  .admin-nav a span,
  .back-link {
    font-size: 0;
  }

  .admin-nav a,
  .back-link {
    justify-content: center;
  }

  .admin-content {
    padding-inline: 24px;
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
    padding: 12px;
  }

  .admin-brand,
  .back-link {
    display: none;
  }

  .admin-nav {
    grid-template-columns: repeat(3, 1fr);
    margin: 0;
  }

  .admin-nav a {
    flex-direction: column;
    gap: 4px;
    font-size: 9px;
  }

  .admin-nav a span {
    display: inline;
    font-size: 9px;
  }

  .admin-topbar {
    padding-inline: 16px;
  }

  .admin-content {
    padding: 28px 15px 60px;
  }
}
</style>
