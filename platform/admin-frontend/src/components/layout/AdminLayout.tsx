import { useState, useEffect } from "react"
import { Outlet, NavLink } from "react-router-dom"
import { useAdminAuthStore } from "@/store/adminAuthStore"
import client from "@/api/client"
import {
  LayoutDashboard, Users, Package, Settings, LogOut, Activity, CreditCard
} from "lucide-react"
import { cn } from "@/lib/utils"

export function AdminLayout() {
  const clearSession = useAdminAuthStore((s) => s.clearSession)
  const [pendingCount, setPendingCount] = useState<number>(0)

  useEffect(() => {
    const fetchPendingCount = async () => {
      try {
        const res = await client.get("/dashboard/stats")
        if (res.data?.pending_requests) {
          setPendingCount(res.data.pending_requests)
        }
      } catch {
        // ignore
      }
    }

    fetchPendingCount()
    const interval = setInterval(fetchPendingCount, 30000)
    return () => clearInterval(interval)
  }, [])

  const navItems = [
    { to: "/dashboard", icon: LayoutDashboard, label: "Dashboard", end: true },
    { to: "/subscriptions", icon: CreditCard, label: "Subscriptions", badge: pendingCount },
    { to: "/tenants", icon: Users, label: "Tenants" },
    { to: "/packs", icon: Package, label: "Packs" },
    { to: "/settings", icon: Settings, label: "Settings" },
  ]

  const handleLogout = async () => {
    try {
      await client.post("/auth/logout")
    } catch {
      // ignore
    }
    clearSession()
    window.location.href = "/login"
  }

  return (
    <div className="flex h-screen overflow-hidden">
      <aside className="w-60 bg-sidebar text-sidebar-foreground border-r border-sidebar-border flex flex-col shrink-0 h-full overflow-hidden">
        <div className="h-16 flex items-center gap-2.5 px-4 border-b border-sidebar-border shrink-0">
          <div className="w-8 h-8 rounded-md bg-sidebar-primary/15 flex items-center justify-center shrink-0 ring-1 ring-sidebar-primary/30">
            <Activity className="w-4 h-4 text-sidebar-primary" />
          </div>
          <div className="leading-tight">
            <div className="text-base font-bold text-sidebar-foreground tracking-tight">
              <span className="text-sidebar-accent">Media</span><span className="text-sidebar-primary">Net</span>
            </div>
            <div className="text-[10px] uppercase tracking-[0.2em] text-sidebar-foreground/50 font-semibold">
              Admin Panel
            </div>
          </div>
        </div>

        <nav className="flex-1 py-3 px-2 space-y-0.5 overflow-y-auto">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                cn(
                  "flex items-center justify-between px-3 py-2 rounded-md text-sm transition-colors border-l-2",
                  isActive
                    ? "bg-sidebar-primary/15 text-sidebar-primary border-l-sidebar-primary"
                    : "text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-transparent"
                )
              }
            >
              <div className="flex items-center gap-3">
                <item.icon className="w-4 h-4 shrink-0" />
                <span className="font-medium">{item.label}</span>
              </div>
              {!!item.badge && item.badge > 0 && (
                <span className="px-1.5 py-0.5 text-[10px] font-bold rounded-full bg-amber-500/20 text-amber-400 border border-amber-500/30 animate-pulse">
                  {item.badge}
                </span>
              )}
            </NavLink>
          ))}
        </nav>

        <div className="py-2 px-2 space-y-0.5 border-t border-sidebar-border shrink-0">
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-2 border-transparent transition-colors"
          >
            <LogOut className="w-4 h-4 shrink-0" />
            <span className="font-medium">Logout</span>
          </button>
        </div>
      </aside>
      <main className="flex-1 overflow-auto bg-background">
        <Outlet />
      </main>
    </div>
  )
}
