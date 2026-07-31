<script setup lang="ts">
import { KeyRound, LogIn, ShieldCheck } from '@lucide/vue'
import { reactive, shallowRef } from 'vue'

import { login } from '@/api/auth'
import { useSessionStore } from '@/stores/session'

const emit = defineEmits<{
  authenticated: []
}>()

const session = useSessionStore()
const loading = shallowRef(false)
const errorMessage = shallowRef('')
const form = reactive({
  studentNo: '',
  password: '',
})

async function submit(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await login({
      student_no: form.studentNo.trim(),
      password: form.password,
    })
    session.establish(response)
    emit('authenticated')
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : '登录失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="login-card">
    <header>
      <span class="login-card__mark"><KeyRound :size="21" /></span>
      <div>
        <small>Student sign in</small>
        <h1>登录学生选课系统</h1>
        <p>身份由后端校验，登录成功后签发仅用于学生端接口的访问令牌。</p>
      </div>
    </header>

    <form @submit.prevent="submit">
      <label class="field">
        <span>学号</span>
        <input
          v-model="form.studentNo"
          autocomplete="username"
          maxlength="32"
          placeholder="请输入学号"
          required
        />
      </label>

      <label class="field">
        <span>密码</span>
        <input
          v-model="form.password"
          type="password"
          autocomplete="current-password"
          maxlength="72"
          placeholder="请输入密码"
          required
        />
      </label>

      <div class="security-note">
        <ShieldCheck :size="17" />
        <span>密码只用于本次登录校验，不会保存在浏览器中。</span>
      </div>

      <p v-if="errorMessage" class="login-error" role="alert">
        {{ errorMessage }}
      </p>

      <button class="login-submit" type="submit" :disabled="loading">
        <LogIn :size="18" />
        {{ loading ? '正在登录…' : '登录并进入选课系统' }}
      </button>
    </form>
  </section>
</template>

<style scoped>
.login-card {
  width: min(520px, 100%);
  padding: clamp(26px, 4vw, 40px);
  border: 1px solid rgba(255, 255, 255, 0.62);
  border-radius: 26px 26px 26px 8px;
  background: rgba(252, 255, 254, 0.94);
  box-shadow: 0 28px 70px rgba(6, 45, 35, 0.18);
  backdrop-filter: blur(18px);
}

.login-card header {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 16px;
}

.login-card__mark {
  display: grid;
  width: 46px;
  height: 46px;
  place-items: center;
  border-radius: 13px;
  color: white;
  background: var(--brand);
}

.login-card small {
  color: #9a650a;
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.login-card h1 {
  margin: 5px 0 7px;
  font-family: var(--font-display);
  font-size: 31px;
}

.login-card header p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}

.login-card form {
  display: grid;
  gap: 16px;
  margin-top: 28px;
}

.field {
  display: grid;
  gap: 7px;
}

.field span {
  color: #536760;
  font-size: 11px;
  font-weight: 700;
}

.field input {
  width: 100%;
  padding: 12px 13px;
  border: 1px solid var(--line);
  border-radius: 10px;
  outline: none;
  background: white;
}

.field input:focus {
  border-color: var(--brand-bright);
  box-shadow: 0 0 0 3px rgba(86, 197, 170, 0.12);
}

.security-note {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 11px;
}

.security-note svg {
  color: var(--brand);
}

.login-error {
  margin: 0;
  padding: 10px 12px;
  border-radius: 9px;
  color: #973b33;
  background: #ffefed;
  font-size: 11px;
}

.login-submit {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 13px;
  border: 0;
  border-radius: 11px;
  color: white;
  background: var(--brand);
  font-weight: 750;
  cursor: pointer;
}

.login-submit:disabled {
  opacity: 0.6;
}
</style>
