import client from "@/api/client"

export type TeamMember = {
  id: number
  full_name: string
  email: string
  role: string
  created_at: string
}

export type PendingInvite = {
  id: number
  email: string
  role: string
  invite_url: string
  expires_at: string
  created_at: string
}

export type TeamResponse = {
  members: TeamMember[]
  pending: PendingInvite[]
}

export type InviteResponse = {
  invitation_id: number
  invite_url: string
}

export async function getTeam(): Promise<TeamResponse> {
  const { data } = await client.get<TeamResponse>("/api/settings/team")
  return data
}

export async function inviteTeammate(email: string, role: string): Promise<InviteResponse> {
  const { data } = await client.post<InviteResponse>("/api/settings/team/invite", { email, role })
  return data
}

export async function removeMember(userId: number): Promise<void> {
  await client.delete(`/api/settings/team/${userId}`)
}

export async function changeRole(userId: number, role: string): Promise<void> {
  await client.patch(`/api/settings/team/${userId}/role`, { role })
}

export async function resendInvitation(id: number): Promise<{ invite_url: string; expires_at: string }> {
  const { data } = await client.post(`/api/settings/team/invitations/${id}/resend`)
  return data
}

export async function revokeInvitation(id: number): Promise<void> {
  await client.delete(`/api/settings/team/invitations/${id}`)
}

export async function validateInvitation(token: string): Promise<{ valid: boolean; email: string; tenant: string }> {
  const { data } = await client.get(`/invite/${token}`)
  return data
}

export async function acceptInvitation(token: string, fullName: string, password: string): Promise<void> {
  await client.post(`/invite/${token}/accept`, { full_name: fullName, password })
}
