import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export const useSessionStore = defineStore('session', () => {
  // 登录能力接入前使用开发学生，后续由鉴权响应覆盖。
  const studentId = ref(10001)
  const studentName = ref('林知夏')
  const studentNo = ref('2026001001')

  const initials = computed(() => studentName.value.slice(-2))

  return {
    studentId,
    studentName,
    studentNo,
    initials,
  }
})
