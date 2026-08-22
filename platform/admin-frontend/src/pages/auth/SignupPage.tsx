import { zodResolver } from "@hookform/resolvers/zod"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Link } from "react-router-dom"
import { z } from "zod"
import { Mail, User, Lock, ArrowRight } from "lucide-react"

import client from "@/api/client"
import { useAdminAuthStore } from "@/store/adminAuthStore"
import AuthShell from "@/components/auth/AuthShell"

const schema = z.object({
  full_name: z.string().min(2, "Name must be at least 2 characters"),
  email: z.string().email("Invalid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
})

type FormValues = z.infer<typeof schema>

export default function SignupPage() {
  const setStatus = useAdminAuthStore((s) => s.setStatus)
  const [error, setError] = useState<string | null>(null)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: { full_name: "", email: "", password: "" },
  })

  async function onSubmit(values: FormValues) {
    setError(null)
    try {
      await client.post("/auth/signup", values)
      setStatus("authenticated")
      window.location.href = "/dashboard"
    } catch {
      setError("Signup failed. Please try again.")
    }
  }

  return (
    <AuthShell mode="signup">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-foreground">Create admin account</h1>
        <p className="text-sm text-muted-foreground mt-1">Get started with the admin panel</p>
      </div>

      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <div>
          <label className="text-sm font-semibold text-foreground uppercase tracking-wider">Full Name</label>
          <div className="relative mt-1.5">
            <User className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="John Doe"
              className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              {...form.register("full_name")}
            />
          </div>
          {form.formState.errors.full_name && (
            <p className="text-sm text-destructive mt-1">{form.formState.errors.full_name.message}</p>
          )}
        </div>

        <div>
          <label className="text-sm font-semibold text-foreground uppercase tracking-wider">Email</label>
          <div className="relative mt-1.5">
            <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="email"
              placeholder="admin@example.com"
              className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              {...form.register("email")}
            />
          </div>
          {form.formState.errors.email && (
            <p className="text-sm text-destructive mt-1">{form.formState.errors.email.message}</p>
          )}
        </div>

        <div>
          <label className="text-sm font-semibold text-foreground uppercase tracking-wider">Password</label>
          <div className="relative mt-1.5">
            <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type="password"
              placeholder="Min 8 characters"
              className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              {...form.register("password")}
            />
          </div>
          {form.formState.errors.password && (
            <p className="text-sm text-destructive mt-1">{form.formState.errors.password.message}</p>
          )}
        </div>

        {error && <p className="text-sm text-destructive">{error}</p>}

        <button
          type="submit"
          disabled={form.formState.isSubmitting}
          className="w-full gradient-accent text-accent-foreground rounded-md text-sm font-semibold hover:opacity-90 transition-opacity inline-flex items-center justify-center gap-2 h-10 disabled:opacity-60"
        >
          {form.formState.isSubmitting ? "Creating account…" : "Sign up"}
          {!form.formState.isSubmitting && <ArrowRight className="w-4 h-4" />}
        </button>
      </form>

      <p className="text-sm text-muted-foreground text-center mt-6">
        Already have an account?{" "}
        <Link to="/login" className="text-primary font-semibold hover:underline">
          Sign in
        </Link>
      </p>
    </AuthShell>
  )
}
