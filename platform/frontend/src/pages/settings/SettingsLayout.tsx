import { NavLink, Outlet, useLocation } from "react-router-dom"
import { Building2, Users, Key, Bell, CreditCard, ChevronRight, Settings } from "lucide-react"

import { cn } from "@/lib/utils"
import { useAuthStore } from "@/store/authStore"

const sidebarItems = [
  {
    path: "/settings/general",
    label: "General",
    icon: Building2,
    description: "Organization name, timezone, and region",
    adminOnly: false,
  },
  {
    path: "/settings/team",
    label: "Team",
    icon: Users,
    description: "Manage members and invitations",
    adminOnly: true,
  },
  {
    path: "/settings/api-keys",
    label: "API Keys",
    icon: Key,
    description: "Agent authentication keys",
    adminOnly: true,
  },
  {
    path: "/settings/notifications",
    label: "Notifications",
    icon: Bell,
    description: "Alert notification channels",
    adminOnly: true,
  },
  {
    path: "/settings/billing",
    label: "Billing",
    icon: CreditCard,
    description: "Plan, usage, and invoices",
    adminOnly: true,
  },
]

export default function SettingsLayout() {
  const location = useLocation()
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === "admin"
  const roleLoading = role === null
  const visibleItems = sidebarItems.filter((t) => roleLoading || !t.adminOnly || isAdmin)

  const currentItem = sidebarItems.find((item) => location.pathname === item.path)

  return (
    <div className="flex h-full">
      <aside className="w-72 shrink-0 border-r border-border bg-card/40 overflow-y-auto">
        <div className="p-4 space-y-1.5">
          {visibleItems.map((item) => {
            const active = location.pathname === item.path
            return (
              <NavLink
                key={item.path}
                to={item.path}
                className={cn(
                  "flex items-center gap-3 rounded-xl px-4 py-3.5 transition-all",
                  active
                    ? "bg-primary/10 shadow-accent-glow border border-primary/25"
                    : "text-muted-foreground/80 hover:text-foreground hover:bg-muted/50 border border-transparent"
                )}
              >
                <div
                  className={cn(
                    "w-10 h-10 rounded-xl flex items-center justify-center shrink-0 transition-colors",
                    active
                      ? "bg-gradient-to-br from-primary/30 to-primary/20 text-primary"
                      : "bg-muted text-muted-foreground"
                  )}
                >
                  <item.icon className="w-4.5 h-4.5" />
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className={cn(
                      "text-sm font-semibold leading-tight",
                      active ? "text-foreground" : "text-muted-foreground"
                    )}
                  >
                    {item.label}
                  </div>
                  <div className="text-sm text-muted-foreground/70 mt-0.5 leading-snug">
                    {item.description}
                  </div>
                </div>
                <ChevronRight
                  className={cn(
                    "w-4 h-4 shrink-0 transition-colors",
                    active ? "text-primary" : "text-muted-foreground/30"
                  )}
                />
              </NavLink>
            )
          })}
        </div>
      </aside>

      <div className="flex-1 overflow-y-auto">
        <div className="max-w-4xl mx-auto w-full py-6 px-6 md:px-8">
          <div className="flex items-center gap-1.5 text-xs uppercase tracking-wider mb-6">
            <Settings className="w-3 h-3 text-muted-foreground" />
            <span className="text-muted-foreground">Settings</span>
            <ChevronRight className="w-2.5 h-2.5 text-muted-foreground/50" />
            <span className="text-primary font-semibold tracking-wide">
              {currentItem?.label}
            </span>
          </div>

          <Outlet />
        </div>
      </div>
    </div>
  )
}
