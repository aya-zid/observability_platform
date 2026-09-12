import * as React from "react"
import { cn } from "@/lib/utils"

export interface TabItem {
  id: string
  label: string
  count?: number
  badge?: React.ReactNode
  icon?: React.ComponentType<{ className?: string }>
}

interface TabsProps {
  tabs: TabItem[]
  activeTab: string
  onChange: (tabId: string) => void
  className?: string
}

export function Tabs({ tabs, activeTab, onChange, className }: TabsProps) {
  return (
    <div className={cn("flex items-center border-b border-border space-x-1", className)}>
      {tabs.map((tab) => {
        const isActive = activeTab === tab.id
        const Icon = tab.icon

        return (
          <button
            key={tab.id}
            onClick={() => onChange(tab.id)}
            className={cn(
              "relative flex items-center gap-2 px-4 py-2.5 text-sm font-medium transition-colors border-b-2 -mb-px outline-none",
              isActive
                ? "border-primary text-primary font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground hover:border-border"
            )}
          >
            {Icon && <Icon className="w-4 h-4" />}
            <span>{tab.label}</span>

            {typeof tab.count === "number" && (
              <span
                className={cn(
                  "ml-1.5 px-2 py-0.5 text-xs font-semibold rounded-full transition-colors",
                  isActive
                    ? "bg-primary/20 text-primary"
                    : "bg-muted text-muted-foreground"
                )}
              >
                {tab.count}
              </span>
            )}

            {tab.badge}
          </button>
        )
      })}
    </div>
  )
}
