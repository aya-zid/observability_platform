import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"
import { Link } from "react-router-dom"
import { Search, UserCheck, UserX } from "lucide-react"
import client from "@/api/client"

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

export default function TenantsPage() {
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(0)
  const limit = 20

  const { data, isLoading } = useQuery({
    queryKey: ["admin", "tenants", search, page],
    queryFn: async () => {
      const res = await client.get("/dashboard/tenants", {
        params: { search, limit, offset: page * limit },
      })
      return res.data
    },
  })

  const banMutation = useMutation({
    mutationFn: async ({ id, reason }: { id: number; reason: string }) => {
      await client.post(`/tenants/${id}/ban`, { reason })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin"] })
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

  const tenants: TenantItem[] = data?.tenants || []
  const total = data?.total || 0

  const handleBan = (id: number) => {
    const reason = prompt("Ban reason:")
    if (reason !== null) {
      banMutation.mutate({ id, reason })
    }
  }

  return (
    <div className="p-6 md:p-8 space-y-6">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold tracking-tight">Tenants</h1>
        <div className="relative">
          <Search className="absolute left-3 top-2.5 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            placeholder="Search tenants…"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setPage(0)
            }}
            className="pl-9 h-9 rounded-md border border-border bg-background px-3 text-sm outline-none focus:border-primary/50 focus:ring-1 focus:ring-primary/25"
          />
        </div>
      </div>

      {isLoading ? (
        <div className="text-sm text-muted-foreground">Loading tenants…</div>
      ) : (
        <>
          <div className="rounded-lg border border-border bg-card shadow-card overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/50">
                <tr>
                  <th className="text-left p-3 font-medium">Company</th>
                  <th className="text-left p-3 font-medium">Email</th>
                  <th className="text-left p-3 font-medium">Plan</th>
                  <th className="text-left p-3 font-medium">Hosts</th>
                  <th className="text-left p-3 font-medium">Status</th>
                  <th className="text-right p-3 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {tenants.map((t) => (
                  <tr key={t.id} className="border-t border-border">
                    <td className="p-3">
                      <Link
                        to={`/tenants/${t.id}`}
                        className="text-primary font-medium hover:underline"
                      >
                        {t.company_name}
                      </Link>
                    </td>
                    <td className="p-3 text-muted-foreground">{t.contact_email}</td>
                    <td className="p-3">{t.plan_name || "None"}</td>
                    <td className="p-3">{t.hosts_used}</td>
                    <td className="p-3">
                      <span
                        className={`inline-flex items-center rounded-full px-2 py-1 text-xs font-medium ${
                          t.is_active
                            ? "bg-status-green/15 text-status-green"
                            : "bg-status-red/15 text-status-red"
                        }`}
                      >
                        {t.subscription_status}
                      </span>
                    </td>
                    <td className="p-3 text-right">
                      {t.is_active ? (
                        <button
                          onClick={() => handleBan(t.id)}
                          className="inline-flex items-center gap-1 text-xs text-destructive hover:underline"
                        >
                          <UserX className="w-3.5 h-3.5" /> Ban
                        </button>
                      ) : (
                        <button
                          onClick={() => unbanMutation.mutate(t.id)}
                          className="inline-flex items-center gap-1 text-xs text-status-green hover:underline"
                        >
                          <UserCheck className="w-3.5 h-3.5" /> Unban
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
                {tenants.length === 0 && (
                  <tr>
                    <td colSpan={6} className="p-8 text-center text-muted-foreground">
                      No tenants found
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">Total: {total}</p>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((p) => Math.max(0, p - 1))}
                disabled={page === 0}
                className="h-8 px-3 text-sm border border-border rounded-md bg-background hover:bg-muted disabled:opacity-50"
              >
                Previous
              </button>
              <span className="text-sm px-3 py-1">Page {page + 1}</span>
              <button
                onClick={() => setPage((p) => p + 1)}
                disabled={(page + 1) * limit >= total}
                className="h-8 px-3 text-sm border border-border rounded-md bg-background hover:bg-muted disabled:opacity-50"
              >
                Next
              </button>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
