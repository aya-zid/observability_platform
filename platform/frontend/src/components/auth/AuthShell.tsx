import { Activity } from "lucide-react"

export default function AuthShell({
  children,
  mode = "login",
}: {
  title?: string
  subtitle?: string
  children: React.ReactNode
  footer?: React.ReactNode
  mode?: "login" | "signup"
}) {
  return (
    <div className="min-h-screen flex bg-background">
      {/* Left brand panel */}
      <div className="hidden lg:flex flex-col justify-between w-1/2 p-10 bg-sidebar text-sidebar-foreground relative overflow-hidden">
        <div className="absolute inset-0 bg-grid opacity-40" />
        <div className="absolute -top-24 -left-24 w-96 h-96 rounded-full bg-sidebar-primary/20 blur-3xl" />
        <div className="absolute bottom-0 right-0 w-96 h-96 rounded-full bg-sidebar-accent/20 blur-3xl" />
        <div className="relative flex items-center gap-2.5">
          <div className="w-9 h-9 rounded-md bg-sidebar-primary/15 flex items-center justify-center ring-1 ring-sidebar-primary/30">
            <Activity className="w-4 h-4 text-sidebar-primary" />
          </div>
          <div className="leading-tight">
            <div className="text-[15px] font-bold tracking-tight">
               <span className="text-sidebar-accent">Media</span><span className="text-sidebar-primary">net</span>
            </div>
            <div className="text-sm uppercase tracking-widest text-sidebar-foreground/70">Observability</div>
          </div>
        </div>
        <div className="relative space-y-4 max-w-md">
          {mode === "login" ? (
            <>
              <h2 className="text-3xl font-bold leading-tight">
                One platform for <span className="text-gradient-brand">hosts, containers &amp; services</span>.
              </h2>
              <p className="text-sm text-sidebar-foreground/80">
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
                  <li key={b} className="flex items-center gap-2.5 text-sm text-sidebar-foreground/80">
                    <span className="w-5 h-5 rounded-full bg-sidebar-primary/20 flex items-center justify-center">
                      <span className="text-sidebar-primary text-sm">✓</span>
                    </span>
                    {b}
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
        <div className="relative text-sm text-sidebar-foreground/65">© 2026 Medianet Observability</div>
      </div>

      {/* Right panel */}
      <div className="flex-1 flex items-center justify-center p-6">
        <div className="w-full max-w-md">{children}</div>
      </div>
    </div>
  )
}
