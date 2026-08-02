<script setup lang="ts">
import { ArrowRight, ShieldCheck } from '@lucide/vue'
import { reactive, shallowRef } from 'vue'

import { loginAdministrator } from '@/api/adminAuth'
import { useAdministratorSessionStore } from '@/stores/adminSession'

const emit = defineEmits<{ authenticated: [] }>()
const session = useAdministratorSessionStore()
const loading = shallowRef(false)
const errorMessage = shallowRef('')
const form = reactive({ username: '', password: '' })

async function submit(): Promise<void> {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await loginAdministrator({
      username: form.username.trim(),
      password: form.password,
    })
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
  <section class="administrator-login-card">
    <header class="administrator-login-card__header">
      <span class="administrator-login-card__icon"><ShieldCheck :size="22" /></span>
      <div>
        <h2>管理员登录</h2>
        <p>验证身份后进入教务管理台。</p>
      </div>
    </header>

    <form class="administrator-login-form" @submit.prevent="submit">
      <label class="administrator-login-field">
        <span>用户名</span>
        <input
          v-model="form.username"
          autocomplete="username"
          maxlength="64"
          placeholder="请输入管理员用户名"
          required
        />
      </label>
      <label class="administrator-login-field">
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
      <p v-if="errorMessage" class="administrator-login-error" role="alert">
        {{ errorMessage }}
      </p>
      <button class="administrator-login-submit" type="submit" :disabled="loading">
        <span>{{ loading ? '正在验证…' : '进入教务管理台' }}</span>
        <ArrowRight :size="18" />
      </button>
    </form>
  </section>
</template>

<style scoped>
.administrator-login-card {
  width: min(460px, 100%);
}

.administrator-login-card__header {
  display: grid;
  grid-template-columns: auto 1fr;
  align-items: center;
  gap: 14px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--line);
}

.administrator-login-card__icon {
  display: grid;
  width: 46px;
  height: 46px;
  place-items: center;
  color: var(--brand);
  background: var(--brand-pale);
}

.administrator-login-card h2 {
  margin: 0 0 3px;
  font-family: var(--font-display);
  font-size: 30px;
  font-variation-settings: "wdth" 80, "wght" 680;
  letter-spacing: -0.025em;
}

.administrator-login-card header p {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
}

.administrator-login-form {
  display: grid;
  gap: 18px;
  margin-top: 27px;
}

.administrator-login-field {
  display: grid;
  gap: 7px;
}

.administrator-login-field span {
  font-size: 11px;
  font-weight: 700;
}

.administrator-login-field input {
  width: 100%;
  min-height: 50px;
  padding: 12px 13px;
  border: 1px solid var(--line);
  border-radius: 8px;
  outline: none;
  background: var(--surface);
}

.administrator-login-field input:focus {
  border-color: var(--brand);
  box-shadow: 0 0 0 3px rgba(80, 109, 143, 0.1);
}

.administrator-login-error {
  margin: 0;
  padding: 11px 12px;
  border: 1px solid #e9aaa3;
  border-radius: 8px;
  color: var(--danger);
  background: #fff0ed;
  font-size: 11px;
}

.administrator-login-submit {
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

.administrator-login-submit:hover:not(:disabled) {
  background: var(--brand-hover);
}

.administrator-login-submit:disabled {
  cursor: wait;
  opacity: 0.6;
}
</style>
