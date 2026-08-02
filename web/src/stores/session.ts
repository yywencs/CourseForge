import { defineStore } from 'pinia'
import { computed, reactive, shallowRef } from 'vue'

import { decodeJwtPayload, isJwtExpired } from '@/utils/jwt'
import type { LoginResponse } from '@/types/auth'

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
    () => {
      if (!accessToken.value || studentId.value <= 0 || isJwtExpired(accessToken.value)) {
        return false
      }
      try {
        const payload = decodeJwtPayload(accessToken.value)
        return payload.actor_type === 'student' && Number(payload.sub) === studentId.value
      } catch {
        return false
      }
    },
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

  function establish(input: LoginResponse): void {
    const token = input.access_token.trim()
    const payload = decodeJwtPayload(token)
    const identity = payload.sub
    const parsedStudentId = Number(identity)
    if (payload.actor_type !== 'student') {
      throw new Error('登录凭证不是学生身份')
    }
    if (!Number.isSafeInteger(parsedStudentId) || parsedStudentId <= 0) {
      throw new Error('登录凭证中缺少有效的学生身份')
    }
    if (isJwtExpired(token)) {
      throw new Error('登录已过期，请重新登录')
    }
    if (parsedStudentId !== input.student.id) {
      throw new Error('登录身份信息不一致')
    }

    accessToken.value = token
    studentId.value = parsedStudentId
    studentName.value =
      input.student.student_name.trim() || `学生 ${parsedStudentId}`
    studentNo.value = input.student.student_no.trim()
    context.termId = input.selection_context?.term_id ?? 0
    context.roundId = input.selection_context?.round_id ?? 0
    save()
  }

  function updateContext(input: {
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
    establish,
    updateContext,
    disconnect,
  }
})
