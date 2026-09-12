import { useMutation, useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { useNavigate, useParams } from "react-router-dom"

import { validateInvitation, acceptInvitation } from "@/api/team"
import AuthShell from "@/components/auth/AuthShell"
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
      <AuthShell mode="login" variant="light">
        <div className="text-sm text-muted-foreground">Validating invitation...</div>
      </AuthShell>
    )
  }

  if (validationQuery.isError || !validationQuery.data?.valid) {
    return (
      <AuthShell mode="login" variant="light">
        <div className="text-center max-w-md">
          <h1 className="text-2xl font-semibold">Invalid Invitation</h1>
          <p className="text-muted-foreground mt-2">This invitation link is invalid or has expired.</p>
        </div>
      </AuthShell>
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
    <AuthShell mode="login" variant="light">
      <div className="bg-white rounded-2xl border border-gray-200 p-6 shadow-sm">
        <h1 className="text-2xl font-semibold text-center text-gray-900">Join {invite.tenant}</h1>
        <p className="text-sm text-gray-500 text-center mt-1">
          You've been invited as {invite.email}
        </p>
        <form onSubmit={handleSubmit} className="mt-6 grid gap-4">
          <div>
            <label className="text-sm font-medium text-gray-700">Full Name</label>
            <input
              type="text"
              value={fullName}
              onChange={(e) => setFullName(e.target.value)}
              required
              minLength={2}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 bg-white mt-1 text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
            />
          </div>
          <div>
            <label className="text-sm font-medium text-gray-700">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              minLength={8}
              className="w-full border border-gray-300 rounded-lg px-3 py-2 bg-white mt-1 text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
            />
          </div>
          <div>
            <label className="text-sm font-medium text-gray-700">Confirm Password</label>
            <input
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              required
              className="w-full border border-gray-300 rounded-lg px-3 py-2 bg-white mt-1 text-gray-900 placeholder:text-gray-400 focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
            />
          </div>
          {error && <div className="text-sm text-destructive">{error}</div>}
          <Button type="submit" disabled={acceptMutation.isPending} className="w-full">
            {acceptMutation.isPending ? "Accepting..." : "Accept Invitation"}
          </Button>
        </form>
      </div>
    </AuthShell>
  )
}
