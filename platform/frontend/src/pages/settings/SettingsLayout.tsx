import { NavLink, Outlet, useLocation } from "react-router-dom"

import { cn } from "@/lib/utils"
import { useAuthStore } from "@/store/authStore"

const tabs = [
  { path: "/settings/general", label: "General", adminOnly: false },
  { path: "/settings/team", label: "Team", adminOnly: true },
  { path: "/settings/api-keys", label: "API Keys", adminOnly: true },
  { path: "/settings/notifications", label: "Notifications", adminOnly: true },
  { path: "/settings/billing", label: "Billing", adminOnly: true },
]

export default function SettingsLayout() {
  const location = useLocation()
  const role = useAuthStore((s) => s.role)

  const isAdmin = role === "admin"
  const roleLoading = role === null
  const visibleTabs = tabs.filter((t) => roleLoading || !t.adminOnly || isAdmin)

  return (
    <div className="mx-auto w-full max-w-6xl grid gap-6">
      <div>
        <h1 className="text-2xl md:text-3xl font-semibold tracking-tight">Settings</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Manage your organization, team, and preferences.
        </p>
      </div>

      <div className="flex gap-1 border-b border-border overflow-x-auto">
        {visibleTabs.map((tab) => (
          <NavLink
            key={tab.path}
            to={tab.path}
            className={cn(
              "px-4 py-2 text-sm font-medium border-b-2 -mb-[1px] whitespace-nowrap transition-colors",
              location.pathname === tab.path
                ? "border-primary text-primary"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {tab.label}
          </NavLink>
        ))}
      </div>

      <Outlet />
    </div>
  )
}
