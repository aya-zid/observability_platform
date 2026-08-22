import client from "@/api/client"

export type AuthResponse = { ok: boolean; role?: string }
export type OkResponse = { ok: boolean }

export type SignupRequest = {
  company_name: string
  business_email: string
  contact_phone?: string
  full_name: string
  password: string
}

export async function login(email: string, password: string): Promise<AuthResponse> {
  const { data } = await client.post<AuthResponse>("/login", { email, password })
  return data
}

export async function signup(req: SignupRequest): Promise<AuthResponse> {
  const { data } = await client.post<AuthResponse>("/signup", req)
  return data
}

export async function refresh(): Promise<AuthResponse> {
  const { data } = await client.post<AuthResponse>("/refresh")
  return data
}

export async function logout(): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/logout")
  return data
}

export async function requestPasswordReset(email: string): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/forgot-password", { email })
  return data
}

export async function validateResetToken(token: string): Promise<{ valid: boolean; email?: string }> {
  const { data } = await client.get(`/reset-password/${token}`)
  return data
}

export async function resetPassword(token: string, password: string): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>(`/reset-password/${token}`, { password })
  return data
}
