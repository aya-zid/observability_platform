import { useMutation, useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate, useParams } from "react-router-dom"

import { validateInvitation, acceptInvitation } from "@/api/team"
import { Button } from "@/components/ui/button"

export default function AcceptInvitePage() {
  const { token } = useParams()
  const navigate = useNavigate()
  const [fullName, setFullName] = useState("")
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [error, setError] = useState<string | null>(null)

  const validationQuery = useQuery({
    queryKey: ["invite", token],
    queryFn: () => validateInvitation(token!),
    enabled: !!token,
    retry: 0,
  })

  const acceptMutation = useMutation({
    mutationFn: () => acceptInvitation(token!, fullName, password),
    onSuccess: () => navigate("/login"),
    onError: (e: Error) => setError(e.message),
  })

  if (validationQuery.isLoading) {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Validating invitation...</div>
      </div>
    )
  }

  if (validationQuery.isError || !validationQuery.data?.valid) {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-center max-w-md">
          <h1 className="text-2xl font-semibold">Invalid Invitation</h1>
          <p className="text-muted-foreground mt-2">This invitation link is invalid or has expired.</p>
        </div>
      </div>
    )
  }

  const invite = validationQuery.data

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    if (password !== confirm) {
      setError("Passwords do not match")
      return
    }
    acceptMutation.mutate()
  }

  return (
    <div className="min-h-dvh grid place-items-center p-4">
      <div className="w-full max-w-md">
        <h1 className="text-2xl font-semibold text-center">Join {invite.tenant}</h1>
        <p className="text-sm text-muted-foreground text-center mt-1">
          You've been invited as {invite.email}
        </p>
        <form onSubmit={handleSubmit} className="mt-6 grid gap-4">
          <div>
            <label className="text-sm font-medium">Full Name</label>
            <input
              type="text"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              required
              minLength={2}
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          <div>
            <label className="text-sm font-medium">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          <div>
            <label className="text-sm font-medium">Confirm Password</label>
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
            />
          </div>
          {error && <div className="text-sm text-destructive">{error}</div>}
          <Button type="submit" disabled={acceptMutation.isPending} className="w-full">
            {acceptMutation.isPending ? "Accepting..." : "Accept Invitation"}
          </Button>
        </form>
      </div>
    </div>
  )
}
