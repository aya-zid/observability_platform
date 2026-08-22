import { useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { z } from "zod"
import { Lock, Eye, EyeOff } from "lucide-react"
import client from "@/api/client"

const schema = z.object({
  current_password: z.string().min(1, "Current password is required"),
  new_password: z.string().min(8, "New password must be at least 8 characters"),
})

type FormData = z.infer<typeof schema>

export default function SettingsPage() {
  const [message, setMessage] = useState("")
  const [error, setError] = useState("")
  const [showPw, setShowPw] = useState(false)

  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
    reset,
  } = useForm<FormData>({
    resolver: zodResolver(schema),
  })

  const onSubmit = async (data: FormData) => {
    setMessage("")
    setError("")
    try {
      await client.post("/auth/change-password", data)
      setMessage("Password changed successfully")
      reset()
    } catch {
      setError("Failed to change password")
    }
  }

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-xl">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Settings</h1>
        <p className="text-muted-foreground mt-1">Manage your admin account</p>
      </div>

      <div className="rounded-lg border border-border bg-card p-6 shadow-card space-y-6">
        <div className="flex items-center gap-3">
          <Lock className="h-5 w-5 text-primary" />
          <h2 className="text-lg font-semibold">Change Password</h2>
        </div>

        {message && <p className="text-sm text-status-green">{message}</p>}
        {error && <p className="text-sm text-destructive">{error}</p>}

        <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
          <div>
            <label className="text-sm font-semibold text-foreground uppercase tracking-wider">
              Current Password
            </label>
            <div className="relative mt-1.5">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type={showPw ? "text" : "password"}
                className="w-full pl-9 pr-9 py-2.5 bg-secondary border border-border rounded-md text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
                {...register("current_password")}
              />
              <button
                type="button"
                onClick={() => setShowPw(!showPw)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              >
                {showPw ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
            {errors.current_password && (
              <p className="text-sm text-destructive mt-1">{errors.current_password.message}</p>
            )}
          </div>
          <div>
            <label className="text-sm font-semibold text-foreground uppercase tracking-wider">
              New Password
            </label>
            <div className="relative mt-1.5">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground" />
              <input
                type="password"
                className="w-full pl-9 pr-3 py-2.5 bg-secondary border border-border rounded-md text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
                {...register("new_password")}
              />
            </div>
            {errors.new_password && (
              <p className="text-sm text-destructive mt-1">{errors.new_password.message}</p>
            )}
          </div>
          <button
            type="submit"
            disabled={isSubmitting}
            className="h-9 px-4 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/80 disabled:opacity-60 transition-colors"
          >
            {isSubmitting ? "Updating…" : "Change Password"}
          </button>
        </form>
      </div>
    </div>
  )
}
