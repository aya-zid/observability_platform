import { useState, useEffect } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useParams, useNavigate } from "react-router-dom"
import {
  ArrowLeft, Ban, UserCheck, RefreshCw, Building2, Mail, Phone, Globe,
  Server, Calendar, ShieldAlert, Sparkles, Package, Clock, Cpu
} from "lucide-react"
import client from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { Skeleton } from "@/components/ui/skeleton"

type TenantDetail = {
  id: number
  company_name: string
  contact_email: string
  contact_phone: string
  is_active: boolean
  subscription_status: string
  timezone: string
  hosts_used: number
  plan_name: string
  plan_price: number
  log_retention_days: number
  metrics_retention_days: number
  max_hosts: number
  current_period_end: string | null
  started_at: string
  banned_at: string | null
  ban_reason: string
  ai_reports_enabled: boolean
  users_count: number
  created_at: string
}

type PackItem = {
  id: number
  name: string
  price_per_month: number
}

export default function TenantDetailPage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  // Modal states
  const [showBanModal, setShowBanModal] = useState(false)
  const [banReason, setBanReason] = useState("")

  const [showRenewModal, setShowRenewModal] = useState(false)
  const [renewMonths, setRenewMonths] = useState(1)

  const [showPackModal, setShowPackModal] = useState(false)
  const [selectedPackId, setSelectedPackId] = useState(0)
  const [packs, setPacks] = useState<PackItem[]>([])

  const { data, isLoading } = useQuery<TenantDetail>({
    queryKey: ["admin", "tenant", id],
    queryFn: async () => {
      const res = await client.get(`/dashboard/tenants/${id}`)
      return res.data
    },
    enabled: !!id,
  })

  useEffect(() => {
    client.get("/packs").then((res) => {
      if (res.data) setPacks(res.data)
    }).catch(() => {})
  }, [])

  const banMutation = useMutation({
    mutationFn: async (reason: string) => {
      await client.post(`/tenants/${id}/ban`, { reason })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
      setShowBanModal(false)
      setBanReason("")
    },
  })

  const unbanMutation = useMutation({
    mutationFn: async () => {
      await client.post(`/tenants/${id}/unban`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
    },
  })

  const renewMutation = useMutation({
    mutationFn: async (months: number) => {
      await client.post(`/tenants/${id}/renew`, { duration_months: months })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
      setShowRenewModal(false)
    },
  })

  const changePackMutation = useMutation({
    mutationFn: async (packId: number) => {
      await client.post(`/tenants/${id}/change-plan`, { pack_id: packId })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
      setShowPackModal(false)
    },
    onError: (err: unknown) => {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to change pack")
    },
  })

  if (isLoading) {
    return (
      <div className="p-6 md:p-8 space-y-6 max-w-5xl mx-auto">
        <Skeleton className="h-10 w-48" />
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
          <Skeleton className="h-28" />
        </div>
      </div>
    )
  }

  if (!data) {
    return (
      <div className="p-8 text-center space-y-4">
        <p className="text-destructive font-semibold">Tenant not found</p>
        <Button onClick={() => navigate("/tenants")}>Back to Tenants</Button>
      </div>
    )
  }

  const hostUsagePercent = data.max_hosts > 0 ? Math.min(100, Math.round((data.hosts_used / data.max_hosts) * 100)) : 0

  return (
    <div className="p-6 md:p-8 space-y-6 max-w-6xl mx-auto">
      {/* Top Bar Navigation */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            size="icon"
            onClick={() => navigate("/tenants")}
            className="h-9 w-9"
          >
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div>
            <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
              {data.company_name}
              {data.banned_at ? (
                <Badge variant="destructive" dot>Banned</Badge>
              ) : data.is_active ? (
                <Badge variant="success" dot>Active</Badge>
              ) : (
                <Badge variant="warning">Inactive</Badge>
              )}
            </h1>
            <p className="text-xs text-muted-foreground mt-0.5">Tenant ID: #{data.id} • Registered on {data.created_at}</p>
          </div>
        </div>

        {/* Primary Actions */}
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowPackModal(true)}
            className="gap-1.5"
          >
            <Package className="w-4 h-4 text-primary" />
            Change Pack
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setShowRenewModal(true)}
            disabled={renewMutation.isPending}
            className="gap-1.5"
          >
            <RefreshCw className={`w-4 h-4 ${renewMutation.isPending ? "animate-spin" : ""}`} />
            Renew Subscription
          </Button>
          {data.is_active && !data.banned_at ? (
            <Button
              variant="destructive"
              size="sm"
              onClick={() => setShowBanModal(true)}
              className="gap-1.5"
            >
              <Ban className="w-4 h-4" />
              Ban Tenant
            </Button>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => unbanMutation.mutate()}
              disabled={unbanMutation.isPending}
              className="gap-1.5 text-emerald-400 border-emerald-500/30 hover:bg-emerald-500/10"
            >
              <UserCheck className="w-4 h-4" />
              Unban Tenant
            </Button>
          )}
        </div>
      </div>

      {/* Banned Alert Banner */}
      {data.banned_at && (
        <div className="p-4 rounded-xl bg-destructive/10 border border-destructive/30 text-destructive flex items-start gap-3">
          <ShieldAlert className="w-5 h-5 mt-0.5 shrink-0" />
          <div className="text-sm">
            <div className="font-bold">Tenant Account Restricted</div>
            <div>Banned on {data.banned_at}. Reason: "{data.ban_reason || "No reason specified"}"</div>
          </div>
        </div>
      )}

      {/* Info Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {/* Contact Info */}
        <div className="p-5 rounded-xl border border-border bg-card space-y-3">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
            <Building2 className="w-4 h-4 text-primary" /> Contact Details
          </div>
          <div className="space-y-2 text-sm">
            <div className="flex items-center gap-2">
              <Mail className="w-4 h-4 text-muted-foreground shrink-0" />
              <span className="font-medium text-foreground">{data.contact_email}</span>
            </div>
            <div className="flex items-center gap-2">
              <Phone className="w-4 h-4 text-muted-foreground shrink-0" />
              <span className="text-muted-foreground">{data.contact_phone || "Not provided"}</span>
            </div>
            <div className="flex items-center gap-2">
              <Globe className="w-4 h-4 text-muted-foreground shrink-0" />
              <span className="text-muted-foreground">{data.timezone}</span>
            </div>
          </div>
        </div>

        {/* Current Pack & Subscription */}
        <div className="p-5 rounded-xl border border-border bg-card space-y-3">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
            <Package className="w-4 h-4 text-primary" /> Subscription Plan
          </div>
          <div className="space-y-1">
            <div className="text-xl font-extrabold text-foreground">
              {data.plan_name || "Free Trial / No Pack"}
            </div>
            <div className="text-sm text-primary font-mono font-semibold">
              ${data.plan_price}/month
            </div>
            <div className="text-xs text-muted-foreground pt-1 flex items-center gap-1">
              <Clock className="w-3.5 h-3.5" />
              Period End: <strong className="text-foreground">{data.current_period_end || "N/A"}</strong>
            </div>
          </div>
        </div>

        {/* Resource Limits */}
        <div className="p-5 rounded-xl border border-border bg-card space-y-3">
          <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-1.5">
            <Cpu className="w-4 h-4 text-primary" /> Host Allocation & Usage
          </div>
          <div className="space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-muted-foreground">Hosts Monitored:</span>
              <span className="font-bold text-foreground">{data.hosts_used} / {data.max_hosts}</span>
            </div>
            <div className="w-full bg-muted h-2.5 rounded-full overflow-hidden">
              <div
                className={`h-full transition-all duration-300 ${
                  hostUsagePercent > 90 ? "bg-rose-500" : hostUsagePercent > 75 ? "bg-amber-500" : "bg-primary"
                }`}
                style={{ width: `${hostUsagePercent}%` }}
              />
            </div>
            <div className="text-[11px] text-muted-foreground flex justify-between">
              <span>{hostUsagePercent}% utilized</span>
              <span>{data.users_count} user accounts</span>
            </div>
          </div>
        </div>
      </div>

      {/* Advanced Features & Retention Parameters */}
      <div className="p-5 rounded-xl border border-border bg-card space-y-4">
        <h3 className="text-base font-bold text-foreground">Pack Configuration & Retention Policies</h3>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
          <div className="p-4 rounded-lg bg-accent/30 border border-border/40">
            <div className="text-xs text-muted-foreground">Log Data Retention</div>
            <div className="text-lg font-bold text-foreground mt-1">{data.log_retention_days} Days</div>
          </div>
          <div className="p-4 rounded-lg bg-accent/30 border border-border/40">
            <div className="text-xs text-muted-foreground">Metrics Data Retention</div>
            <div className="text-lg font-bold text-foreground mt-1">{data.metrics_retention_days} Days</div>
          </div>
          <div className="p-4 rounded-lg bg-accent/30 border border-border/40">
            <div className="text-xs text-muted-foreground">AI Incident Reports</div>
            <div className="text-lg font-bold text-foreground mt-1 flex items-center gap-1.5">
              {data.ai_reports_enabled ? (
                <>
                  <Sparkles className="w-4 h-4 text-amber-400" />
                  <span className="text-emerald-400">Enabled</span>
                </>
              ) : (
                <span className="text-muted-foreground">Disabled</span>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Ban Modal */}
      <Modal
        isOpen={showBanModal}
        onClose={() => setShowBanModal(false)}
        title="Ban Tenant Account"
        description={`Are you sure you want to ban "${data.company_name}"?`}
        footer={
          <>
            <Button variant="outline" onClick={() => setShowBanModal(false)}>
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={() => banMutation.mutate(banReason)}
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
            placeholder="Enter reason..."
            className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>
      </Modal>

      {/* Renew Modal */}
      <Modal
        isOpen={showRenewModal}
        onClose={() => setShowRenewModal(false)}
        title="Renew Subscription"
        description={`Extend subscription for "${data.company_name}".`}
        footer={
          <>
            <Button variant="outline" onClick={() => setShowRenewModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => renewMutation.mutate(renewMonths)}
              disabled={renewMutation.isPending}
            >
              {renewMutation.isPending ? "Renewing..." : "Confirm Renewal"}
            </Button>
          </>
        }
      >
        <div>
          <label className="block text-xs font-medium text-foreground mb-1">
            Duration (Months)
          </label>
          <select
            value={renewMonths}
            onChange={(e) => setRenewMonths(Number(e.target.value))}
            className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          >
            <option value={1}>1 Month</option>
            <option value={3}>3 Months</option>
            <option value={6}>6 Months</option>
            <option value={12}>12 Months (1 Year)</option>
          </select>
        </div>
      </Modal>

      {/* Change Pack Modal */}
      <Modal
        isOpen={showPackModal}
        onClose={() => setShowPackModal(false)}
        title="Directly Change Pack"
        description={`Assign a new pack to "${data.company_name}".`}
        footer={
          <>
            <Button variant="outline" onClick={() => setShowPackModal(false)}>
              Cancel
            </Button>
            <Button
              onClick={() => {
                if (selectedPackId > 0) changePackMutation.mutate(selectedPackId)
              }}
              disabled={selectedPackId === 0 || changePackMutation.isPending}
            >
              {changePackMutation.isPending ? "Updating..." : "Confirm Change"}
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
            {packs.map((p) => (
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
