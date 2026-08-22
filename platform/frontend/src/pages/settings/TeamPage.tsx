import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { Ban, UserCheck } from "lucide-react"

import { getTeam, inviteTeammate, removeMember, changeRole, setUserStatus, resendInvitation, revokeInvitation } from "@/api/team"
import { SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"

export default function TeamPage() {
  const queryClient = useQueryClient()
  const [showInvite, setShowInvite] = useState(false)
  const [email, setEmail] = useState("")
  const [inviteRole, setInviteRole] = useState("member")

  const teamQuery = useQuery({
    queryKey: ["team"],
    queryFn: getTeam,
  })

  const inviteMutation = useMutation({
    mutationFn: () => inviteTeammate(email, inviteRole),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["team"] })
      setShowInvite(false)
      setEmail("")
    },
  })

  const removeMutation = useMutation({
    mutationFn: (userId: number) => removeMember(userId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["team"] }),
  })

  const roleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: number; role: string }) => changeRole(userId, role),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["team"] }),
  })

  const statusMutation = useMutation({
    mutationFn: ({ userId, status }: { userId: number; status: "active" | "inactive" }) => setUserStatus(userId, status),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["team"] }),
  })

  const resendMutation = useMutation({
    mutationFn: (id: number) => resendInvitation(id),
  })

  const revokeMutation = useMutation({
    mutationFn: (id: number) => revokeInvitation(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["team"] }),
  })

  const team = teamQuery.data

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">Team</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your team members, roles, and invitations.
        </p>
      </div>

      <SectionCard title="Team Members" description="Manage your team members and their roles.">
        {teamQuery.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading team...</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-muted-foreground">
                  <th className="pb-2 font-medium">Name</th>
                  <th className="pb-2 font-medium">Email</th>
                  <th className="pb-2 font-medium">Role</th>
                  <th className="pb-2 font-medium">Status</th>
                  <th className="pb-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {team?.members.map((m) => {
                  const isInactive = m.status === "inactive"
                  return (
                    <tr key={m.id} className="border-b border-border/50">
                      <td className="py-2">{m.full_name}</td>
                      <td className="py-2 text-muted-foreground">{m.email}</td>
                      <td className="py-2">
                        <select
                          value={m.role}
                          onChange={(e) =>
                            roleMutation.mutate({ userId: m.id, role: e.target.value })
                          }
                          className="bg-transparent border border-border rounded px-2 py-1 text-sm"
                        >
                          <option value="member">member</option>
                          <option value="admin">admin</option>
                        </select>
                      </td>
                      <td className="py-2">
                        <span
                          className={
                            isInactive
                              ? "inline-flex items-center gap-1.5 text-xs font-medium px-2 py-1 rounded-full bg-destructive/10 text-destructive"
                              : "inline-flex items-center gap-1.5 text-xs font-medium px-2 py-1 rounded-full bg-green-500/10 text-green-600"
                          }
                        >
                          <span className={`w-1.5 h-1.5 rounded-full ${isInactive ? "bg-destructive" : "bg-green-500"}`} />
                          {isInactive ? "inactive" : "active"}
                        </span>
                      </td>
                      <td className="py-2">
                        {isInactive ? (
                        <div className="flex gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => statusMutation.mutate({ userId: m.id, status: "active" })}
                          >
                            <UserCheck className="w-3.5 h-3.5 mr-1.5" />
                            Activate
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-destructive"
                            onClick={() => removeMutation.mutate(m.id)}
                          >
                            Remove
                          </Button>
                        </div>
                      ) : (
                        <div className="flex gap-2">
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-destructive"
                            onClick={() => statusMutation.mutate({ userId: m.id, status: "inactive" })}
                          >
                            <Ban className="w-3.5 h-3.5 mr-1.5" />
                            Deactivate
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            className="text-destructive"
                            onClick={() => removeMutation.mutate(m.id)}
                          >
                            Remove
                          </Button>
                        </div>
                      )}
                    </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </SectionCard>

      <SectionCard title="Pending Invitations" description="Invitations that have been sent but not yet accepted.">
        {!team?.pending.length ? (
          <div className="text-sm text-muted-foreground">No pending invitations.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-left text-muted-foreground">
                  <th className="pb-2 font-medium">Email</th>
                  <th className="pb-2 font-medium">Role</th>
                  <th className="pb-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {team.pending.map((inv) => (
                  <tr key={inv.id} className="border-b border-border/50">
                    <td className="py-2">{inv.email}</td>
                    <td className="py-2 text-muted-foreground">{inv.role}</td>
                    <td className="py-2 flex gap-2">
                      <Button variant="outline" size="sm" onClick={() => resendMutation.mutate(inv.id)}>Resend</Button>
                      <Button variant="ghost" size="sm" className="text-destructive" onClick={() => revokeMutation.mutate(inv.id)}>Revoke</Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </SectionCard>

      {showInvite && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-md">
            <h3 className="font-semibold text-lg mb-4">Invite Teammate</h3>
            <div className="grid gap-4">
              <div>
                <label className="text-sm text-muted-foreground">Email</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
                  placeholder="colleague@example.com"
                />
              </div>
              <div>
                <label className="text-sm text-muted-foreground">Role</label>
                <select
                  value={inviteRole}
                  onChange={(e) => setInviteRole(e.target.value)}
                  className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
                >
                  <option value="member">Member</option>
                  <option value="admin">Admin</option>
                </select>
              </div>
              <div className="flex gap-2 justify-end">
                <Button variant="outline" onClick={() => setShowInvite(false)}>Cancel</Button>
                <Button onClick={() => inviteMutation.mutate()} disabled={!email}>Send Invite</Button>
              </div>
            </div>
          </div>
        </div>
      )}

      <div>
        <Button onClick={() => setShowInvite(true)}>Invite Teammate</Button>
      </div>
    </div>
  )
}
