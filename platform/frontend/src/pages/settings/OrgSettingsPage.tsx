import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState, useEffect } from "react"

import client from "@/api/client"
import { SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"

type OrgSettings = {
  company_name: string
  contact_email: string
  timezone: string
  created_at: string
}

async function getOrgSettings(): Promise<OrgSettings> {
  const { data } = await client.get<OrgSettings>("/api/settings/org")
  return data
}

async function updateOrgSettings(updates: { company_name?: string; timezone?: string }) {
  await client.patch("/api/settings/org", updates)
}

const timezones = [
  "UTC", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles",
  "Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Moscow",
  "Asia/Tokyo", "Asia/Shanghai", "Asia/Dubai", "Asia/Kolkata",
  "Australia/Sydney", "Pacific/Auckland",
]

export default function OrgSettingsPage() {
  const queryClient = useQueryClient()
  const query = useQuery({ queryKey: ["org-settings"], queryFn: getOrgSettings })
  const [companyName, setCompanyName] = useState("")
  const [timezone, setTimezone] = useState("UTC")

  useEffect(() => {
    if (query.data) {
      setCompanyName(query.data.company_name)
      setTimezone(query.data.timezone)
    }
  }, [query.data])

  const mutation = useMutation({
    mutationFn: () => updateOrgSettings({ company_name: companyName, timezone }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["org-settings"] }),
  })

  const hasChanges = query.data && (companyName !== query.data.company_name || timezone !== query.data.timezone)

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">General</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Manage your organization settings and preferences.
        </p>
      </div>

      <SectionCard title="Organization" description="Manage your organization details.">
        {query.isLoading ? (
          <div className="text-sm text-muted-foreground py-8 text-center">Loading...</div>
        ) : (
          <div className="grid gap-6 md:grid-cols-2">
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                Company Name
              </label>
              <input
                type="text"
                value={companyName}
                onChange={(e) => setCompanyName(e.target.value)}
                className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                placeholder="Your company"
              />
              <p className="text-xs text-muted-foreground">Used for branding and notifications.</p>
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                Contact Email
              </label>
              <input
                type="text"
                value={query.data?.contact_email ?? ""}
                disabled
                className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm opacity-60 ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              />
              <p className="text-xs text-muted-foreground">Primary contact for billing and alerts.</p>
            </div>
            <div className="space-y-1.5">
              <label className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                Timezone
              </label>
              <select
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {timezones.map((tz) => (
                  <option key={tz} value={tz}>{tz}</option>
                ))}
              </select>
              <p className="text-xs text-muted-foreground">Used for time displays across the platform.</p>
            </div>
          </div>
        )}
      </SectionCard>

      {!query.isLoading && (
        <div className="flex items-center justify-end gap-3">
          <Button
            variant="outline"
            disabled={!hasChanges}
            onClick={() => {
              setCompanyName(query.data?.company_name ?? "")
              setTimezone(query.data?.timezone ?? "UTC")
            }}
          >
            Cancel
          </Button>
          <Button
            onClick={() => mutation.mutate()}
            disabled={mutation.isPending || !hasChanges}
            className="min-w-[120px] shadow-accent-glow"
          >
            {mutation.isPending ? "Saving..." : "Save changes"}
          </Button>
        </div>
      )}

      <section className="rounded-xl border border-destructive/20 bg-destructive/5 shadow-card p-6">
        <div className="mb-4">
          <div className="font-semibold text-destructive">Danger Zone</div>
          <div className="mt-1 text-sm text-muted-foreground">
            Irreversible actions that affect your entire organization.
          </div>
        </div>
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium text-foreground">Delete organization</div>
            <p className="text-xs text-muted-foreground mt-0.5">
              Permanently delete your organization and all associated data.
            </p>
          </div>
          <Button variant="outline" disabled className="text-destructive border-destructive/50 shrink-0">
            Delete
          </Button>
        </div>
        <p className="text-xs text-muted-foreground mt-2">Coming soon.</p>
      </section>
    </div>
  )
}
