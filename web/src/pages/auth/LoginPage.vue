<script setup lang="ts">
import { ArrowRight, CheckCircle2 } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'

import StudentLoginForm from '@/features/auth/StudentLoginForm.vue'

const router = useRouter()
const route = useRoute()

function handleAuthenticated(): void {
  const redirect =
    typeof route.query.redirect === 'string'
      ? route.query.redirect
      : '/student/courses'
  void router.replace(redirect)
}
</script>

<template>
  <main class="login-page">
    <section class="login-story">
      <span>COURSEFORGE / STUDENT</span>
      <h2>每个选择，都有一条可以追踪的处理链。</h2>
      <ul>
        <li><CheckCircle2 :size="17" /> 后端验签决定学生身份</li>
        <li><CheckCircle2 :size="17" /> 重复提交复用同一申请结果</li>
        <li><CheckCircle2 :size="17" /> 退课与候补状态实时可见</li>
      </ul>
      <RouterLink to="/admin">
        查看教务运行状态
        <ArrowRight :size="16" />
      </RouterLink>
    </section>
    <StudentLoginForm @authenticated="handleAuthenticated" />
  </main>
</template>

<style scoped>
.login-page {
  position: relative;
  display: grid;
  min-height: 100vh;
  grid-template-columns: minmax(300px, 0.8fr) minmax(520px, 1.2fr);
  place-items: center;
  gap: clamp(30px, 7vw, 110px);
  padding: clamp(24px, 6vw, 90px);
  background:
    linear-gradient(120deg, rgba(232, 246, 240, 0.92), rgba(242, 247, 245, 0.86)),
    repeating-linear-gradient(
      90deg,
      transparent 0,
      transparent 78px,
      rgba(17, 91, 74, 0.05) 79px,
      rgba(17, 91, 74, 0.05) 80px
    );
}

.login-story {
  max-width: 470px;
}

.login-story > span {
  color: var(--brand);
  font-family: var(--font-mono);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.16em;
}

.login-story h2 {
  margin: 18px 0 28px;
  font-family: var(--font-display);
  font-size: clamp(38px, 5vw, 62px);
  letter-spacing: -0.05em;
  line-height: 1.08;
}

.login-story ul {
  display: grid;
  gap: 12px;
  margin: 0 0 30px;
  padding: 0;
  color: #3d5d54;
  list-style: none;
}

.login-story li,
.login-story a {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.login-story li svg {
  color: var(--brand-bright);
}

.login-story a {
  width: fit-content;
  color: var(--brand);
  font-weight: 750;
}

@media (max-width: 900px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .login-story {
    display: none;
  }
}
</style>
