import client from "@/api/client"

export type APIKeyResponse = { api_key: string }
export type MeResponse = {
  user_id: number
  tenant_id: number
  full_name: string
  email: string
  role: string
  company_name: string
}

export async function getApiKey(): Promise<APIKeyResponse> {
  const { data } = await client.get<APIKeyResponse>("/api/api-key")
  return data
}

export async function rotateApiKey(): Promise<APIKeyResponse> {
  const { data } = await client.post<APIKeyResponse>("/api/api-key/rotate")
  return data
}

export async function getMe(): Promise<MeResponse> {
  const { data } = await client.get<MeResponse>("/api/me")
  return data
}
