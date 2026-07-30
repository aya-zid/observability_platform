import { useState } from "react"
import { NavLink, useLocation } from "react-router-dom"
import { useQuery } from "@tanstack/react-query"
import {
  LayoutDashboard, Bell, Server, Compass, Box, Settings, User,
  ChevronLeft, ChevronRight, Sun, Moon, Activity, Rocket, Languages, BookOpen,
} from "lucide-react"

import { cn } from "@/lib/utils"
import { useThemeStore } from "@/store/themeStore"
import { useTranslation } from "@/i18n/LanguageContext"
import { getAlertEvents } from "@/api/alerts"

const navItems = [
  { to: "/overview", icon: LayoutDashboard, label: "Overview", end: true },
  { to: "/get-started", icon: Rocket, label: "Get Started" },
  { to: "/docs", icon: BookOpen, label: "Documentation" },
  { to: "/alerts", icon: Bell, label: "Alerts" },
  { to: "/hosts", icon: Server, label: "Hosts" },
  { to: "/services", icon: Compass, label: "Services" },
  { to: "/containers", icon: Box, label: "Containers" },
]

export default function Sidebar() {
  const [collapsed, setCollapsed] = useState(false)
  const location = useLocation()
  const theme = useThemeStore((s) => s.theme)
  const toggleTheme = useThemeStore((s) => s.toggle)
  const { lang, setLang } = useTranslation()

  const firingCount = useQuery({
    queryKey: ["alert-events", "sidebar"],
    queryFn: async () => {
      const events = await getAlertEvents(50)
      return events.filter(e => e.status === "firing" && !e.acknowledged_by).length
    },
    refetchInterval: 30_000,
  })

  function isActive(to: string, end?: boolean) {
    if (end) return location.pathname === to
    return location.pathname.startsWith(to)
  }

  return (
    <aside
      className={cn(
        "bg-sidebar text-sidebar-foreground border-r border-sidebar-border flex flex-col transition-all duration-200 shrink-0 h-full overflow-hidden",
        collapsed ? "w-16" : "w-60"
      )}
    >
      {/* Brand */}
      <div className={cn(
        "h-16 flex items-center border-b border-sidebar-border gap-2.5 shrink-0",
        collapsed ? "justify-center px-0" : "px-4"
      )}>
        <div className="w-8 h-8 rounded-md bg-sidebar-primary/15 flex items-center justify-center shrink-0 ring-1 ring-sidebar-primary/30">
          <Activity className="w-4 h-4 text-sidebar-primary" />
        </div>
        {!collapsed && (
          <div className="leading-tight">
            <div className="text-base font-bold text-sidebar-foreground tracking-tight">
               <span className="text-sidebar-accent">Media</span><span className="text-sidebar-primary">Net</span>
            </div>
            <div className="text-[10px] uppercase tracking-[0.2em] text-sidebar-foreground/50 font-semibold">
              Observability
            </div>
          </div>
        )}
      </div>

      {/* Nav */}
      <nav className="flex-1 py-3 px-2 space-y-0.5 overflow-y-auto">
        {navItems.map((item) => {
          const active = isActive(item.to, item.end)
          return (
            <NavLink
              key={item.to}
              to={item.to}
              title={collapsed ? item.label : undefined}
              className={cn(
                "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors border-l-2",
                active
                  ? "bg-sidebar-primary/15 text-sidebar-primary border-l-sidebar-primary"
                  : "text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-transparent"
              )}
            >
              {item.label === "Alerts" && firingCount.data && firingCount.data > 0 ? (
                <div className="relative">
                  <item.icon className={cn("w-4 h-4 shrink-0", active && "text-sidebar-primary")} />
                  <span className="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-red-500 text-[9px] font-bold text-white flex items-center justify-center ring-2 ring-sidebar">
                    {firingCount.data > 9 ? "9+" : firingCount.data}
                  </span>
                </div>
              ) : (
                <item.icon className={cn("w-4 h-4 shrink-0", active && "text-sidebar-primary")} />
              )}
              {!collapsed && <span className="font-medium">{item.label}</span>}
            </NavLink>
          )
        })}

      </nav>

      {/* Bottom */}
      <div className="py-2 px-2 space-y-0.5 border-t border-sidebar-border shrink-0">
        <NavLink
          to="/settings/general"
          title={collapsed ? "Settings" : undefined}
          className={cn(
            "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors border-l-2",
            location.pathname.startsWith("/settings")
              ? "bg-sidebar-primary/15 text-sidebar-primary border-l-sidebar-primary"
              : "text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-transparent"
          )}
        >
          <Settings className="w-4 h-4 shrink-0" />
          {!collapsed && <span className="font-medium">Settings</span>}
        </NavLink>
        <NavLink
          to="/profile"
          title={collapsed ? "Profile" : undefined}
          className={cn(
            "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors border-l-2",
            location.pathname === "/profile"
              ? "bg-sidebar-primary/15 text-sidebar-primary border-l-sidebar-primary"
              : "text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-transparent"
          )}
        >
          <User className="w-4 h-4 shrink-0" />
          {!collapsed && <span className="font-medium">Profile</span>}
        </NavLink>
        <button
          onClick={toggleTheme}
          title={collapsed ? (theme === "dark" ? "Light mode" : "Dark mode") : undefined}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-2 border-transparent transition-colors"
        >
          {theme === "dark" ? <Sun className="w-4 h-4 shrink-0" /> : <Moon className="w-4 h-4 shrink-0" />}
          {!collapsed && (
            <span className="font-medium">{theme === "dark" ? "Light mode" : "Dark mode"}</span>
          )}
        </button>
        <button
          onClick={() => setLang(lang === "en" ? "fr" : "en")}
          title={collapsed ? (lang === "en" ? "Français" : "English") : undefined}
          className="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm text-sidebar-foreground/80 hover:text-sidebar-foreground hover:bg-sidebar-accent/10 border-l-2 border-transparent transition-colors"
        >
          <Languages className="w-4 h-4 shrink-0" />
          {!collapsed && (
            <span className="font-medium">{lang === "en" ? "Français" : "English"}</span>
          )}
        </button>
      </div>

      {/* Collapse toggle */}
      <button
        onClick={() => setCollapsed(!collapsed)}
        className="h-9 flex items-center justify-center border-t border-sidebar-border text-sidebar-foreground/70 hover:text-sidebar-foreground transition-colors shrink-0"
      >
        {collapsed ? <ChevronRight className="w-4 h-4" /> : <ChevronLeft className="w-4 h-4" />}
      </button>
    </aside>
  )
}
