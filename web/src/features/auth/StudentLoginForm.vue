<script setup lang="ts">
import { ArrowRight, LockKeyhole } from '@lucide/vue'
import { reactive, shallowRef } from 'vue'

import { login } from '@/api/auth'
import { useSessionStore } from '@/stores/session'

const emit = defineEmits<{ authenticated: [] }>()
const session = useSessionStore()
const loading = shallowRef(false)
const errorMessage = shallowRef('')
const form = reactive({ studentNo: '', password: '' })

async function submit(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await login({ student_no: form.studentNo.trim(), password: form.password })
    session.establish(response)
    emit('authenticated')
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '登录失败，请稍后重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="login-card">
    <header>
      <span><LockKeyhole :size="21" /></span>
      <div><h2>学生登录</h2><p>进入本轮选课与个人课表。</p></div>
    </header>

    <form @submit.prevent="submit">
      <label class="field">
        <span>学号</span>
        <input v-model="form.studentNo" autocomplete="username" maxlength="32" placeholder="请输入学号" required />
      </label>
      <label class="field">
        <span>密码</span>
        <input v-model="form.password" type="password" autocomplete="current-password" maxlength="72" placeholder="请输入密码" required />
      </label>
      <p v-if="errorMessage" class="login-error" role="alert">{{ errorMessage }}</p>
      <button class="login-submit" type="submit" :disabled="loading">
        <span>{{ loading ? '正在登录…' : '登录并进入选课' }}</span>
        <ArrowRight :size="18" />
      </button>
    </form>
  </section>
</template>

<style scoped>
.login-card {
  width: min(480px, 100%);
}

.login-card header {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 14px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--line);
}

.login-card header > span {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  color: var(--brand);
  background: var(--brand-pale);
}

.login-card h2 {
  margin: 0 0 3px;
  font-family: var(--font-display);
  font-size: 30px;
  font-variation-settings: "wdth" 80, "wght" 680;
  letter-spacing: -0.025em;
}

.login-card header p {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
}

.login-card form {
  display: grid;
  gap: 18px;
  margin-top: 27px;
}

.field {
  display: grid;
  gap: 7px;
}

.field span {
  font-size: 11px;
  font-weight: 700;
}

.field input {
  width: 100%;
  min-height: 50px;
  padding: 12px 13px;
  border: 1px solid var(--line);
  border-radius: 8px;
  outline: none;
  background: var(--surface);
}

.field input:focus {
  border-color: var(--brand);
  box-shadow: 0 0 0 3px rgba(80, 109, 143, 0.1);
}

.login-error {
  margin: 0;
  padding: 11px 12px;
  border: 1px solid #e9aaa3;
  border-radius: 8px;
  color: var(--danger);
  background: #fff0ed;
  font-size: 11px;
}

.login-submit {
  display: flex;
  min-height: 52px;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border: 1px solid var(--brand);
  border-radius: 8px;
  color: white;
  background: var(--brand);
  font-weight: 750;
  cursor: pointer;
}

.login-submit:hover:not(:disabled) {
  background: var(--brand-hover);
}

.login-submit:disabled {
  opacity: 0.6;
}
</style>
