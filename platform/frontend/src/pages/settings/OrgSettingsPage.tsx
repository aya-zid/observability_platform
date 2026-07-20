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

const timezones = ["UTC", "America/New_York", "America/Chicago", "America/Denver", "America/Los_Angeles", "Europe/London", "Europe/Paris", "Europe/Berlin", "Europe/Moscow", "Asia/Tokyo", "Asia/Shanghai", "Asia/Dubai", "Asia/Kolkata", "Australia/Sydney", "Pacific/Auckland"]

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

  return (
    <div className="grid gap-6">
      <SectionCard title="Organization Settings" description="Manage your organization details.">
        {query.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading...</div>
        ) : (
          <div className="grid gap-4 max-w-md">
            <div>
              <label className="text-sm text-muted-foreground">Company Name</label>
              <input
                type="text"
                value={companyName}
                onChange={(e) => setCompanyName(e.target.value)}
                className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground">Contact Email</label>
              <input
                type="text"
                value={query.data?.contact_email ?? ""}
                disabled
                className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1 opacity-60"
              />
            </div>
            <div>
              <label className="text-sm text-muted-foreground">Timezone</label>
              <select
                value={timezone}
                onChange={(e) => setTimezone(e.target.value)}
                className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
              >
                {timezones.map((tz) => (
                  <option key={tz} value={tz}>{tz}</option>
                ))}
              </select>
            </div>
            <div className="pt-2">
              <Button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
                {mutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
          </div>
        )}
      </SectionCard>

      <SectionCard title="Danger Zone" description="Irreversible actions.">
        <p className="text-sm text-muted-foreground mb-3">
          Deleting your organization will remove all data permanently.
        </p>
        <Button variant="outline" disabled className="text-destructive border-destructive/50">
          Delete Organization
        </Button>
        <p className="text-xs text-muted-foreground mt-2">Coming soon.</p>
      </SectionCard>
    </div>
  )
}
