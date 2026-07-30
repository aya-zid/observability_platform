import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import client from "@/api/client"
import { SectionCard } from "@/components/shared/Page"
import { Button } from "@/components/ui/button"

type NotificationChannel = {
  id: number
  type: string
  target: string
  is_verified: boolean
  created_at: string
}

async function getChannels(): Promise<NotificationChannel[]> {
  const { data } = await client.get<NotificationChannel[]>("/api/settings/notifications")
  return data
}

async function addChannel(type: string, target: string): Promise<NotificationChannel> {
  const { data } = await client.post<NotificationChannel>("/api/settings/notifications", { type, target })
  return data
}

async function deleteChannel(id: number): Promise<void> {
  await client.delete(`/api/settings/notifications/${id}`)
}

export default function NotificationsPage() {
  const queryClient = useQueryClient()
  const [showAdd, setShowAdd] = useState(false)
  const [email, setEmail] = useState("")

  const query = useQuery({ queryKey: ["notification-channels"], queryFn: getChannels })

  const addMutation = useMutation({
    mutationFn: () => addChannel("email", email),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["notification-channels"] })
      setShowAdd(false)
      setEmail("")
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteChannel(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["notification-channels"] }),
  })

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">Notifications</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Configure where to send alert notifications.
        </p>
      </div>

      <SectionCard title="Notification Channels" description="Configure where to send alert notifications.">
        {query.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading...</div>
        ) : !query.data?.length ? (
          <div className="text-sm text-muted-foreground">No notification channels configured.</div>
        ) : (
          <div className="grid gap-2">
            {query.data.map((ch) => (
              <div key={ch.id} className="flex items-center justify-between rounded-lg border border-border px-4 py-3">
                <div>
                  <span className="font-mono text-sm">{ch.target}</span>
                  <span className="ml-2 text-sm text-muted-foreground">({ch.type})</span>
                  {ch.is_verified && (
                    <span className="ml-2 text-sm text-green-500">verified</span>
                  )}
                </div>
                <Button variant="ghost" size="sm" className="text-destructive" onClick={() => deleteMutation.mutate(ch.id)}>
                  Delete
                </Button>
              </div>
            ))}
          </div>
        )}
        <div className="mt-3">
          <Button onClick={() => setShowAdd(true)}>Add Channel</Button>
        </div>
      </SectionCard>

      {showAdd && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-md">
            <h3 className="font-semibold text-lg mb-4">Add Notification Channel</h3>
            <div className="grid gap-4">
              <div>
                <label className="text-sm text-muted-foreground">Type</label>
                <select className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1" defaultValue="email">
                  <option value="email">Email</option>
                </select>
              </div>
              <div>
                <label className="text-sm text-muted-foreground">Email Address</label>
                <input
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
                  placeholder="alerts@example.com"
                />
              </div>
              <div className="flex gap-2 justify-end">
                <Button variant="outline" onClick={() => setShowAdd(false)}>Cancel</Button>
                <Button onClick={() => addMutation.mutate()} disabled={!email}>Add</Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
