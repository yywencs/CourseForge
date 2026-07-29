export interface ApiEnvelope<T> {
  code: number
  info: string
  data: T
}

export class ApiError extends Error {
  readonly code: number

  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}
