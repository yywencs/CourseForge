export interface StudentProfile {
  id: number
  student_no: string
  student_name: string
}

export interface SelectionContext {
  term_id: number
  round_id: number
}

export interface LoginRequest {
  student_no: string
  password: string
}

export interface LoginResponse {
  access_token: string
  token_type: 'Bearer'
  expires_at: string
  student: StudentProfile
  selection_context?: SelectionContext
}

export interface CurrentSessionResponse {
  student: StudentProfile
  selection_context?: SelectionContext
}

export interface AdministratorProfile {
  id: number
  username: string
}

export interface AdministratorLoginRequest {
  username: string
  password: string
}

export interface AdministratorLoginResponse {
  access_token: string
  token_type: 'Bearer'
  expires_at: string
  administrator: AdministratorProfile
}

export interface AdministratorCurrentSessionResponse {
  administrator: AdministratorProfile
}
