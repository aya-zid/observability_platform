import { useState } from "react"
import { Sparkles, Loader2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import type { OverviewResponse } from "@/api/overview"

export function AIInsightPanel({ overview }: { overview: OverviewResponse }) {
  const [insight, setInsight] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const generateInsight = async () => {
    setLoading(true)
    setError(null)

    const summary = {
      hosts: `${overview.global.hosts_online}/${overview.global.hosts_total} online`,
      services: `${overview.global.services_healthy} healthy, ${overview.global.services_degraded} degraded, ${overview.global.services_down} down`,
      signals: overview.global.active_signals,
      findings: (overview.findings ?? []).map((f) => ({
        title: f.title,
        severity: f.severity,
        hostname: f.hostname,
        message: f.message,
      })),
      issues: (overview.issues ?? []).map((i) => ({
        hostname: i.hostname,
        service: i.service,
        issue: i.diagnosis,
        severity: i.severity,
      })),
    }

    try {
      const resp = await fetch(
        "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=AIzaSyDSKQKBcElRfa79RqjQ7tqPdJPzFc3RR1c",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            contents: [
              {
                parts: [
                  {
                    text: `You are a senior DevOps engineer. Analyze this observability overview data and provide a concise operational summary (max 3 paragraphs). Focus on: 1) what needs immediate attention, 2) any correlating patterns, 3) recommended next steps. Data: ${JSON.stringify(summary)}`,
                  },
                ],
              },
            ],
          }),
        }
      )

      if (!resp.ok) {
        throw new Error(`API error: ${resp.status}`)
      }

      const data = await resp.json()
      const text = data?.candidates?.[0]?.content?.parts?.[0]?.text
      if (text) {
        setInsight(text)
      } else {
        setError("No insight generated")
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to generate insight")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="rounded-lg border border-border bg-card/70 p-4 md:p-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h3 className="font-medium">AI Insight</h3>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Generate an operational summary from current telemetry data
          </p>
        </div>
        <Button onClick={generateInsight} disabled={loading} variant="outline" size="sm">
          {loading ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Sparkles className="h-4 w-4" />
          )}
          {loading ? "Generating…" : "Generate Insight"}
        </Button>
      </div>

      {insight ? (
        <div className="mt-4 rounded-md border border-border bg-background/50 p-3 text-sm leading-relaxed text-muted-foreground whitespace-pre-wrap">
          {insight}
        </div>
      ) : null}

      {error ? (
        <div className="mt-4 rounded-md border border-red-200 bg-red-50/50 p-3 text-sm text-red-600 dark:border-red-900/40 dark:bg-red-950/20 dark:text-red-400">
          {error}
        </div>
      ) : null}
    </div>
  )
}
