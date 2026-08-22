import { useMutation, useQuery } from "@tanstack/react-query"
import { useState } from "react"
import { Link, useParams } from "react-router-dom"
import { z } from "zod"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
import { Lock, Eye, EyeOff, ArrowRight, CheckCircle2 } from "lucide-react"

import { validateResetToken, resetPassword } from "@/api/auth"
import AuthShell from "@/components/auth/AuthShell"
import { Button } from "@/components/ui/button"

const schema = z.object({
  password: z
    .string()
    .min(8, "Use at least 8 characters")
    .regex(
      /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$/,
      "Must contain at least 1 uppercase letter, 1 lowercase letter, and 1 number"
    ),
  confirm: z.string(),
}).refine((v) => v.password === v.confirm, {
  message: "Passwords do not match",
  path: ["confirm"],
})

type FormValues = z.infer<typeof schema>

export default function ResetPasswordPage() {
  const { token } = useParams()
  const [error, setError] = useState<string | null>(null)
  const [showPw, setShowPw] = useState(false)
  const [done, setDone] = useState(false)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { password: "", confirm: "" },
  })

  const validationQuery = useQuery({
    queryKey: ["reset-token", token],
    queryFn: () => validateResetToken(token!),
    enabled: !!token,
    retry: 0,
  })

  const resetMutation = useMutation({
    mutationFn: (password: string) => resetPassword(token!, password),
    onSuccess: () => setDone(true),
    onError: () => setError("This reset link is invalid or has expired. Request a new one."),
  })

  async function onSubmit(values: FormValues) {
    setError(null)
    resetMutation.mutate(values.password)
  }

  if (validationQuery.isLoading) {
    return (
      <div className="min-h-dvh grid place-items-center">
        <div className="text-sm text-muted-foreground">Validating link…</div>
      </div>
    )
  }

  if (validationQuery.isError || !validationQuery.data?.valid) {
    return (
      <div className="min-h-dvh grid place-items-center p-4">
        <div className="text-center max-w-md">
          <h1 className="text-2xl font-semibold">Invalid or Expired Link</h1>
          <p className="text-muted-foreground mt-2">
            This password reset link is invalid or has expired. Please request a new one.
          </p>
          <Link
            to="/forgot-password"
            className="mt-6 inline-block text-sm text-primary font-semibold hover:underline"
          >
            Request a new reset link
          </Link>
        </div>
      </div>
    )
  }

  return (
    <AuthShell>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-foreground">Choose a new password</h1>
        <p className="text-sm text-muted-foreground mt-1">
          {validationQuery.data.email ? `For ${validationQuery.data.email}` : "Set a new password for your account"}
        </p>
      </div>

      {done ? (
        <div className="space-y-4">
          <div className="flex items-start gap-3 rounded-md border border-border bg-secondary p-4">
            <CheckCircle2 className="w-5 h-5 text-emerald-500 mt-0.5 shrink-0" />
            <div className="text-sm text-foreground">
              <p className="font-semibold">Password updated</p>
              <p className="text-muted-foreground mt-1 leading-relaxed">
                Your password has been changed. You can now sign in with your new password.
              </p>
            </div>
          </div>
          <Link
            to="/login"
            className="block w-full text-center text-sm text-primary font-semibold hover:underline"
          >
            Back to sign in
          </Link>
        </div>
      ) : (
        <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label className="text-sm font-semibold text-foreground uppercase tracking-wider">New password</label>
            <div className="relative mt-1.5">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type={showPw ? "text" : "password"}
                autoComplete="new-password"
                placeholder="At least 8 characters"
                className="w-full pl-9 pr-9 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
                {...form.register("password")}
              />
              <button type="button" onClick={() => setShowPw(!showPw)} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
                {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
            <p className="text-sm text-muted-foreground mt-1">Use at least 8 characters with 1 uppercase, 1 lowercase, and 1 number.</p>
            {form.formState.errors.password ? (
              <p className="text-sm text-destructive mt-1">{form.formState.errors.password.message}</p>
            ) : null}
          </div>

          <div>
            <label className="text-sm font-semibold text-foreground uppercase tracking-wider">Confirm password</label>
            <div className="relative mt-1.5">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type={showPw ? "text" : "password"}
                autoComplete="new-password"
                placeholder="Re-enter your password"
                className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
                {...form.register("confirm")}
              />
            </div>
            {form.formState.errors.confirm ? (
              <p className="text-sm text-destructive mt-1">{form.formState.errors.confirm.message}</p>
            ) : null}
          </div>

          {error ? <p className="text-sm text-destructive">{error}</p> : null}

          <Button
            type="submit"
            disabled={form.formState.isSubmitting || resetMutation.isPending}
            className="w-full gradient-accent text-accent-foreground rounded-md text-sm font-semibold hover:opacity-90 transition-opacity inline-flex items-center justify-center gap-2 h-10"
          >
            {resetMutation.isPending ? "Updating…" : "Update password"}
            {!resetMutation.isPending && <ArrowRight className="w-4 h-4" />}
          </Button>
        </form>
      )}
    </AuthShell>
  )
}