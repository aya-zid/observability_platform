import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import {
  Package, Plus, Edit, Trash2, Check, Sparkles, LayoutGrid, List,
  Server, ShieldCheck, Clock, Users, Zap
} from "lucide-react"
import client from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/ui/empty-state"

type Pack = {
  id: number
  name: string
  description: string
  price_per_month: number
  max_hosts: number
  max_services_per_host: number
  log_retention_days: number
  metrics_retention_days: number
  ai_reports_enabled: boolean
  duration_months: number
  is_active: boolean
  is_trial: boolean
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
  ai_reports_enabled: false,
  duration_months: 1,
  is_active: true,
  is_trial: false,
}

export default function PacksPage() {
  const queryClient = useQueryClient()
  const [viewMode, setViewMode] = useState<"grid" | "table">("grid")
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<Pack | null>(null)
  const [form, setForm] = useState(emptyForm)
  const [deletePackItem, setDeletePackItem] = useState<Pack | null>(null)

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
      closeModal()
    },
    onError: (err: unknown) => {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to create pack")
    },
  })

  const updateMutation = useMutation({
    mutationFn: async ({ id, data }: { id: number; data: typeof form }) => {
      await client.patch(`/packs/${id}`, data)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "packs"] })
      closeModal()
    },
    onError: (err: unknown) => {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to update pack")
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      await client.delete(`/packs/${id}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "packs"] })
      setDeletePackItem(null)
    },
    onError: (err: unknown) => {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to delete pack")
    },
  })

  const openCreate = () => {
    setEditing(null)
    setForm(emptyForm)
    setShowModal(true)
  }

  const openEdit = (pack: Pack) => {
    setEditing(pack)
    setForm({
      name: pack.name,
      description: pack.description || "",
      price_per_month: pack.price_per_month,
      max_hosts: pack.max_hosts,
      max_services_per_host: pack.max_services_per_host,
      log_retention_days: pack.log_retention_days,
      metrics_retention_days: pack.metrics_retention_days,
      ai_reports_enabled: pack.ai_reports_enabled,
      duration_months: pack.duration_months,
      is_active: pack.is_active,
      is_trial: pack.is_trial,
    })
    setShowModal(true)
  }

  const closeModal = () => {
    setShowModal(false)
    setEditing(null)
    setForm(emptyForm)
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
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-foreground flex items-center gap-2">
            <Package className="w-7 h-7 text-primary" />
            Pack & Tier Catalog
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Configure subscription tiers, host limits, retention windows, and AI report privileges.
          </p>
        </div>

        <div className="flex items-center gap-3 self-start sm:self-auto">
          <div className="flex items-center rounded-lg border border-border bg-card p-1">
            <button
              onClick={() => setViewMode("grid")}
              className={`p-1.5 rounded-md transition-colors ${
                viewMode === "grid" ? "bg-primary/20 text-primary" : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <LayoutGrid className="w-4 h-4" />
            </button>
            <button
              onClick={() => setViewMode("table")}
              className={`p-1.5 rounded-md transition-colors ${
                viewMode === "table" ? "bg-primary/20 text-primary" : "text-muted-foreground hover:text-foreground"
              }`}
            >
              <List className="w-4 h-4" />
            </button>
          </div>

          <Button onClick={openCreate} className="gap-2 bg-primary text-primary-foreground font-medium">
            <Plus className="w-4 h-4" />
            Create New Pack
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-72 rounded-xl" />
        </div>
      ) : !packs || packs.length === 0 ? (
        <EmptyState
          icon={Package}
          title="No subscription packs created"
          description="Create your first subscription pack to allow tenants to choose an observability plan."
          action={<Button onClick={openCreate}>Create First Pack</Button>}
        />
      ) : viewMode === "grid" ? (
        /* Grid View */
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {packs.map((pack) => (
            <div
              key={pack.id}
              className={`relative rounded-xl border p-6 flex flex-col justify-between transition-all duration-200 ${
                pack.is_trial
                  ? "bg-card border-purple-500/30"
                  : pack.price_per_month > 100
                  ? "bg-gradient-to-b from-card to-primary/5 border-primary/40 shadow-lg shadow-primary/5 ring-1 ring-primary/20"
                  : "bg-card border-border"
              }`}
            >
              <div className="space-y-4">
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="text-xl font-bold text-foreground tracking-tight">{pack.name}</h3>
                    {pack.description && (
                      <p className="text-xs text-muted-foreground mt-1 line-clamp-2">{pack.description}</p>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-1">
                    {pack.is_trial ? (
                      <Badge variant="purple">Trial Pack</Badge>
                    ) : pack.is_active ? (
                      <Badge variant="success">Active</Badge>
                    ) : (
                      <Badge variant="outline">Disabled</Badge>
                    )}
                  </div>
                </div>

                {/* Price Display */}
                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-extrabold text-foreground">${pack.price_per_month}</span>
                  <span className="text-xs text-muted-foreground">/ month</span>
                  <span className="text-xs font-semibold text-primary/80 ml-auto bg-primary/10 px-2 py-0.5 rounded-full">
                    {pack.duration_months} month billing
                  </span>
                </div>

                {/* Feature List */}
                <div className="space-y-2 pt-2 border-t border-border/60 text-xs">
                  <div className="flex items-center gap-2 text-foreground">
                    <Server className="w-4 h-4 text-primary shrink-0" />
                    <span>Up to <strong>{pack.max_hosts} Hosts</strong> monitored</span>
                  </div>
                  <div className="flex items-center gap-2 text-foreground">
                    <Clock className="w-4 h-4 text-sky-400 shrink-0" />
                    <span>Logs: <strong>{pack.log_retention_days} days</strong> retention</span>
                  </div>
                  <div className="flex items-center gap-2 text-foreground">
                    <Zap className="w-4 h-4 text-emerald-400 shrink-0" />
                    <span>Metrics: <strong>{pack.metrics_retention_days} days</strong> retention</span>
                  </div>
                  <div className="flex items-center gap-2 text-foreground">
                    <Sparkles className="w-4 h-4 text-amber-400 shrink-0" />
                    <span>AI Reports: <strong>{pack.ai_reports_enabled ? "Included" : "Disabled"}</strong></span>
                  </div>
                  <div className="flex items-center gap-2 text-muted-foreground pt-1">
                    <Users className="w-4 h-4 shrink-0" />
                    <span>{pack.active_subscriptions} active tenant subscriber(s)</span>
                  </div>
                </div>
              </div>

              {/* Action Buttons */}
              <div className="mt-6 pt-4 border-t border-border/60 flex items-center justify-between gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  className="w-full gap-1.5"
                  onClick={() => openEdit(pack)}
                >
                  <Edit className="w-3.5 h-3.5" /> Edit Pack
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive hover:bg-destructive/10 hover:text-destructive shrink-0"
                  disabled={pack.active_subscriptions > 0}
                  onClick={() => setDeletePackItem(pack)}
                >
                  <Trash2 className="w-4 h-4" />
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        /* Table View */
        <div className="overflow-x-auto rounded-xl border border-border bg-card shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="bg-muted/50 text-xs font-semibold text-muted-foreground uppercase border-b border-border">
              <tr>
                <th className="px-4 py-3">Pack Name</th>
                <th className="px-4 py-3">Price</th>
                <th className="px-4 py-3">Host Limit</th>
                <th className="px-4 py-3">Retention (Logs / Metrics)</th>
                <th className="px-4 py-3">AI Reports</th>
                <th className="px-4 py-3">Subscribers</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {packs.map((pack) => (
                <tr key={pack.id} className="hover:bg-accent/30 transition-colors">
                  <td className="px-4 py-3 font-semibold text-foreground">
                    <div>{pack.name}</div>
                    <div className="text-xs text-muted-foreground">{pack.description}</div>
                  </td>
                  <td className="px-4 py-3 font-mono font-bold text-primary">${pack.price_per_month}/mo</td>
                  <td className="px-4 py-3 font-mono">{pack.max_hosts} hosts</td>
                  <td className="px-4 py-3 text-xs">{pack.log_retention_days}d / {pack.metrics_retention_days}d</td>
                  <td className="px-4 py-3">
                    {pack.ai_reports_enabled ? (
                      <span className="text-emerald-400 font-semibold text-xs flex items-center gap-1">
                        <Check className="w-3.5 h-3.5" /> Yes
                      </span>
                    ) : (
                      <span className="text-muted-foreground text-xs">No</span>
                    )}
                  </td>
                  <td className="px-4 py-3 font-mono">{pack.active_subscriptions}</td>
                  <td className="px-4 py-3">
                    {pack.is_trial ? (
                      <Badge variant="purple">Trial</Badge>
                    ) : pack.is_active ? (
                      <Badge variant="success">Active</Badge>
                    ) : (
                      <Badge variant="outline">Disabled</Badge>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right space-x-1">
                    <Button size="sm" variant="ghost" onClick={() => openEdit(pack)}>
                      <Edit className="w-3.5 h-3.5 mr-1" /> Edit
                    </Button>
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive hover:bg-destructive/10"
                      disabled={pack.active_subscriptions > 0}
                      onClick={() => setDeletePackItem(pack)}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Modal: Create or Edit Pack */}
      <Modal
        isOpen={showModal}
        onClose={closeModal}
        title={editing ? `Edit Pack: ${editing.name}` : "Create New Subscription Pack"}
        description="Define specifications, pricing, and resource allocations."
        maxWidth="lg"
      >
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Pack Name</label>
              <input
                type="text"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. Enterprise Tier"
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Price ($ / Month)</label>
              <input
                type="number"
                step="0.01"
                value={form.price_per_month}
                onChange={(e) => setForm({ ...form, price_per_month: parseFloat(e.target.value) || 0 })}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>

            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-foreground mb-1">Description</label>
              <input
                type="text"
                value={form.description}
                onChange={(e) => setForm({ ...form, description: e.target.value })}
                placeholder="Short summary of target audience or benefits"
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Max Monitored Hosts</label>
              <input
                type="number"
                value={form.max_hosts}
                onChange={(e) => setForm({ ...form, max_hosts: parseInt(e.target.value, 10) || 1 })}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Duration (Months)</label>
              <input
                type="number"
                value={form.duration_months}
                onChange={(e) => setForm({ ...form, duration_months: parseInt(e.target.value, 10) || 1 })}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Log Retention (Days)</label>
              <input
                type="number"
                value={form.log_retention_days}
                onChange={(e) => setForm({ ...form, log_retention_days: parseInt(e.target.value, 10) || 7 })}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground mb-1">Metrics Retention (Days)</label>
              <input
                type="number"
                value={form.metrics_retention_days}
                onChange={(e) => setForm({ ...form, metrics_retention_days: parseInt(e.target.value, 10) || 30 })}
                className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                required
              />
            </div>
          </div>

          <div className="pt-2 border-t border-border flex flex-wrap gap-4 text-xs font-medium text-foreground">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.ai_reports_enabled}
                onChange={(e) => setForm({ ...form, ai_reports_enabled: e.target.checked })}
                className="rounded border-input text-primary focus:ring-primary"
              />
              Enable AI Reports & Incident Analysis
            </label>

            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.is_active}
                onChange={(e) => setForm({ ...form, is_active: e.target.checked })}
                className="rounded border-input text-primary focus:ring-primary"
              />
              Available for Tenant Selection
            </label>

            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={form.is_trial}
                onChange={(e) => setForm({ ...form, is_trial: e.target.checked })}
                className="rounded border-input text-primary focus:ring-primary"
              />
              Mark as Free Trial Pack
            </label>
          </div>

          <div className="mt-6 flex items-center justify-end gap-3 pt-4 border-t border-border">
            <Button type="button" variant="outline" onClick={closeModal}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={createMutation.isPending || updateMutation.isPending}
            >
              {createMutation.isPending || updateMutation.isPending
                ? "Saving..."
                : editing
                ? "Save Changes"
                : "Create Pack"}
            </Button>
          </div>
        </form>
      </Modal>

      {/* Delete Confirmation Modal */}
      <Modal
        isOpen={deletePackItem !== null}
        onClose={() => setDeletePackItem(null)}
        title="Delete Subscription Pack"
        description={`Are you sure you want to delete pack "${deletePackItem?.name}"? This action cannot be undone.`}
        footer={
          <>
            <Button variant="outline" onClick={() => setDeletePackItem(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (deletePackItem) deleteMutation.mutate(deletePackItem.id)
              }}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? "Deleting..." : "Delete Permanently"}
            </Button>
          </>
        }
      >
        <p className="text-xs text-muted-foreground">
          Note: Packs currently assigned to active tenant subscriptions cannot be deleted.
        </p>
      </Modal>
    </div>
  )
}
