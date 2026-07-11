import { NavLink } from "react-router-dom"

import { cn } from "@/lib/utils"

const linkBase =
  "block rounded-md px-3 py-2 text-sm transition-colors hover:bg-muted"

export default function Sidebar() {
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
        <NavLink
          to="/settings/api-keys"
          className={({ isActive }) => cn(linkBase, isActive && "bg-muted")}
        >
          API Keys
        </NavLink>
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

