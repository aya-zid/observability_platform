import { zodResolver } from "@hookform/resolvers/zod"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Link, useNavigate } from "react-router-dom"
import { z } from "zod"

import * as authApi from "@/api/auth"
import AuthShell, { AuthLabel } from "@/components/auth/AuthShell"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/store/authStore"

const schema = z.object({
  email: z.string().email(),
  password: z.string().min(1, "Password is required"),
})

type FormValues = z.infer<typeof schema>

export default function LoginPage() {
  const navigate = useNavigate()
  const setStatus = useAuthStore((s) => s.setStatus)
  const [error, setError] = useState<string | null>(null)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: "", password: "" },
  })

  async function onSubmit(values: FormValues) {
    setError(null)
    try {
      await authApi.login(values.email, values.password)
      setStatus("authenticated")
      navigate("/overview", { replace: true })
    } catch {
      setStatus("unauthenticated")
      setError("Invalid email or password.")
    }
  }

  return (
    <AuthShell
      title="Sign in"
      subtitle="Use your business email to access your tenant."
      footer={
        <>
          No account?{" "}
          <Link className="text-foreground underline" to="/signup">
            Create one
          </Link>
          .
        </>
      }
    >
        <form
          className="grid gap-4"
          onSubmit={form.handleSubmit(onSubmit)}
        >
          <label className="grid gap-1">
            <AuthLabel required>Business email</AuthLabel>
            <input
              className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              type="email"
              autoComplete="email"
              {...form.register("email")}
            />
            {form.formState.errors.email ? (
              <span className="text-xs text-destructive">
                {form.formState.errors.email.message}
              </span>
            ) : null}
          </label>

          <label className="grid gap-1">
            <AuthLabel required>Password</AuthLabel>
            <input
              className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
              type="password"
              autoComplete="current-password"
              {...form.register("password")}
            />
            {form.formState.errors.password ? (
              <span className="text-xs text-destructive">
                {form.formState.errors.password.message}
              </span>
            ) : null}
          </label>

          {error ? (
            <div className="text-sm text-destructive">{error}</div>
          ) : null}

          <Button disabled={form.formState.isSubmitting} type="submit">
            {form.formState.isSubmitting ? "Signing in…" : "Sign in"}
          </Button>
        </form>
    </AuthShell>
  )
}
