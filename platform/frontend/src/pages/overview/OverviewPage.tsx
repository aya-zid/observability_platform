import { useQuery } from "@tanstack/react-query"

import { getApiKey } from "@/api/tenant"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"

export default function OverviewPage() {
  const q = useQuery({
    queryKey: ["tenant", "api-key"],
    queryFn: getApiKey,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  return (
    <Page>
      <PageHeader
        title="Overview"
        description="Your tenant dashboard. We’ll place KPI cards and charts here."
      />

      <SectionCard
        title="Your API key (agents)"
        description="Use this API key to authenticate your agents (Basic auth) against backend ingest endpoints."
      >
        <div className="rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm break-all">
          {q.isLoading
            ? "Loading…"
            : q.isError
              ? "Unable to load API key."
              : q.data?.api_key ?? "No API key."}
        </div>
      </SectionCard>
    </Page>
  )
}
