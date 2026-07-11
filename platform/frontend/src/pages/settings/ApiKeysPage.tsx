import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { getApiKey, rotateApiKey } from "@/api/tenant"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"

export default function ApiKeysPage() {
  const [rotateError, setRotateError] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const q = useQuery({
    queryKey: ["tenant", "api-key"],
    queryFn: getApiKey,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  async function onRotate() {
    setRotateError(null)
    try {
      const data = await rotateApiKey()
      queryClient.setQueryData(["tenant", "api-key"], data)
    } catch {
      setRotateError("Failed to rotate API key.")
    }
  }

  return (
    <Page>
      <PageHeader
        title="API Keys"
        description="Manage keys used by agents and services."
      />

      <SectionCard
        title="Current API key"
        description="This is the key agents use as Basic auth to send telemetry to /ingest/*."
      >
        <div className="mb-3">
          <Button onClick={onRotate} disabled={q.isLoading}>
            Rotate API key
          </Button>
        </div>
        <div className="rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm break-all">
          {q.isLoading
            ? "Loading…"
            : q.isError
              ? "Unable to load API key."
              : q.data?.api_key ?? "No API key."}
        </div>
        {rotateError ? (
          <div className="mt-2 text-xs text-destructive">{rotateError}</div>
        ) : null}
      </SectionCard>
    </Page>
  )
}
