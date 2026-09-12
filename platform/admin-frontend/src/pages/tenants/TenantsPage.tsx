import { useState, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { Link } from "react-router-dom"
import {
  Search, UserCheck, UserX, Package, Eye, Building2,
  ChevronLeft, ChevronRight, ShieldAlert, Sparkles
} from "lucide-react"
import client from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/ui/empty-state"

type TenantItem = {
  id: number
  company_name: string
  contact_email: string
  is_active: boolean
  subscription_status: string
  hosts_used: number
  plan_name: string
  plan_price: number
  current_period_end: string | null
  banned_at: string | null
  ban_reason: string
  created_at: string
}

type PackItem = {
  id: number
  name: string
  price_per_month: number
}

export default function TenantsPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(0)
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "banned">("all")
  const limit = 15

  // Modals state
  const [banModalTenant, setBanModalTenant] = useState<TenantItem | null>(null)
  const [banReason, setBanReason] = useState("")

  const [packModalTenant, setPackModalTenant] = useState<TenantItem | null>(null)
  const [selectedPackId, setSelectedPackId] = useState<number>(0)
  const [availablePacks, setAvailablePacks] = useState<PackItem[]>([])

  const { data, isLoading } = useQuery({
    queryKey: ["admin", "tenants", search, page],
    queryFn: async () => {
      const res = await client.get("/dashboard/tenants", {
        params: { search, limit, offset: page * limit },
      })
      return res.data
    },
  })

  useEffect(() => {
    client.get("/packs").then((res) => {
      if (res.data) setAvailablePacks(res.data)
    }).catch(() => {})
  }, [])

  const banMutation = useMutation({
    mutationFn: async ({ id, reason }: { id: number; reason: string }) => {
      await client.post(`/tenants/${id}/ban`, { reason })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
      setBanModalTenant(null)
      setBanReason("")
    },
  })

  const unbanMutation = useMutation({
    mutationFn: async (id: number) => {
      await client.post(`/tenants/${id}/unban`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
    },
  })

  const assignPackMutation = useMutation({
    mutationFn: async ({ tenantId, packId }: { tenantId: number; packId: number }) => {
      await client.post(`/tenants/${tenantId}/change-plan`, { pack_id: packId })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
      setPackModalTenant(null)
    },
    onError: (err: unknown) => {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to assign pack")
    },
  })

  const rawTenants: TenantItem[] = data?.tenants || []
  const total = data?.total || 0

  const tenants = rawTenants.filter((t) => {
    if (statusFilter === "active") return t.is_active && !t.banned_at
    if (statusFilter === "banned") return !t.is_active || !!t.banned_at
    return true
  })

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-7xl mx-auto">
      {/* Header & Search */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h1 className="text-3xl font-extrabold tracking-tight text-foreground flex items-center gap-2">
            <Building2 className="w-7 h-7 text-primary" />
            Tenant Management
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Overview of all registered organizations, active subscriptions, and account status.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <div className="relative w-full sm:w-72">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
            <input
              type="text"
              placeholder="Search by company or email..."
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setPage(0)
              }}
              className="w-full rounded-md border border-input bg-background pl-9 pr-4 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>
      </div>

      {/* Filter Chips */}
      <div className="flex items-center gap-2 border-b border-border pb-3">
        <button
          onClick={() => setStatusFilter("all")}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold transition-colors ${
            statusFilter === "all"
              ? "bg-primary text-primary-foreground"
              : "bg-muted/60 text-muted-foreground hover:text-foreground"
          }`}
        >
          All Tenants ({total})
        </button>
        <button
          onClick={() => setStatusFilter("active")}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold transition-colors ${
            statusFilter === "active"
              ? "bg-emerald-500 text-white"
              : "bg-muted/60 text-muted-foreground hover:text-foreground"
          }`}
        >
          Active
        </button>
        <button
          onClick={() => setStatusFilter("banned")}
          className={`px-3 py-1.5 rounded-full text-xs font-semibold transition-colors ${
            statusFilter === "banned"
              ? "bg-rose-500 text-white"
              : "bg-muted/60 text-muted-foreground hover:text-foreground"
          }`}
        >
          Banned / Restricted
        </button>
      </div>

      {isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
          <Skeleton className="h-12 w-full" />
        </div>
      ) : tenants.length === 0 ? (
        <EmptyState
          icon={Building2}
          title="No tenants found"
          description="Try matching another search query or clear filters."
        />
      ) : (
        <>
          <div className="overflow-x-auto rounded-xl border border-border bg-card shadow-sm">
            <table className="w-full text-left text-sm">
              <thead className="bg-muted/50 text-xs font-semibold text-muted-foreground uppercase border-b border-border">
                <tr>
                  <th className="px-4 py-3">Company</th>
                  <th className="px-4 py-3">Email</th>
                  <th className="px-4 py-3">Pack</th>
                  <th className="px-4 py-3">Hosts</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Created</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {tenants.map((t) => (
                  <tr key={t.id} className="hover:bg-accent/30 transition-colors">
                    <td className="px-4 py-3 font-semibold text-foreground">
                      <Link
                        to={`/tenants/${t.id}`}
                        className="hover:text-primary transition-colors flex items-center gap-1.5"
                      >
                        {t.company_name}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{t.contact_email}</td>
                    <td className="px-4 py-3 font-medium">
                      {t.plan_name ? (
                        <span className="inline-flex items-center gap-1 text-foreground font-semibold">
                          {t.plan_name}
                          <span className="text-xs text-muted-foreground">(${t.plan_price})</span>
                        </span>
                      ) : (
                        <span className="text-muted-foreground italic text-xs">No Pack</span>
                      )}
                    </td>
                    <td className="px-4 py-3 font-mono font-medium">{t.hosts_used}</td>
                    <td className="px-4 py-3">
                      {t.banned_at ? (
                        <Badge variant="destructive" dot>
                          Banned
                        </Badge>
                      ) : t.is_active ? (
                        <Badge variant="success" dot>
                          Active
                        </Badge>
                      ) : (
                        <Badge variant="warning">Inactive</Badge>
                      )}
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">{t.created_at}</td>
                    <td className="px-4 py-3 text-right space-x-1">
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-8 px-2 text-xs gap-1"
                        asChild
                      >
                        <Link to={`/tenants/${t.id}`}>
                          <Eye className="w-3.5 h-3.5" /> Details
                        </Link>
                      </Button>

                      <Button
                        size="sm"
                        variant="outline"
                        className="h-8 px-2 text-xs gap-1"
                        onClick={() => {
                          setPackModalTenant(t)
                          setSelectedPackId(0)
                        }}
                      >
                        <Package className="w-3.5 h-3.5 text-primary" /> Assign Pack
                      </Button>

                      {t.banned_at || !t.is_active ? (
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-8 px-2 text-xs text-emerald-400 border-emerald-500/30 hover:bg-emerald-500/10 gap-1"
                          onClick={() => unbanMutation.mutate(t.id)}
                        >
                          <UserCheck className="w-3.5 h-3.5" /> Unban
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-8 px-2 text-xs text-rose-400 hover:bg-rose-500/10 gap-1"
                          onClick={() => {
                            setBanModalTenant(t)
                            setBanReason("")
                          }}
                        >
                          <UserX className="w-3.5 h-3.5" /> Ban
                        </Button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Pagination */}
          <div className="flex items-center justify-between border-t border-border pt-4">
            <p className="text-xs text-muted-foreground">
              Showing page {page + 1} of {Math.ceil(total / limit) || 1} ({total} total tenants)
            </p>
            <div className="flex items-center gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="gap-1"
              >
                <ChevronLeft className="w-4 h-4" /> Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setPage((p) => p + 1)}
                disabled={(page + 1) * limit >= total}
                className="gap-1"
              >
                Next <ChevronRight className="w-4 h-4" />
              </Button>
            </div>
          </div>
        </>
      )}

      {/* Ban Tenant Modal */}
      <Modal
        isOpen={banModalTenant !== null}
        onClose={() => setBanModalTenant(null)}
        title="Ban Tenant Account"
        description={`Restrict access for "${banModalTenant?.company_name}". They will not be able to log in or ingest telemetry data.`}
        footer={
          <>
            <Button variant="outline" onClick={() => setBanModalTenant(null)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (banModalTenant) {
                  banMutation.mutate({ id: banModalTenant.id, reason: banReason })
                }
              }}
              disabled={banMutation.isPending}
            >
              {banMutation.isPending ? "Banning..." : "Confirm Ban"}
            </Button>
          </>
        }
      >
        <div>
          <label className="block text-xs font-medium text-foreground mb-1">
            Reason for ban
          </label>
          <textarea
            rows={3}
            value={banReason}
            onChange={(e) => setBanReason(e.target.value)}
            placeholder="e.g. Terms of Service violation, non-payment of invoice..."
            className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
      </Modal>

      {/* Assign Pack Modal */}
      <Modal
        isOpen={packModalTenant !== null}
        onClose={() => setPackModalTenant(null)}
        title="Directly Assign Pack"
        description={`Assign a plan/pack directly to "${packModalTenant?.company_name}".`}
        footer={
          <>
            <Button variant="outline" onClick={() => setPackModalTenant(null)}>
              Cancel
            </Button>
            <Button
              onClick={() => {
                if (packModalTenant && selectedPackId > 0) {
                  assignPackMutation.mutate({ tenantId: packModalTenant.id, packId: selectedPackId })
                }
              }}
              disabled={selectedPackId === 0 || assignPackMutation.isPending}
            >
              {assignPackMutation.isPending ? "Assigning..." : "Confirm Assignment"}
            </Button>
          </>
        }
      >
        <div>
          <label className="block text-xs font-medium text-foreground mb-1">
            Select Pack
          </label>
          <select
            value={selectedPackId}
            onChange={(e) => setSelectedPackId(Number(e.target.value))}
            className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <option value={0}>-- Select a pack --</option>
            {availablePacks.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name} (${p.price_per_month}/mo)
              </option>
            ))}
          </select>
        </div>
      </Modal>
    </div>
  )
}
