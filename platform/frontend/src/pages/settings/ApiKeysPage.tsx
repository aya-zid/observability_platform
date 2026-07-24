import { useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { getApiKey, rotateApiKey } from "@/api/tenant"
import { getMe } from "@/api/tenant"
import { SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"
import { useAuthStore } from "@/store/authStore"

import client from "@/api/client"

function getPlatformHost(): string {
  try {
    const url = new URL(client.defaults.baseURL ?? "")
    return `${url.hostname}:${url.port || "8080"}`
  } catch {
    return `${window.location.hostname}:8080`
  }
}

export default function ApiKeysPage() {
  const role = useAuthStore((s) => s.role)
  const isAdmin = role === "admin"
  const [rotateError, setRotateError] = useState<string | null>(null)
  const [showConfirm, setShowConfirm] = useState(false)
  const [newKey, setNewKey] = useState<string | null>(null)
  const queryClient = useQueryClient()

  const q = useQuery({
    queryKey: ["tenant", "api-key"],
    queryFn: getApiKey,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  const meQuery = useQuery({
    queryKey: ["me"],
    queryFn: getMe,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  async function onRotate() {
    setRotateError(null)
    try {
      const data = await rotateApiKey()
      setNewKey(data.api_key)
      queryClient.setQueryData(["tenant", "api-key"], data)
      setShowConfirm(false)
    } catch {
      setRotateError("Failed to rotate API key.")
    }
  }

  function maskKey(key: string): string {
    if (key.length <= 12) return key
    return key.substring(0, 12) + "••••••••"
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">API Keys</h1>
        <p className="text-sm text-muted-foreground mt-1">Manage keys used by agents and services to send telemetry.</p>
      </div>

      <SectionCard
        title="Current API key"
        description="This key is used by agents as Basic auth to send telemetry to /ingest/* endpoints."
      >
        {isAdmin && (
          <div className="mb-3">
            <Button onClick={() => setShowConfirm(true)} disabled={q.isLoading}>
              Rotate API key
            </Button>
          </div>
        )}

        <div className="rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm break-all flex items-center justify-between">
          <span>
            {q.isLoading
              ? "Loading…"
              : q.isError
                ? "Unable to load API key."
                : q.data?.api_key
                  ? maskKey(q.data.api_key)
                  : "No API key."}
          </span>
          {q.data?.api_key && (
            <button
              onClick={() => navigator.clipboard.writeText(q.data!.api_key)}
              className="ml-2 text-xs text-primary hover:underline shrink-0"
            >
              Copy
            </button>
          )}
        </div>

        {q.data?.api_key && (
          <div className="mt-2 text-xs text-muted-foreground">
            Last used: {meQuery.data?.company_name ? "tracked" : "N/A"}
          </div>
        )}

        {rotateError ? (
          <div className="mt-2 text-xs text-destructive">{rotateError}</div>
        ) : null}
      </SectionCard>

      <SectionCard
        title="Agent Installation"
        description="Run these commands on any host you want to monitor. Requires root/sudo access."
      >
        <div className="space-y-4">
          <div>
            <p className="text-sm font-medium mb-1.5">Install agent</p>
            <div className="rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm overflow-x-auto">
              <div className="flex items-center justify-between whitespace-nowrap">
                <span>
                  {q.isLoading
                    ? "Loading…"
                    : q.data?.api_key
                      ? `curl -sSL http://${getPlatformHost()}/install.sh | sudo API_KEY=${q.data.api_key} bash`
                      : "Load an API key first."}
                </span>
                {q.data?.api_key && (
                  <button
                    onClick={() =>
                      navigator.clipboard.writeText(
                        `curl -sSL http://${getPlatformHost()}/install.sh | sudo API_KEY=${q.data!.api_key} bash`
                      )
                    }
                    className="ml-2 text-xs text-primary hover:underline shrink-0"
                  >
                    Copy
                  </button>
                )}
              </div>
            </div>
          </div>

          <div>
            <p className="text-sm font-medium mb-1.5">Uninstall agent</p>
            <div className="rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm overflow-x-auto">
              <div className="flex items-center justify-between whitespace-nowrap">
                <span>
                  {`curl -sSL http://${getPlatformHost()}/uninstall.sh | sudo bash`}
                </span>
                <button
                  onClick={() =>
                    navigator.clipboard.writeText(
                      `curl -sSL http://${getPlatformHost()}/uninstall.sh | sudo bash`
                    )
                  }
                  className="ml-2 text-xs text-primary hover:underline shrink-0"
                >
                  Copy
                </button>
              </div>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              Removes the agent from the host. Does not affect stored telemetry data.
            </p>
          </div>
        </div>
      </SectionCard>

      {isAdmin && showConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-md">
            <h3 className="font-semibold text-lg">Rotate API Key?</h3>
            <p className="text-sm text-muted-foreground mt-2">
              This will immediately invalidate the old key. All agents using the current key will stop sending data until reconfigured.
            </p>
            <div className="mt-4 flex gap-2 justify-end">
              <Button variant="outline" onClick={() => setShowConfirm(false)}>Cancel</Button>
              <Button onClick={onRotate}>Confirm Rotation</Button>
            </div>
          </div>
        </div>
      )}

      {newKey && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-md">
            <h3 className="font-semibold text-lg text-amber-500">Copy Your New Key</h3>
            <p className="text-sm text-muted-foreground mt-2">
              You won't be able to see this key again. Copy it now and update your agents.
            </p>
            <div className="mt-3 rounded-md border border-border bg-background/70 px-3 py-2 font-mono text-sm break-all">
              {newKey}
            </div>
            <div className="mt-4 flex gap-2 justify-end">
              <Button onClick={() => { navigator.clipboard.writeText(newKey); setNewKey(null) }}>
                Copy & Close
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
