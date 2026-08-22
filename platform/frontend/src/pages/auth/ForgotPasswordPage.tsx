import { zodResolver } from "@hookform/resolvers/zod"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Link } from "react-router-dom"
import { z } from "zod"
import { Mail, ArrowRight, CheckCircle2 } from "lucide-react"

import * as authApi from "@/api/auth"
import AuthShell from "@/components/auth/AuthShell"
import { Button } from "@/components/ui/button"

const schema = z.object({
  email: z.string().email("Enter a valid email address"),
})

type FormValues = z.infer<typeof schema>

export default function ForgotPasswordPage() {
  const [error, setError] = useState<string | null>(null)
  const [submitted, setSubmitted] = useState(false)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { email: "" },
  })

  async function onSubmit(values: FormValues) {
    setError(null)
    try {
      await authApi.requestPasswordReset(values.email)
      setSubmitted(true)
    } catch {
      setError("Something went wrong. Please try again.")
    }
  }

  return (
    <AuthShell mode="signup">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-foreground">Forgot your password?</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Enter the email you registered with and we'll send you a reset link.
        </p>
      </div>

      {submitted ? (
        <div className="space-y-4">
          <div className="flex items-start gap-3 rounded-md border border-border bg-secondary p-4">
            <CheckCircle2 className="w-5 h-5 text-emerald-500 mt-0.5 shrink-0" />
            <div className="text-sm text-foreground">
              <p className="font-semibold">Check your inbox</p>
              <p className="text-muted-foreground mt-1 leading-relaxed">
                If an account exists for that email, we've sent a password reset link. It expires
                in 1 hour. If you don't see it, check your spam folder.
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
        <>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
            <div>
              <label className="text-sm font-semibold text-foreground uppercase tracking-wider">Email</label>
              <div className="relative mt-1.5">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
                <input
                  type="email"
                  autoComplete="email"
                  placeholder="you@company.com"
                  className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
                  {...form.register("email")}
                />
              </div>
              {form.formState.errors.email ? (
                <p className="text-sm text-destructive mt-1">{form.formState.errors.email.message}</p>
              ) : null}
            </div>

            {error ? <p className="text-sm text-destructive">{error}</p> : null}

            <Button
              type="submit"
              disabled={form.formState.isSubmitting}
              className="w-full gradient-accent text-accent-foreground rounded-md text-sm font-semibold hover:opacity-90 transition-opacity inline-flex items-center justify-center gap-2 h-10"
            >
              {form.formState.isSubmitting ? "Sending…" : "Send reset link"}
              {!form.formState.isSubmitting && <ArrowRight className="w-4 h-4" />}
            </Button>
          </form>

          <p className="text-sm text-muted-foreground text-center mt-6">
            Remembered your password?{" "}
            <Link to="/login" className="text-primary font-semibold hover:underline">
              Back to sign in
            </Link>
          </p>
        </>
      )}
    </AuthShell>
  )
}