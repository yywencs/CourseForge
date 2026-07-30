export interface ApiEnvelope<T> {
  code: number
  info: string
  data: T
}

export class ApiError extends Error {
  readonly code: number
  readonly transport: boolean

  constructor(code: number, message: string, transport = false) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.transport = transport
  }
}

export interface HealthStatus {
  status: 'ok'
}

export interface ReadinessStatus {
  status: 'ready' | 'not_ready'
  failed_checks?: string[]
}

export interface AdminStatus {
  service: string
  status: string
}

export interface ServiceProbe {
  name: string
  description: string
  status: 'healthy' | 'degraded' | 'unknown'
  detail: string
  checkedAt?: string
}
