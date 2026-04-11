import { useQuery } from "@tanstack/react-query"

import { getMe } from "@/api/tenant"
import { Page, PageHeader, SectionCard } from "@/components/shared/Page"

export default function ProfilePage() {
  const q = useQuery({
    queryKey: ["me"],
    queryFn: getMe,
    refetchOnWindowFocus: false,
    retry: 0,
  })

  return (
    <Page>
      <PageHeader
        title="Profile"
        description="User settings and preferences."
      />
      <SectionCard title="Current user" description="Identity resolved from your authenticated session.">
        {q.isLoading ? (
          <div className="text-sm text-muted-foreground">Loading profile…</div>
        ) : q.isError ? (
          <div className="text-sm text-destructive">Unable to load profile.</div>
        ) : (
          <div className="grid gap-2 text-sm">
            <div>
              <span className="text-muted-foreground">Full name:</span> {q.data?.full_name}
            </div>
            <div>
              <span className="text-muted-foreground">Email:</span> {q.data?.email}
            </div>
            <div>
              <span className="text-muted-foreground">Role:</span> {q.data?.role}
            </div>
            <div>
              <span className="text-muted-foreground">Company:</span> {q.data?.company_name}
            </div>
          </div>
        )}
      </SectionCard>
    </Page>
  )
}
