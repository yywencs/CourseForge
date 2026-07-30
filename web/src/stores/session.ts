import { defineStore } from 'pinia'
import { computed, reactive, shallowRef } from 'vue'

import { decodeJwtPayload, isJwtExpired } from '@/utils/jwt'

const sessionStorageKey = 'courseforge.student-session'

export interface StudentSessionSnapshot {
  accessToken: string
  studentId: number
  studentName: string
  studentNo: string
  termId: number
  roundId: number
}

function emptySession(): StudentSessionSnapshot {
  return {
    accessToken: '',
    studentId: 0,
    studentName: '',
    studentNo: '',
    termId: 0,
    roundId: 0,
  }
}

function loadSession(): StudentSessionSnapshot {
  if (typeof window === 'undefined') return emptySession()
  try {
    const raw = window.sessionStorage.getItem(sessionStorageKey)
    if (!raw) return emptySession()
    const parsed = JSON.parse(raw) as Partial<StudentSessionSnapshot>
    return {
      accessToken: parsed.accessToken ?? '',
      studentId: Number(parsed.studentId) || 0,
      studentName: parsed.studentName?.trim() ?? '',
      studentNo: parsed.studentNo?.trim() ?? '',
      termId: Number(parsed.termId) || 0,
      roundId: Number(parsed.roundId) || 0,
    }
  } catch {
    return emptySession()
  }
}

function persistSession(snapshot: StudentSessionSnapshot): void {
  if (typeof window === 'undefined') return
  window.sessionStorage.setItem(sessionStorageKey, JSON.stringify(snapshot))
}

export function readStoredAccessToken(): string {
  return loadSession().accessToken
}

export const useSessionStore = defineStore('session', () => {
  const initial = loadSession()
  const accessToken = shallowRef(initial.accessToken)
  const studentId = shallowRef(initial.studentId)
  const studentName = shallowRef(initial.studentName)
  const studentNo = shallowRef(initial.studentNo)
  const context = reactive({
    termId: initial.termId,
    roundId: initial.roundId,
  })

  const isAuthenticated = computed(
    () =>
      Boolean(accessToken.value) &&
      studentId.value > 0 &&
      !isJwtExpired(accessToken.value),
  )
  const initials = computed(() => {
    const value = studentName.value || studentNo.value || String(studentId.value)
    return value.slice(-2)
  })

  function save(): void {
    persistSession({
      accessToken: accessToken.value,
      studentId: studentId.value,
      studentName: studentName.value,
      studentNo: studentNo.value,
      termId: context.termId,
      roundId: context.roundId,
    })
  }

  function connect(input: {
    accessToken: string
    studentName: string
    studentNo: string
    termId: number
    roundId: number
  }): void {
    const token = input.accessToken.trim()
    const payload = decodeJwtPayload(token)
    const identity = payload.student_id ?? payload.user_id ?? payload.sub
    const parsedStudentId = Number(identity)
    if (!Number.isSafeInteger(parsedStudentId) || parsedStudentId <= 0) {
      throw new Error('JWT 中缺少有效的 student_id、user_id 或 sub')
    }
    if (isJwtExpired(token)) {
      throw new Error('JWT 已过期，请获取新令牌')
    }
    if (
      !Number.isSafeInteger(input.termId) ||
      input.termId <= 0 ||
      !Number.isSafeInteger(input.roundId) ||
      input.roundId <= 0
    ) {
      throw new Error('学期 ID 和选课轮次 ID 必须是正整数')
    }

    accessToken.value = token
    studentId.value = parsedStudentId
    studentName.value = input.studentName.trim() || `学生 ${parsedStudentId}`
    studentNo.value = input.studentNo.trim() || String(parsedStudentId)
    context.termId = input.termId
    context.roundId = input.roundId
    save()
  }

  function updateContext(input: {
    studentName: string
    studentNo: string
    termId: number
    roundId: number
  }): void {
    if (
      !Number.isSafeInteger(input.termId) ||
      input.termId <= 0 ||
      !Number.isSafeInteger(input.roundId) ||
      input.roundId <= 0
    ) {
      throw new Error('学期 ID 和选课轮次 ID 必须是正整数')
    }
    studentName.value = input.studentName.trim() || studentName.value
    studentNo.value = input.studentNo.trim() || studentNo.value
    context.termId = input.termId
    context.roundId = input.roundId
    save()
  }

  function disconnect(): void {
    accessToken.value = ''
    studentId.value = 0
    studentName.value = ''
    studentNo.value = ''
    context.termId = 0
    context.roundId = 0
    if (typeof window !== 'undefined') {
      window.sessionStorage.removeItem(sessionStorageKey)
    }
  }

  return {
    accessToken,
    studentId,
    studentName,
    studentNo,
    context,
    isAuthenticated,
    initials,
    connect,
    updateContext,
    disconnect,
  }
})
