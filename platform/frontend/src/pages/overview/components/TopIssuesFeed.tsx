type TopIssue = {
  machine_id: string
  hostname: string
  service: string
  severity: string
  issue: string
  value: string
  age: string
}
import { cn } from "@/lib/utils"

const sevStyles: Record<string, { dot: string; bg: string }> = {
  critical: {
    dot: "bg-red-500",
    bg: "bg-red-50/50 dark:bg-red-950/20",
  },
  warning: {
    dot: "bg-amber-500",
    bg: "bg-amber-50/50 dark:bg-amber-950/20",
  },
  info: {
    dot: "bg-blue-500",
    bg: "bg-blue-50/50 dark:bg-blue-950/20",
  },
}

export function TopIssuesFeed({ issues }: { issues: TopIssue[] | null | undefined }) {
  if (!issues || issues.length === 0) {
    return (
      <div className="rounded-lg border border-border bg-card/50 p-8 text-center text-sm text-muted-foreground">
        No active issues — everything looks healthy
      </div>
    )
  }

  return (
    <div className="space-y-1.5">
      {issues.map((issue, i) => {
        const style = sevStyles[issue.severity] ?? sevStyles.info
        return (
          <div
            key={`${issue.machine_id}-${issue.service}-${i}`}
            className={cn(
              "flex items-start gap-3 rounded-md border border-border px-3 py-2 text-sm",
              style.bg
            )}
          >
            <span className={cn("mt-1.5 inline-block h-2 w-2 shrink-0 rounded-full", style.dot)} />
            <div className="min-w-0 flex-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="font-medium">{issue.hostname}</span>
                <span className="text-sm text-muted-foreground">·</span>
                <span className="text-sm text-muted-foreground">{issue.service}</span>
                <span className="text-sm text-muted-foreground">·</span>
                <span className="text-sm font-medium">{issue.issue}</span>
              </div>
              <div className="mt-0.5 flex items-center gap-2 text-sm text-muted-foreground">
                <span>{issue.value}</span>
                <span>·</span>
                <span>{issue.age}</span>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
