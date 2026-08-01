<script setup lang="ts">
import { ArrowUpRight, Play, Radio } from '@lucide/vue'
import { useRoute, useRouter } from 'vue-router'

import StudentLoginForm from '@/features/auth/StudentLoginForm.vue'

const router = useRouter()
const route = useRoute()

function handleAuthenticated(): void {
  const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/student/courses'
  void router.replace(redirect)
}
</script>

<template>
  <main class="login-page">
    <section class="login-poster">
      <header>
        <span class="login-mark">C</span>
        <strong>CourseForge</strong>
        <small>学生选课系统</small>
      </header>

      <div class="login-poster__headline">
        <h1>选好课程，<br /><em>开始新学期</em></h1>
        <p>课程、教师、时间与名额，都在这里清楚呈现。</p>
      </div>

      <div class="program-strip">
        <span><b>01</b> 浏览本轮课程</span>
        <span><b>02</b> 提交选课或候补</span>
        <span><b>03</b> 查看结果与课表</span>
      </div>

      <div class="media-note">
        <span><Play :size="18" fill="currentColor" />课程预览</span>
        <span><Radio :size="18" />直播宣讲</span>
        <small>课程内容入口将逐步开放</small>
      </div>

      <RouterLink to="/admin">进入教务端 <ArrowUpRight :size="17" /></RouterLink>
    </section>

    <section class="login-access">
      <StudentLoginForm @authenticated="handleAuthenticated" />
      <p>请使用学校提供的学生账号登录。</p>
    </section>
  </main>
</template>

<style scoped>
.login-page {
  display: grid;
  min-height: 100vh;
  grid-template-columns: minmax(480px, 1.15fr) minmax(430px, 0.85fr);
  background: var(--surface);
}

.login-poster {
  position: relative;
  display: flex;
  overflow: hidden;
  min-height: 100vh;
  flex-direction: column;
  padding: clamp(28px, 5vw, 70px);
  border-right: 1px solid var(--line);
  color: var(--ink);
  background: var(--brand-pale);
}

.login-poster::after {
  position: absolute;
  right: clamp(28px, 5vw, 70px);
  bottom: 0;
  width: 110px;
  height: 6px;
  background: var(--signal);
  content: "";
}

.login-poster > header {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 2px 12px;
}

.login-mark {
  display: grid;
  width: 44px;
  height: 54px;
  grid-row: 1 / 3;
  place-items: center;
  color: white;
  background: var(--brand);
  font-family: var(--font-display);
  font-size: 32px;
  font-variation-settings: "wdth" 60, "wght" 850;
}

.login-poster header strong {
  font-family: var(--font-display);
  font-size: 24px;
  font-variation-settings: "wdth" 68, "wght" 780;
  letter-spacing: -0.03em;
}

.login-poster header small {
  color: var(--muted);
  font-size: 10px;
}

.login-poster__headline {
  position: relative;
  z-index: 1;
  margin: auto 0;
}

.login-poster h1 {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(44px, 5.5vw, 70px);
  font-variation-settings: "wdth" 78, "wght" 680;
  letter-spacing: -0.035em;
  line-height: 1.04;
}

.login-poster h1 em {
  color: var(--brand);
  font-style: normal;
}

.login-poster__headline p {
  margin: 25px 0 0;
  color: var(--muted);
  font-size: 14px;
}

.program-strip {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  border-top: 1px solid var(--line);
  border-bottom: 1px solid var(--line);
}

.program-strip span {
  display: grid;
  gap: 5px;
  padding: 14px 12px;
  border-right: 1px solid var(--line);
  font-size: 11px;
}

.program-strip span:last-child {
  border-right: 0;
}

.program-strip b {
  color: var(--brand);
  font-family: var(--font-display);
  font-size: 23px;
  font-variation-settings: "wdth" 62, "wght" 800;
}

.media-note {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 20px;
  margin-top: 18px;
}

.media-note > span {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
  font-weight: 700;
}

.media-note > span:first-child {
  color: var(--brand);
}

.media-note > span:nth-child(2) {
  color: var(--signal);
}

.media-note small {
  color: var(--muted);
  font-size: 9px;
}

.login-poster > a {
  position: absolute;
  z-index: 2;
  top: clamp(28px, 5vw, 70px);
  right: clamp(28px, 5vw, 70px);
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
}

.login-access {
  display: grid;
  align-content: center;
  padding: clamp(28px, 7vw, 100px);
  background: var(--canvas);
}

.login-access > p {
  margin: 18px 0 0;
  color: var(--muted);
  font-size: 10px;
  text-align: center;
}

@media (max-width: 900px) {
  .login-page {
    grid-template-columns: 1fr;
  }

  .login-poster {
    min-height: 410px;
  }

  .login-poster h1 {
    font-size: clamp(42px, 9vw, 60px);
  }

  .login-poster__headline {
    margin: 70px 0 55px;
  }
}

@media (max-width: 580px) {
  .login-poster {
    min-height: 405px;
    padding: 22px;
  }

  .login-poster > a,
  .media-note small {
    display: none;
  }

  .login-poster__headline {
    margin: 58px 0 20px;
  }

  .login-poster h1 {
    font-size: 40px;
  }

  .login-poster__headline p {
    margin-top: 16px;
    font-size: 11px;
  }

  .program-strip {
    grid-template-columns: repeat(3, 1fr);
  }

  .program-strip span {
    padding-inline: 8px;
    font-size: 9px;
  }

  .media-note {
    gap: 13px;
    margin-top: 12px;
  }

  .login-access {
    padding: 30px 20px 46px;
  }
}
</style>
