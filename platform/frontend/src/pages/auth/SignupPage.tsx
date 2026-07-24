import { zodResolver } from "@hookform/resolvers/zod"
import { useState } from "react"
import { useForm } from "react-hook-form"
import { Link, useNavigate } from "react-router-dom"
import { z } from "zod"
import { Mail, Lock, User, Building2, Phone, Eye, EyeOff, ArrowRight } from "lucide-react"

import * as authApi from "@/api/auth"
import AuthShell from "@/components/auth/AuthShell"
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
  const [showPw, setShowPw] = useState(false)

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
    <AuthShell mode="signup">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-foreground">Create your workspace</h1>
        <p className="text-sm text-muted-foreground mt-1">Start monitoring your infrastructure today</p>
      </div>

      <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
        <Field
          label="Company name" icon={Building2} required
          placeholder="Acme Corp"
          error={form.formState.errors.company?.message}
          {...form.register("company")}
        />
        <Field
          label="Full name" icon={User} required
          placeholder="Jane Doe"
          error={form.formState.errors.full_name?.message}
          {...form.register("full_name")}
        />
        <Field
          label="Work email" icon={Mail} type="email" required
          placeholder="jane@company.com"
          error={form.formState.errors.business_email?.message}
          {...form.register("business_email")}
        />
        <Field
          label="Phone" icon={Phone} type="tel" optional
          placeholder="+1 (555) 000-0000"
          {...form.register("phone")}
        />

        <div>
          <div className="flex items-center justify-between">
            <label className="text-xs font-semibold text-foreground uppercase tracking-wider">Password</label>
          </div>
          <div className="relative mt-1.5">
            <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
            <input
              type={showPw ? "text" : "password"}
              placeholder="At least 8 characters"
              className="w-full pl-9 pr-9 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              {...form.register("password")}
            />
            <button type="button" onClick={() => setShowPw(!showPw)} className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground">
              {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </button>
          </div>
          <p className="text-[11px] text-muted-foreground mt-1">Use at least 8 characters with 1 uppercase, 1 lowercase, and 1 number.</p>
          {form.formState.errors.password ? (
            <p className="text-xs text-destructive mt-1">{form.formState.errors.password.message}</p>
          ) : null}
        </div>

        {error ? <p className="text-xs text-destructive">{error}</p> : null}

        <Button
          type="submit"
          disabled={form.formState.isSubmitting}
          className="w-full gradient-accent text-accent-foreground rounded-md text-sm font-semibold hover:opacity-90 transition-opacity inline-flex items-center justify-center gap-2 h-10"
        >
          {form.formState.isSubmitting ? "Creating account…" : "Create account"}
          {!form.formState.isSubmitting && <ArrowRight className="w-4 h-4" />}
        </Button>

        <p className="text-[11px] text-muted-foreground text-center leading-relaxed">
          By creating an account you agree to our{" "}
          <a className="text-primary hover:underline" href="#">Terms</a> and{" "}
          <a className="text-primary hover:underline" href="#">Privacy Policy</a>.
        </p>
      </form>

      <p className="text-xs text-muted-foreground text-center mt-6">
        Already have an account?{" "}
        <Link to="/login" className="text-primary font-semibold hover:underline">Sign in</Link>
      </p>
    </AuthShell>
  )
}

function Field({
  label, icon: Icon, type = "text", required, optional, error, ...inputProps
}: {
  label: string; icon: React.ComponentType<{ className?: string }>;
  type?: string; required?: boolean; optional?: boolean; error?: string;
} & React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <div>
      <div className="flex items-center justify-between">
        <label className="text-xs font-semibold text-foreground uppercase tracking-wider">{label}</label>
        {optional && <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Optional</span>}
      </div>
      <div className="relative mt-1.5">
        <Icon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
        <input
          type={type}
          className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
          {...inputProps}
        />
      </div>
      {error ? <p className="text-xs text-destructive mt-1">{error}</p> : null}
    </div>
  )
}
