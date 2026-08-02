<script setup lang="ts">
import { ArrowLeft, BookCopy, CalendarRange, ChartNoAxesCombined } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'

import AdministratorLoginForm from '@/features/auth/AdministratorLoginForm.vue'

const route = useRoute()
const router = useRouter()

function handleAuthenticated(): void {
  const requestedPath = typeof route.query.redirect === 'string' ? route.query.redirect : ''
  const redirect = requestedPath.startsWith('/admin') && requestedPath !== '/admin/login'
    ? requestedPath
    : '/admin'
  void router.replace(redirect)
}
</script>

<template>
  <main class="administrator-login-page">
    <section class="administrator-login-brief">
      <RouterLink class="administrator-login-brand" to="/admin">
        <span>C</span>
        <strong>CourseForge</strong>
        <small>教务管理台</small>
      </RouterLink>

      <div class="administrator-login-brief__message">
        <small>AUTHORIZED WORKSPACE</small>
        <h1>让每一轮选课<br />按计划发生。</h1>
        <p>集中维护课程、教学班和选课轮次，及时查看系统运行状态。</p>
      </div>

      <ul class="administrator-login-capabilities" aria-label="教务管理能力">
        <li><BookCopy :size="19" /><span><b>课程编排</b><small>课程与教学班</small></span></li>
        <li><CalendarRange :size="19" /><span><b>轮次配置</b><small>时间与开放范围</small></span></li>
        <li><ChartNoAxesCombined :size="19" /><span><b>运行观察</b><small>服务与选课状态</small></span></li>
      </ul>
    </section>

    <section class="administrator-login-access">
      <RouterLink class="administrator-login-back" to="/login">
        <ArrowLeft :size="16" />返回学生登录
      </RouterLink>
      <AdministratorLoginForm @authenticated="handleAuthenticated" />
      <p>仅限已授权的教务管理员使用。</p>
    </section>
  </main>
</template>

<style scoped>
.administrator-login-page {
  display: grid;
  min-height: 100vh;
  grid-template-columns: minmax(440px, 0.9fr) minmax(480px, 1.1fr);
  background: var(--canvas);
}

.administrator-login-brief {
  position: relative;
  display: flex;
  overflow: hidden;
  min-height: 100vh;
  flex-direction: column;
  padding: clamp(30px, 5vw, 68px);
  color: white;
  background-color: var(--brand);
  background-image:
    linear-gradient(rgba(255, 255, 255, 0.07) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.07) 1px, transparent 1px);
  background-size: 56px 56px;
}

.administrator-login-brief::after {
  position: absolute;
  right: -90px;
  bottom: -90px;
  width: 280px;
  height: 280px;
  border: 54px solid rgba(255, 255, 255, 0.08);
  border-radius: 50%;
  content: "";
}

.administrator-login-brand {
  position: relative;
  z-index: 1;
  display: grid;
  width: fit-content;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 1px 12px;
}

.administrator-login-brand > span {
  display: grid;
  width: 43px;
  height: 52px;
  grid-row: 1 / 3;
  place-items: center;
  color: var(--brand);
  background: white;
  font-family: var(--font-display);
  font-size: 29px;
  font-variation-settings: "wdth" 64, "wght" 820;
}

.administrator-login-brand strong {
  font-family: var(--font-display);
  font-size: 21px;
  font-variation-settings: "wdth" 72, "wght" 740;
}

.administrator-login-brand small {
  color: rgba(255, 255, 255, 0.68);
  font-size: 10px;
}

.administrator-login-brief__message {
  position: relative;
  z-index: 1;
  margin: auto 0;
}

.administrator-login-brief__message > small {
  display: block;
  margin-bottom: 18px;
  font-family: var(--font-mono);
  font-size: 10px;
  letter-spacing: 0.14em;
  opacity: 0.7;
}

.administrator-login-brief h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(40px, 5vw, 64px);
  font-variation-settings: "wdth" 76, "wght" 680;
  letter-spacing: -0.04em;
  line-height: 1.08;
}

.administrator-login-brief__message p {
  max-width: 38ch;
  margin: 24px 0 0;
  color: rgba(255, 255, 255, 0.74);
  font-size: 13px;
  line-height: 1.8;
}

.administrator-login-capabilities {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  margin: 0;
  padding: 0;
  border-top: 1px solid rgba(255, 255, 255, 0.25);
  list-style: none;
}

.administrator-login-capabilities li {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 10px;
  padding: 16px 12px;
  border-right: 1px solid rgba(255, 255, 255, 0.25);
}

.administrator-login-capabilities li:last-child {
  border-right: 0;
}

.administrator-login-capabilities span {
  display: grid;
  gap: 3px;
}

.administrator-login-capabilities b {
  font-size: 11px;
}

.administrator-login-capabilities small {
  color: rgba(255, 255, 255, 0.62);
  font-size: 9px;
}

.administrator-login-access {
  position: relative;
  display: grid;
  align-content: center;
  justify-items: center;
  padding: clamp(36px, 8vw, 110px);
  background: var(--surface);
}

.administrator-login-back {
  position: absolute;
  top: clamp(30px, 5vw, 68px);
  right: clamp(30px, 5vw, 68px);
  display: flex;
  align-items: center;
  gap: 7px;
  color: var(--muted);
  font-size: 11px;
}

.administrator-login-back:hover {
  color: var(--ink);
}

.administrator-login-access > p {
  margin: 18px 0 0;
  color: var(--muted);
  font-size: 10px;
}

@media (max-width: 900px) {
  .administrator-login-page {
    grid-template-columns: 1fr;
  }

  .administrator-login-brief {
    min-height: 430px;
  }

  .administrator-login-brief__message {
    margin: 64px 0 50px;
  }
}

@media (max-width: 580px) {
  .administrator-login-brief {
    min-height: 390px;
    padding: 22px;
  }

  .administrator-login-brief__message {
    margin: 55px 0 30px;
  }

  .administrator-login-brief h1 {
    font-size: 39px;
  }

  .administrator-login-capabilities li {
    display: flex;
    justify-content: center;
    padding-inline: 6px;
  }

  .administrator-login-capabilities svg,
  .administrator-login-capabilities small {
    display: none;
  }

  .administrator-login-access {
    padding: 80px 20px 46px;
  }

  .administrator-login-back {
    top: 28px;
    right: 20px;
  }
}
</style>
