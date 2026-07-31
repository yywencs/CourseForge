<script setup lang="ts">
import { LogOut, Save, ShieldCheck } from '@lucide/vue'
import { reactive, shallowRef } from 'vue'
import { useRouter } from 'vue-router'

import { useEnrollmentTrackerStore } from '@/stores/enrollmentTracker'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const tracker = useEnrollmentTrackerStore()
const router = useRouter()
const saved = shallowRef(false)
const errorMessage = shallowRef('')
const form = reactive({
  termId: String(session.context.termId),
  roundId: String(session.context.roundId),
})

function save(): void {
  saved.value = false
  errorMessage.value = ''
  try {
    session.updateContext({
      termId: Number(form.termId),
      roundId: Number(form.roundId),
    })
    saved.value = true
  } catch (error) {
    errorMessage.value =
      error instanceof Error ? error.message : '保存学生上下文失败'
  }
}

function disconnect(): void {
  session.disconnect()
  tracker.clear()
  void router.replace('/login')
}
</script>

<template>
  <section class="account-panel">
    <header>
      <span><ShieldCheck :size="20" /></span>
      <div>
        <small>Authenticated student</small>
        <h2>学生身份与选课上下文</h2>
        <p>学生 ID 来自 JWT，不允许在页面修改；学期和轮次用于列表查询与提交。</p>
      </div>
    </header>

    <div class="identity-strip">
      <span>{{ session.studentName }} · {{ session.studentNo }}</span>
      <strong>学生 ID {{ session.studentId }}</strong>
      <i>后端验签</i>
    </div>

    <form @submit.prevent="save">
      <label>
        <span>当前学期 ID</span>
        <input v-model="form.termId" inputmode="numeric" />
      </label>
      <label>
        <span>选课轮次 ID</span>
        <input v-model="form.roundId" inputmode="numeric" />
      </label>

      <p v-if="saved" class="save-state is-success">上下文已保存</p>
      <p v-else-if="errorMessage" class="save-state is-error">{{ errorMessage }}</p>

      <div class="account-actions">
        <button class="save-button" type="submit"><Save :size="16" />保存设置</button>
        <button class="disconnect-button" type="button" @click="disconnect">
          <LogOut :size="16" />清除会话并退出
        </button>
      </div>
    </form>
  </section>
</template>

<style scoped>
.account-panel {
  padding: clamp(24px, 4vw, 36px);
  border: 1px solid var(--line);
  border-radius: var(--radius-panel);
  background: var(--surface);
  box-shadow: var(--shadow-card);
}

.account-panel header {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 15px;
}

.account-panel header > span {
  display: grid;
  width: 44px;
  height: 44px;
  place-items: center;
  border-radius: 12px;
  color: var(--brand);
  background: #def2eb;
}

.account-panel small {
  color: #98630c;
  font-family: var(--font-mono);
  font-size: 9px;
  font-weight: 750;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.account-panel h2 {
  margin: 5px 0;
  font-family: var(--font-display);
  font-size: 28px;
}

.account-panel header p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.identity-strip {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 26px 0;
  padding: 15px;
  border-radius: 12px;
  background: var(--surface-muted);
}

.identity-strip span {
  color: var(--muted);
  font-size: 11px;
}

.identity-strip strong {
  font-family: var(--font-mono);
}

.identity-strip i {
  margin-left: auto;
  padding: 4px 8px;
  border-radius: 999px;
  color: var(--success);
  background: #d9f3e9;
  font-size: 10px;
  font-style: normal;
}

.account-panel form {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.account-panel label {
  display: grid;
  gap: 6px;
}

.account-panel label span {
  color: #536760;
  font-size: 11px;
  font-weight: 700;
}

.account-panel input {
  width: 100%;
  padding: 11px 12px;
  border: 1px solid var(--line);
  border-radius: 10px;
  outline: 0;
}

.account-panel input:focus {
  border-color: var(--brand-bright);
}

.save-state,
.account-actions {
  grid-column: 1 / -1;
}

.save-state {
  margin: 0;
  padding: 10px 12px;
  border-radius: 9px;
  font-size: 11px;
}

.save-state.is-success {
  color: var(--success);
  background: #def5ec;
}

.save-state.is-error {
  color: var(--danger);
  background: #fff0ed;
}

.account-actions {
  display: flex;
  gap: 10px;
  padding-top: 8px;
}

.account-actions button {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 10px 13px;
  border-radius: 9px;
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.save-button {
  border: 1px solid var(--brand);
  color: white;
  background: var(--brand);
}

.disconnect-button {
  border: 1px solid #e4c9c5;
  color: var(--danger);
  background: #fff8f6;
}

@media (max-width: 580px) {
  .account-panel form {
    grid-template-columns: 1fr;
  }

  .save-state,
  .account-actions {
    grid-column: auto;
  }

  .account-actions {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
