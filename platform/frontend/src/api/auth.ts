import client from "@/api/client"

export type OkResponse = { ok: boolean }

export type SignupRequest = {
  company_name: string
  business_email: string
  contact_phone?: string
  full_name: string
  password: string
}

export async function login(email: string, password: string): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/login", { email, password })
  return data
}

export async function signup(req: SignupRequest): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/signup", req)
  return data
}

export async function refresh(): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/refresh")
  return data
}

export async function logout(): Promise<OkResponse> {
  const { data } = await client.post<OkResponse>("/logout")
  return data
}
