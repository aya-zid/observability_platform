import { useState } from "react"
import { NavLink } from "react-router-dom"

import { cn } from "@/lib/utils"
import { useAuthStore } from "@/store/authStore"

const linkBase =
  "block rounded-md px-3 py-2 text-sm transition-colors hover:bg-muted"

export default function Sidebar() {
  const role = useAuthStore((s) => s.role)
  const roleLoading = role === null
  const canSeeAdmin = roleLoading || role === "admin"
  const [settingsOpen, setSettingsOpen] = useState(false)

  return (
    <aside className="border-b md:border-b-0 md:border-r border-border bg-card">
      <div className="p-4 md:p-6">
        <div className="font-semibold tracking-tight">Observability Platform</div>
        <div className="mt-1 text-xs text-muted-foreground">
          Overview, alerts, hosts, services
        </div>
      </div>

      <nav className="px-3 pb-4 md:px-4 md:pb-6 grid gap-1">
        <NavLink
          to="/overview"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          Overview
        </NavLink>
        <NavLink
          to="/alerts"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          Alerts
        </NavLink>
        <NavLink
          to="/hosts"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          Hosts
        </NavLink>
        <NavLink
          to="/services"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          Services
        </NavLink>

        <div>
          <button
            onClick={() => setSettingsOpen(!settingsOpen)}
            className={cn(linkBase, "w-full text-left flex items-center justify-between")}
          >
            <span>Settings</span>
            <span className={cn(
              "text-xs transition-transform",
              settingsOpen && "rotate-90"
            )}>
              ▶
            </span>
          </button>
          {settingsOpen && (
            <div className="ml-3 mt-1 grid gap-1">
              <NavLink
                to="/settings/general"
                className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
              >
                General
              </NavLink>
              {canSeeAdmin && (
                <NavLink
                  to="/settings/team"
                  className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
                >
                  Team
                </NavLink>
              )}
              {canSeeAdmin && (
                <NavLink
                  to="/settings/api-keys"
                  className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
                >
                  API Keys
                </NavLink>
              )}
              {canSeeAdmin && (
                <NavLink
                  to="/settings/notifications"
                  className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
                >
                  Notifications
                </NavLink>
              )}
              {canSeeAdmin && (
                <NavLink
                  to="/settings/billing"
                  className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
                >
                  Billing
                </NavLink>
              )}
            </div>
          )}
        </div>

        <NavLink
          to="/profile"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          Profile
        </NavLink>
      </nav>
    </aside>
  )
}
