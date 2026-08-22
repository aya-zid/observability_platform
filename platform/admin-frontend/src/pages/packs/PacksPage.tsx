import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import client from "@/api/client"

type Pack = {
  id: number
  name: string
  description: string
  price_per_month: number
  max_hosts: number
  max_services_per_host: number
  log_retention_days: number
  metrics_retention_days: number
  max_log_bytes_per_day: number
  ai_reports_enabled: boolean
  duration_months: number
  is_active: boolean
  active_subscriptions: number
}

const emptyForm = {
  name: "",
  description: "",
  price_per_month: 0,
  max_hosts: 1,
  max_services_per_host: 20,
  log_retention_days: 7,
  metrics_retention_days: 30,
  max_log_bytes_per_day: 1073741824,
  ai_reports_enabled: false,
  duration_months: 1,
  is_active: true,
}

export default function PacksPage() {
  const queryClient = useQueryClient()
  const [editing, setEditing] = useState<Pack | null>(null)
  const [form, setForm] = useState(emptyForm)

  const { data: packs, isLoading } = useQuery<Pack[]>({
    queryKey: ["admin", "packs"],
    queryFn: async () => {
      const res = await client.get("/packs")
      return res.data
    },
  })

  const createMutation = useMutation({
    mutationFn: async (data: typeof form) => {
      await client.post("/packs", data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "packs"] })
      setForm(emptyForm)
    },
  })

  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: number; data: typeof form }) => {
      await client.patch(`/packs/${id}`, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "packs"] })
      setEditing(null)
      setForm(emptyForm)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await client.delete(`/packs/${id}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "packs"] })
    },
  })

  const startEdit = (pack: Pack) => {
    setEditing(pack)
    setForm({
      name: pack.name,
      description: pack.description,
      price_per_month: pack.price_per_month,
      max_hosts: pack.max_hosts,
      max_services_per_host: pack.max_services_per_host,
      log_retention_days: pack.log_retention_days,
      metrics_retention_days: pack.metrics_retention_days,
      max_log_bytes_per_day: pack.max_log_bytes_per_day,
      ai_reports_enabled: pack.ai_reports_enabled,
      duration_months: pack.duration_months,
      is_active: pack.is_active,
    })
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (editing) {
      updateMutation.mutate({ id: editing.id, data: form })
    } else {
      createMutation.mutate(form)
    }
  }

  return (
    <div className="p-6 md:p-8 space-y-6">
      <h1 className="text-3xl font-bold tracking-tight">Packs</h1>

      <form onSubmit={handleSubmit} className="rounded-lg border border-border bg-card p-5 shadow-card space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">{editing ? "Edit Pack" : "Create Pack"}</h2>
          {editing && (
            <button
              type="button"
              onClick={() => {
                setEditing(null)
                setForm(emptyForm)
              }}
              className="text-sm text-muted-foreground hover:text-foreground"
            >
              Cancel
            </button>
          )}
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="text-sm font-medium block mb-1">Name</label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Price ($/mo)</label>
            <input
              type="number"
              step="0.01"
              value={form.price_per_month}
              onChange={(e) => setForm({ ...form, price_per_month: parseFloat(e.target.value) || 0 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Max Hosts</label>
            <input
              type="number"
              value={form.max_hosts}
              onChange={(e) => setForm({ ...form, max_hosts: parseInt(e.target.value, 10) || 1 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Duration (months)</label>
            <input
              type="number"
              value={form.duration_months}
              onChange={(e) => setForm({ ...form, duration_months: parseInt(e.target.value, 10) || 1 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Log Retention (days)</label>
            <input
              type="number"
              value={form.log_retention_days}
              onChange={(e) => setForm({ ...form, log_retention_days: parseInt(e.target.value, 10) || 7 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Metrics Retention (days)</label>
            <input
              type="number"
              value={form.metrics_retention_days}
              onChange={(e) => setForm({ ...form, metrics_retention_days: parseInt(e.target.value, 10) || 30 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div>
            <label className="text-sm font-medium block mb-1">Max Log Bytes/Day</label>
            <input
              type="number"
              value={form.max_log_bytes_per_day}
              onChange={(e) => setForm({ ...form, max_log_bytes_per_day: parseInt(e.target.value, 10) || 0 })}
              className="w-full h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
              required
            />
          </div>
          <div className="flex items-center gap-2">
            <input
              type="checkbox"
              id="ai_reports"
              checked={form.ai_reports_enabled}
              onChange={(e) => setForm({ ...form, ai_reports_enabled: e.target.checked })}
              className="rounded border-border"
            />
            <label htmlFor="ai_reports" className="text-sm font-medium">AI Reports Enabled</label>
          </div>
        </div>
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={createMutation.isPending || updateMutation.isPending}
            className="h-9 px-4 bg-primary text-primary-foreground rounded-md text-sm font-medium hover:bg-primary/80 disabled:opacity-60 transition-colors"
          >
            {editing ? "Update" : "Create"}
          </button>
        </div>
      </form>

      {isLoading ? (
        <div className="text-sm text-muted-foreground">Loading packs…</div>
      ) : (
        <div className="rounded-lg border border-border bg-card shadow-card overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-muted/50">
              <tr>
                <th className="text-left p-3 font-medium">Name</th>
                <th className="text-left p-3 font-medium">Price</th>
                <th className="text-left p-3 font-medium">Max Hosts</th>
                <th className="text-left p-3 font-medium">Retention</th>
                <th className="text-left p-3 font-medium">AI Reports</th>
                <th className="text-left p-3 font-medium">Active</th>
                <th className="text-right p-3 font-medium">Actions</th>
              </tr>
            </thead>
            <tbody>
              {packs?.map((pack) => (
                <tr key={pack.id} className="border-t border-border">
                  <td className="p-3 font-medium">{pack.name}</td>
                  <td className="p-3">${pack.price_per_month}</td>
                  <td className="p-3">{pack.max_hosts}</td>
                  <td className="p-3">{pack.log_retention_days}d / {pack.metrics_retention_days}d</td>
                  <td className="p-3">{pack.ai_reports_enabled ? "Yes" : "No"}</td>
                  <td className="p-3">{pack.is_active ? "Yes" : "No"}</td>
                  <td className="p-3 text-right">
                    <button
                      onClick={() => startEdit(pack)}
                      className="text-primary hover:underline text-xs mr-3"
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => {
                        if (confirm("Delete this pack?")) {
                          deleteMutation.mutate(pack.id)
                        }
                      }}
                      className="text-destructive hover:underline text-xs"
                      disabled={pack.active_subscriptions > 0}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
              {packs?.length === 0 && (
                <tr>
                  <td colSpan={7} className="p-8 text-center text-muted-foreground">
                    No packs created yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
