import { useState, useEffect } from "react"
import client from "@/api/client"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Modal } from "@/components/ui/modal"
import { Tabs, type TabItem } from "@/components/ui/tabs"
import { Skeleton } from "@/components/ui/skeleton"
import { EmptyState } from "@/components/ui/empty-state"
import {
  CheckCircle2, XCircle, Clock, AlertTriangle, RefreshCw,
  Search, CreditCard, ShieldCheck, Inbox
} from "lucide-react"

interface SubscriptionRequest {
  id: number
  tenant_id: number
  tenant_name: string
  tenant_email: string
  plan_id: number
  plan_name: string
  plan_price: number
  status: "pending" | "approved" | "rejected"
  notes: string
  admin_notes: string
  reviewed_at: string | null
  created_at: string
}

interface SubscriptionItem {
  id: number
  tenant_id: number
  tenant_name: string
  tenant_email: string
  plan_name: string
  plan_price: number
  status: string
  started_at: string
  current_period_end: string | null
  notes: string
}

export default function SubscriptionsPage() {
  const [activeTab, setActiveTab] = useState<string>("requests")
  const [requests, setRequests] = useState<SubscriptionRequest[]>([])
  const [allSubs, setAllSubs] = useState<SubscriptionItem[]>([])
  const [expiredSubs, setExpiredSubs] = useState<SubscriptionItem[]>([])
  const [loading, setLoading] = useState<boolean>(true)

  // Filters & search
  const [search, setSearch] = useState<string>("")
  const [statusFilter, setStatusFilter] = useState<string>("all")

  // Modal review state
  const [selectedReq, setSelectedReq] = useState<SubscriptionRequest | null>(null)
  const [reviewAction, setReviewAction] = useState<"approved" | "rejected">("approved")
  const [adminNotes, setAdminNotes] = useState<string>("")
  const [submittingReview, setSubmittingReview] = useState<boolean>(false)

  // Renew modal state
  const [renewTenantId, setRenewTenantId] = useState<number | null>(null)
  const [renewMonths, setRenewMonths] = useState<number>(1)
  const [submittingRenew, setSubmittingRenew] = useState<boolean>(false)

  const fetchData = async () => {
    setLoading(true)
    try {
      const [reqRes, allRes, expRes] = await Promise.all([
        client.get("/subscriptions/requests"),
        client.get("/subscriptions"),
        client.get("/subscriptions/expired"),
      ])
      setRequests(reqRes.data || [])
      setAllSubs(allRes.data || [])
      setExpiredSubs(expRes.data || [])
    } catch {
      // error handled silently or with fallback empty array
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchData()
  }, [])

  const handleOpenReview = (req: SubscriptionRequest, action: "approved" | "rejected") => {
    setSelectedReq(req)
    setReviewAction(action)
    setAdminNotes("")
  }

  const handleReviewSubmit = async () => {
    if (!selectedReq) return
    setSubmittingReview(true)
    try {
      await client.post(`/subscriptions/requests/${selectedReq.id}/review`, {
        status: reviewAction,
        admin_notes: adminNotes,
      })
      setSelectedReq(null)
      fetchData()
    } catch (err: unknown) {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to submit review")
    } finally {
      setSubmittingReview(false)
    }
  }

  const handleRenewSubmit = async () => {
    if (!renewTenantId) return
    setSubmittingRenew(true)
    try {
      await client.post(`/tenants/${renewTenantId}/renew`, {
        duration_months: Number(renewMonths),
      })
      setRenewTenantId(null)
      fetchData()
    } catch (err: unknown) {
      const errorObj = err as { response?: { data?: { error?: string } } }
      alert(errorObj.response?.data?.error || "Failed to renew subscription")
    } finally {
      setSubmittingRenew(false)
    }
  }

  const pendingRequestsCount = requests.filter((r) => r.status === "pending").length

  const tabs: TabItem[] = [
    {
      id: "requests",
      label: "Subscription Requests",
      count: pendingRequestsCount,
      icon: Inbox,
    },
    {
      id: "all",
      label: "All Subscriptions",
      count: allSubs.length,
      icon: CreditCard,
    },
    {
      id: "expired",
      label: "Expired Subscriptions",
      count: expiredSubs.length,
      icon: AlertTriangle,
    },
  ]

  const filteredRequests = requests.filter((r) => {
    const matchesSearch =
      r.tenant_name.toLowerCase().includes(search.toLowerCase()) ||
      r.tenant_email.toLowerCase().includes(search.toLowerCase()) ||
      r.plan_name.toLowerCase().includes(search.toLowerCase())
    const matchesStatus = statusFilter === "all" || r.status === statusFilter
    return matchesSearch && matchesStatus
  })

  const filteredAllSubs = allSubs.filter((s) => {
    return (
      s.tenant_name.toLowerCase().includes(search.toLowerCase()) ||
      s.tenant_email.toLowerCase().includes(search.toLowerCase()) ||
      s.plan_name.toLowerCase().includes(search.toLowerCase())
    )
  })

  return (
    <div className="p-6 space-y-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground flex items-center gap-2">
            <CreditCard className="w-7 h-7 text-primary" />
            Subscription & Pack Demand Management
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            Review enterprise pack requests, manage active subscriptions, and handle renewals.
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={fetchData}
          disabled={loading}
          className="gap-2 shrink-0 self-start md:self-auto"
        >
          <RefreshCw className={`w-4 h-4 ${loading ? "animate-spin" : ""}`} />
          Refresh Queue
        </Button>
      </div>

      {/* Tabs */}
      <Tabs tabs={tabs} activeTab={activeTab} onChange={setActiveTab} />

      {/* Search & Filter Bar */}
      <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
        <div className="relative w-full sm:w-80">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search tenant, email, or pack..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full rounded-md border border-input bg-background pl-9 pr-4 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
          />
        </div>

        {activeTab === "requests" && (
          <div className="flex items-center gap-2 w-full sm:w-auto">
            <span className="text-xs font-medium text-muted-foreground">Status:</span>
            <select
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value)}
              className="rounded-md border border-input bg-background px-3 py-1.5 text-xs focus:outline-none focus:ring-1 focus:ring-ring"
            >
              <option value="all">All Statuses</option>
              <option value="pending">Pending Only</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected</option>
            </select>
          </div>
        )}
      </div>

      {/* Content based on Active Tab */}
      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-24 w-full" />
        </div>
      ) : activeTab === "requests" ? (
        filteredRequests.length === 0 ? (
          <EmptyState
            icon={Inbox}
            title="No subscription requests found"
            description="When tenants pay IRL or request an enterprise pack upgrade, requests will appear here for review."
          />
        ) : (
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
            {filteredRequests.map((req) => (
              <div
                key={req.id}
                className={`p-5 rounded-xl border transition-all duration-200 flex flex-col justify-between ${
                  req.status === "pending"
                    ? "bg-card border-amber-500/30 shadow-lg shadow-amber-500/5 ring-1 ring-amber-500/20"
                    : "bg-card/60 border-border"
                }`}
              >
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <h3 className="font-semibold text-foreground text-base">{req.tenant_name}</h3>
                      <p className="text-xs text-muted-foreground">{req.tenant_email}</p>
                    </div>
                    {req.status === "pending" && (
                      <Badge variant="warning" dot>
                        Pending Review
                      </Badge>
                    )}
                    {req.status === "approved" && (
                      <Badge variant="success">Approved</Badge>
                    )}
                    {req.status === "rejected" && (
                      <Badge variant="destructive">Rejected</Badge>
                    )}
                  </div>

                  <div className="p-3 rounded-lg bg-accent/40 border border-border/50 space-y-1">
                    <div className="text-xs text-muted-foreground">Requested Pack</div>
                    <div className="flex items-center justify-between">
                      <span className="font-bold text-foreground">{req.plan_name}</span>
                      <span className="font-mono text-sm text-primary font-semibold">
                        ${req.plan_price}/mo
                      </span>
                    </div>
                  </div>

                  {req.notes && (
                    <div className="text-xs text-muted-foreground bg-muted/30 p-2.5 rounded-md border border-border/40">
                      <span className="font-semibold text-foreground">Tenant note: </span>
                      "{req.notes}"
                    </div>
                  )}

                  {req.admin_notes && (
                    <div className="text-xs text-muted-foreground bg-accent/20 p-2.5 rounded-md border border-border/40">
                      <span className="font-semibold text-foreground">Admin response: </span>
                      "{req.admin_notes}"
                    </div>
                  )}

                  <div className="text-[11px] text-muted-foreground flex items-center gap-1">
                    <Clock className="w-3 h-3" /> Requested on {req.created_at}
                  </div>
                </div>

                {req.status === "pending" && (
                  <div className="mt-4 pt-3 border-t border-border flex items-center gap-2">
                    <Button
                      size="sm"
                      variant="default"
                      className="w-full bg-emerald-600 hover:bg-emerald-700 text-white gap-1.5"
                      onClick={() => handleOpenReview(req, "approved")}
                    >
                      <CheckCircle2 className="w-4 h-4" />
                      Approve & Grant Pack
                    </Button>
                    <Button
                      size="sm"
                      variant="destructive"
                      className="w-full gap-1.5"
                      onClick={() => handleOpenReview(req, "rejected")}
                    >
                      <XCircle className="w-4 h-4" />
                      Reject
                    </Button>
                  </div>
                )}
              </div>
            ))}
          </div>
        )
      ) : activeTab === "all" ? (
        filteredAllSubs.length === 0 ? (
          <EmptyState
            icon={CreditCard}
            title="No subscriptions found"
            description="Active tenant subscriptions will be listed here."
          />
        ) : (
          <div className="overflow-x-auto rounded-xl border border-border bg-card">
            <table className="w-full text-left text-sm">
              <thead className="bg-muted/50 text-xs font-semibold text-muted-foreground uppercase border-b border-border">
                <tr>
                  <th className="px-4 py-3">Tenant</th>
                  <th className="px-4 py-3">Pack</th>
                  <th className="px-4 py-3">Price</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Period End</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredAllSubs.map((sub) => (
                  <tr key={sub.id} className="hover:bg-accent/30 transition-colors">
                    <td className="px-4 py-3 font-medium text-foreground">
                      <div>{sub.tenant_name}</div>
                      <div className="text-xs text-muted-foreground">{sub.tenant_email}</div>
                    </td>
                    <td className="px-4 py-3 font-semibold text-foreground">{sub.plan_name}</td>
                    <td className="px-4 py-3 font-mono">${sub.plan_price}/mo</td>
                    <td className="px-4 py-3">
                      <Badge
                        variant={
                          sub.status === "active"
                            ? "success"
                            : sub.status === "expired"
                            ? "destructive"
                            : "outline"
                        }
                      >
                        {sub.status}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      {sub.current_period_end || "N/A"}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => setRenewTenantId(sub.tenant_id)}
                      >
                        Renew Subscription
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      ) : (
        /* Expired Tab */
        expiredSubs.length === 0 ? (
          <EmptyState
            icon={ShieldCheck}
            title="No expired subscriptions!"
            description="All tenant subscriptions are currently active and up to date."
          />
        ) : (
          <div className="overflow-x-auto rounded-xl border border-destructive/30 bg-card">
            <table className="w-full text-left text-sm">
              <thead className="bg-destructive/10 text-xs font-semibold text-destructive uppercase border-b border-destructive/20">
                <tr>
                  <th className="px-4 py-3">Tenant</th>
                  <th className="px-4 py-3">Expired Pack</th>
                  <th className="px-4 py-3">Expired On</th>
                  <th className="px-4 py-3 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {expiredSubs.map((sub) => (
                  <tr key={sub.id} className="hover:bg-accent/30 transition-colors">
                    <td className="px-4 py-3 font-medium text-foreground">
                      <div>{sub.tenant_name}</div>
                      <div className="text-xs text-muted-foreground">{sub.tenant_email}</div>
                    </td>
                    <td className="px-4 py-3 font-semibold text-foreground">{sub.plan_name}</td>
                    <td className="px-4 py-3 text-xs text-destructive font-mono">
                      {sub.current_period_end}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        size="sm"
                        variant="default"
                        onClick={() => setRenewTenantId(sub.tenant_id)}
                      >
                        Renew Now
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )
      )}

      {/* Modal: Review Subscription Request */}
      <Modal
        isOpen={selectedReq !== null}
        onClose={() => setSelectedReq(null)}
        title={
          reviewAction === "approved" ? "Approve Subscription Request" : "Reject Subscription Request"
        }
        description={
          selectedReq
            ? `${reviewAction === "approved" ? "Approve" : "Reject"} pack "${selectedReq.plan_name}" for enterprise ${selectedReq.tenant_name}.`
            : ""
        }
        footer={
          <>
            <Button variant="outline" onClick={() => setSelectedReq(null)}>
              Cancel
            </Button>
            <Button
              variant={reviewAction === "approved" ? "default" : "destructive"}
              onClick={handleReviewSubmit}
              disabled={submittingReview}
              className={reviewAction === "approved" ? "bg-emerald-600 hover:bg-emerald-700 text-white" : ""}
            >
              {submittingReview ? "Processing..." : `Confirm ${reviewAction === "approved" ? "Approval" : "Rejection"}`}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="p-3 rounded-lg bg-accent/40 text-sm space-y-1">
            <div><span className="text-muted-foreground">Tenant:</span> <strong>{selectedReq?.tenant_name}</strong> ({selectedReq?.tenant_email})</div>
            <div><span className="text-muted-foreground">Requested Pack:</span> <strong>{selectedReq?.plan_name}</strong> (${selectedReq?.plan_price}/mo)</div>
            {selectedReq?.notes && <div><span className="text-muted-foreground">Notes:</span> "{selectedReq.notes}"</div>}
          </div>

          <div>
            <label className="block text-xs font-medium text-foreground mb-1">
              Admin Notes / Response Message (Optional)
            </label>
            <textarea
              rows={3}
              value={adminNotes}
              onChange={(e) => setAdminNotes(e.target.value)}
              placeholder="e.g. Wire payment confirmed. Enterprise pack activated for 12 months."
              className="w-full rounded-md border border-input bg-background p-2.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
        </div>
      </Modal>

      {/* Modal: Renew Subscription */}
      <Modal
        isOpen={renewTenantId !== null}
        onClose={() => setRenewTenantId(null)}
        title="Renew Subscription"
        description="Extend the subscription period for this tenant."
        footer={
          <>
            <Button variant="outline" onClick={() => setRenewTenantId(null)}>
              Cancel
            </Button>
            <Button onClick={handleRenewSubmit} disabled={submittingRenew}>
              {submittingRenew ? "Renewing..." : "Confirm Renewal"}
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
            <option value={24}>24 Months (2 Years)</option>
          </select>
        </div>
      </Modal>
    </div>
  )
}
