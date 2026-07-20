import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { getMe } from "@/api/tenant"
import client from "@/api/client"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"

async function updateProfile(fullName: string) {
  await client.patch("/api/profile", { full_name: fullName })
}

async function changePassword(currentPassword: string, newPassword: string) {
  await client.post("/api/profile/change-password", { current_password: currentPassword, new_password: newPassword })
}

export default function ProfilePage() {
  const queryClient = useQueryClient()
  const q = useQuery({
    queryKey: ["me"],
    queryFn: getMe,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  const [fullName, setFullName] = useState("")
  const [cp, setCp] = useState("")
  const [np, setNp] = useState("")
  const [cnp, setCnp] = useState("")
  const [pwError, setPwError] = useState<string | null>(null)
  const [pwSuccess, setPwSuccess] = useState(false)

  const profileMutation = useMutation({
    mutationFn: () => updateProfile(fullName),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["me"] }),
  })

  const pwMutation = useMutation({
    mutationFn: () => changePassword(cp, np),
    onSuccess: () => {
      setCp("")
      setNp("")
      setCnp("")
      setPwError(null)
      setPwSuccess(true)
    },
    onError: (e: Error) => {
      setPwError(e.message)
      setPwSuccess(false)
    },
  })

  return (
    <Page>
      <PageHeader title="Profile" description="User settings and preferences." />
      <SectionCard title="Personal Info" description="Update your name and view account details.">
        {q.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading profile…</div>
        ) : q.isError ? (
          <div className="text-sm text-destructive">Unable to load profile.</div>
        ) : (
          <div className="grid gap-4 max-w-md">
            <div>
              <label className="text-sm text-muted-foreground">Email</label>
              <div className="font-medium">{q.data?.email}</div>
            </div>
            <div>
              <label className="text-sm text-muted-foreground">Role</label>
              <div className="font-medium capitalize">{q.data?.role}</div>
            </div>
            <div>
              <label className="text-sm text-muted-foreground">Company</label>
              <div className="font-medium">{q.data?.company_name}</div>
            </div>
            <div>
              <label className="text-sm text-muted-foreground">Full Name</label>
              <input
                type="text"
                defaultValue={q.data?.full_name}
                onChange={(e) => setFullName(e.target.value)}
                className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
              />
            </div>
            <div>
              <Button onClick={() => profileMutation.mutate()} disabled={profileMutation.isPending || !fullName}>
                {profileMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        )}
      </SectionCard>

      <SectionCard title="Change Password" description="Update your account password.">
        <div className="grid gap-4 max-w-md">
          <div>
            <label className="text-sm text-muted-foreground">Current Password</label>
            <input
              type="password"
              value={cp}
              onChange={(e) => setCp(e.target.value)}
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          <div>
            <label className="text-sm text-muted-foreground">New Password</label>
            <input
              type="password"
              value={np}
              onChange={(e) => setNp(e.target.value)}
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          <div>
            <label className="text-sm text-muted-foreground">Confirm New Password</label>
            <input
              type="password"
              value={cnp}
              onChange={(e) => setCnp(e.target.value)}
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          {pwError && <div className="text-sm text-destructive">{pwError}</div>}
          {pwSuccess && <div className="text-sm text-green-500">Password changed successfully.</div>}
          <div>
            <Button
              onClick={() => {
                if (np !== cnp) {
                  setPwError("Passwords do not match")
                  return
                }
                pwMutation.mutate()
              }}
              disabled={pwMutation.isPending || !cp || !np || !cnp}
            >
              {pwMutation.isPending ? "Changing..." : "Change Password"}
            </Button>
          </div>
        </div>
      </SectionCard>
    </Page>
  )
}
