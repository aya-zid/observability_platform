import { Activity } from "lucide-react"
import { cn } from "@/lib/utils"

export default function AuthShell({
  children,
  mode = "login",
  rightPanelClassName,
  variant = "dark",
}: {
  title?: string
  subtitle?: string
  children: React.ReactNode
  footer?: React.ReactNode
  mode?: "login" | "signup"
  rightPanelClassName?: string
  variant?: "dark" | "light"
}) {
  return (
    <div className="min-h-screen flex bg-background">
      {/* Left brand panel */}
      <div
        className={cn(
          "hidden lg:flex flex-col justify-between w-1/2 p-10 relative overflow-hidden",
          variant === "light"
            ? "bg-white text-gray-900 border-r border-gray-200"
            : "bg-sidebar text-sidebar-foreground"
        )}
      >
        {variant === "dark" && (
          <>
            <div className="absolute inset-0 bg-grid opacity-40" />
            <div className="absolute -top-24 -left-24 w-96 h-96 rounded-full bg-sidebar-primary/20 blur-3xl" />
            <div className="absolute bottom-0 right-0 w-96 h-96 rounded-full bg-sidebar-accent/20 blur-3xl" />
          </>
        )}
        <div className="relative flex items-center gap-2.5">
          <div
            className={cn(
              "w-9 h-9 rounded-md flex items-center justify-center ring-1",
              variant === "light"
                ? "bg-gray-100 text-gray-900 ring-gray-200"
                : "bg-sidebar-primary/15 text-sidebar-primary ring-sidebar-primary/30"
            )}
          >
            <Activity className="w-4 h-4" />
          </div>
          <div className="leading-tight">
            <div className="text-[15px] font-bold tracking-tight">
               <span className={variant === "light" ? "text-gray-900" : "text-sidebar-accent"}>Media</span><span className={variant === "light" ? "text-gray-900" : "text-sidebar-primary"}>net</span>
            </div>
            <div className={cn("text-sm uppercase tracking-widest", variant === "light" ? "text-gray-500" : "text-sidebar-foreground/70")}>Observability</div>
          </div>
        </div>
        <div className="relative space-y-4 max-w-md">
          {mode === "login" ? (
            <>
              <h2 className="text-3xl font-bold leading-tight">
                One platform for <span className="text-gradient-brand">hosts, containers &amp; services</span>.
              </h2>
              <p className={cn("text-sm", variant === "light" ? "text-gray-600" : "text-sidebar-foreground/80")}>
                Real-time metrics, logs and correlated insights across your entire fleet.
              </p>
            </>
          ) : (
            <>
              <h2 className="text-3xl font-bold leading-tight">
                Get end-to-end visibility in <span className="text-gradient-brand">under 5 minutes</span>.
              </h2>
              <ul className="space-y-2.5">
                {[
                  "Auto-discovered hosts, containers & services",
                  "Real-time metrics, logs and CVE scanning",
                  "Correlated insights across your fleet",
                ].map((b) => (
                  <li key={b} className={cn("flex items-center gap-2.5 text-sm", variant === "light" ? "text-gray-700" : "text-sidebar-foreground/80")}>
                    <span className={cn("w-5 h-5 rounded-full flex items-center justify-center", variant === "light" ? "bg-gray-100 text-gray-900" : "bg-sidebar-primary/20 text-sidebar-primary")}>
                      <span className="text-sm">✓</span>
                    </span>
                    {b}
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
        <div className={cn("relative text-sm", variant === "light" ? "text-gray-400" : "text-sidebar-foreground/65")}>© 2026 Medianet Observability</div>
      </div>

      {/* Right panel */}
      <div
        className={cn(
          "flex-1 flex items-center justify-center p-6",
          variant === "light" ? "bg-white" : undefined,
          rightPanelClassName
        )}
      >
        <div className="w-full max-w-md">{children}</div>
      </div>
    </div>
  )
}
