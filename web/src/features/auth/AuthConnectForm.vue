<script setup lang="ts">
import { KeyRound, ShieldCheck, WandSparkles } from '@lucide/vue'
import { reactive, shallowRef } from 'vue'

import { useSessionStore } from '@/stores/session'
import { createDevelopmentJwt } from '@/utils/jwt'

const emit = defineEmits<{
  connected: []
}>()

const session = useSessionStore()
const developmentMode = import.meta.env.DEV
const mode = shallowRef<'token' | 'development'>('token')
const loading = shallowRef(false)
const errorMessage = shallowRef('')
const form = reactive({
  token: '',
  studentId: '10001',
  studentName: '林知夏',
  studentNo: '2026001001',
  termId: '1',
  roundId: '1',
  signingKey: '',
  issuer: 'courseforge',
  audience: 'courseforge-student',
})

async function connect(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    let accessToken = form.token.trim()
    if (mode.value === 'development') {
      accessToken = await createDevelopmentJwt({
        studentId: Number(form.studentId),
        signingKey: form.signingKey,
        issuer: form.issuer.trim(),
        audience: form.audience.trim(),
      })
    }
    session.connect({
      accessToken,
      studentName: form.studentName,
      studentNo: form.studentNo,
      termId: Number(form.termId),
      roundId: Number(form.roundId),
    })
    emit('connected')
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : '无法连接学生身份'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="connect-card">
    <header>
      <span class="connect-card__mark"><KeyRound :size="21" /></span>
      <div>
        <small>Student identity</small>
        <h1>连接学生身份</h1>
        <p>JWT 只保存在当前浏览器会话中，学生 ID 最终由后端验签确认。</p>
      </div>
    </header>

    <div class="connect-tabs" role="tablist">
      <button
        type="button"
        :class="{ 'is-active': mode === 'token' }"
        @click="mode = 'token'"
      >
        粘贴 JWT
      </button>
      <button
        v-if="developmentMode"
        type="button"
        :class="{ 'is-active': mode === 'development' }"
        @click="mode = 'development'"
      >
        本地开发令牌
      </button>
    </div>

    <form @submit.prevent="connect">
      <label v-if="mode === 'token'" class="field field--wide">
        <span>学生访问令牌</span>
        <textarea
          v-model="form.token"
          rows="4"
          placeholder="粘贴由统一身份服务签发的 Bearer JWT"
          required
        />
      </label>

      <template v-else>
        <div class="development-note field--wide">
          <WandSparkles :size="18" />
          <span>
            仅用于本地联调。签名密钥由你手动输入，不会写入仓库或持久化到浏览器。
          </span>
        </div>
        <label class="field">
          <span>学生 ID</span>
          <input v-model="form.studentId" inputmode="numeric" required />
        </label>
        <label class="field">
          <span>开发签名密钥</span>
          <input
            v-model="form.signingKey"
            type="password"
            minlength="32"
            autocomplete="off"
            placeholder="至少 32 个字符"
            required
          />
        </label>
        <label class="field">
          <span>Issuer</span>
          <input v-model="form.issuer" required />
        </label>
        <label class="field">
          <span>Audience</span>
          <input v-model="form.audience" required />
        </label>
      </template>

      <label class="field">
        <span>学生姓名</span>
        <input v-model="form.studentName" placeholder="用于页面展示" />
      </label>
      <label class="field">
        <span>学号</span>
        <input v-model="form.studentNo" placeholder="用于页面展示" />
      </label>
      <label class="field">
        <span>当前学期 ID</span>
        <input v-model="form.termId" inputmode="numeric" required />
      </label>
      <label class="field">
        <span>选课轮次 ID</span>
        <input v-model="form.roundId" inputmode="numeric" required />
      </label>

      <p v-if="errorMessage" class="connect-error field--wide" role="alert">
        {{ errorMessage }}
      </p>

      <button class="connect-submit field--wide" type="submit" :disabled="loading">
        <ShieldCheck :size="18" />
        {{ loading ? '正在建立会话…' : '进入选课系统' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.connect-card {
  width: min(580px, 100%);
  padding: clamp(24px, 4vw, 38px);
  border: 1px solid rgba(255, 255, 255, 0.62);
  border-radius: 26px 26px 26px 8px;
  background: rgba(252, 255, 254, 0.94);
  box-shadow: 0 28px 70px rgba(6, 45, 35, 0.18);
  backdrop-filter: blur(18px);
}

.connect-card header {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 16px;
}

.connect-card__mark {
  display: grid;
  width: 46px;
  height: 46px;
  place-items: center;
  border-radius: 13px;
  color: white;
  background: var(--brand);
}

.connect-card small {
  color: #9a650a;
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.connect-card h1 {
  margin: 5px 0 7px;
  font-family: var(--font-display);
  font-size: 31px;
}

.connect-card header p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}

.connect-tabs {
  display: flex;
  gap: 4px;
  margin: 26px 0 18px;
  padding: 4px;
  border-radius: 10px;
  background: #e9f1ed;
}

.connect-tabs button {
  flex: 1;
  padding: 9px;
  border: 0;
  border-radius: 7px;
  color: var(--muted);
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.connect-tabs button.is-active {
  color: var(--ink);
  background: white;
  box-shadow: 0 3px 10px rgba(9, 48, 38, 0.08);
}

.connect-card form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.field {
  display: grid;
  gap: 6px;
}

.field--wide {
  grid-column: 1 / -1;
}

.field > span {
  color: #536760;
  font-size: 11px;
  font-weight: 700;
}

.field input,
.field textarea {
  width: 100%;
  padding: 11px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  outline: none;
  background: white;
  resize: vertical;
}

.field input:focus,
.field textarea:focus {
  border-color: var(--brand-bright);
  box-shadow: 0 0 0 3px rgba(86, 197, 170, 0.12);
}

.development-note {
  display: flex;
  gap: 9px;
  padding: 11px 13px;
  border-radius: 10px;
  color: #76500f;
  background: #fff1c9;
  font-size: 11px;
  line-height: 1.5;
}

.connect-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: 9px;
  color: #973b33;
  background: #ffefed;
  font-size: 11px;
}

.connect-submit {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px;
  border: 0;
  border-radius: 11px;
  color: white;
  background: var(--brand);
  font-weight: 750;
  cursor: pointer;
}

.connect-submit:disabled {
  opacity: 0.6;
}

@media (max-width: 560px) {
  .connect-card form {
    grid-template-columns: 1fr;
  }

  .field--wide {
    grid-column: auto;
  }
}
</style>
