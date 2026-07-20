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
  business_email: z.string().email("Enter a valid business email"),
  full_name: z.string().min(2, "Full name is required"),
  company: z.string().min(2, "Company name is required"),
  password: z
    .string()
    .min(8, "Use at least 8 characters")
    .regex(
      /^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{8,}$/,
      "Must contain at least 1 uppercase letter, 1 lowercase letter, and 1 number"
    ),
  phone: z.string().optional(),
})

type FormValues = z.infer<typeof schema>

export default function SignupPage() {
  const navigate = useNavigate()
  const setStatus = useAuthStore((s) => s.setStatus)
  const setRole = useAuthStore((s) => s.setRole)
  const [error, setError] = useState<string | null>(null)

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      business_email: "",
      full_name: "",
      company: "",
      password: "",
      phone: "",
    },
  })

  async function onSubmit(values: FormValues) {
    setError(null)
    try {
      const res = await authApi.signup({
        company_name: values.company,
        business_email: values.business_email,
        contact_phone: values.phone ? values.phone : undefined,
        full_name: values.full_name,
        password: values.password,
      })
      setStatus("authenticated")
      if (res.role) setRole(res.role)
      navigate("/overview", { replace: true })
    } catch {
      setStatus("unauthenticated")
      setError("Signup failed. Check your inputs and try again.")
    }
  }

  return (
    <AuthShell
      title="Create your tenant"
      subtitle="Your business email is used as both tenant contact and admin user email."
      footer={
        <>
          Already have an account?{" "}
          <Link className="text-foreground underline" to="/login">
            Sign in
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
              {...form.register("business_email")}
            />
            {form.formState.errors.business_email ? (
              <span className="text-xs text-destructive">
                {form.formState.errors.business_email.message}
              </span>
            ) : null}
          </label>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="grid gap-1">
              <AuthLabel required>Full name</AuthLabel>
              <input
                className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                {...form.register("full_name")}
              />
              {form.formState.errors.full_name ? (
                <span className="text-xs text-destructive">
                  {form.formState.errors.full_name.message}
                </span>
              ) : null}
            </label>

            <label className="grid gap-1">
              <AuthLabel required>Company</AuthLabel>
              <input
                className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                {...form.register("company")}
              />
              {form.formState.errors.company ? (
                <span className="text-xs text-destructive">
                  {form.formState.errors.company.message}
                </span>
              ) : null}
            </label>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="grid gap-1">
              <AuthLabel required>Password</AuthLabel>
              <input
                className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                type="password"
                autoComplete="new-password"
                {...form.register("password")}
              />
              <span className="text-xs text-muted-foreground">
                Use at least 8 characters containing at least 1 uppercase letter, 1 lowercase letter, and 1 number.
              </span>
              {form.formState.errors.password ? (
                <span className="text-xs text-destructive">
                  {form.formState.errors.password.message}
                </span>
              ) : null}
            </label>

            <label className="grid gap-1">
              <AuthLabel>Phone</AuthLabel>
              <input
                className="h-10 rounded-md border border-input bg-background/70 px-3 text-sm outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
                autoComplete="tel"
                {...form.register("phone")}
              />
            </label>
          </div>

          {error ? (
            <div className="text-sm text-destructive">{error}</div>
          ) : null}

          <Button disabled={form.formState.isSubmitting} type="submit">
            {form.formState.isSubmitting ? "Creating…" : "Create tenant"}
          </Button>
        </form>
    </AuthShell>
  )
}
