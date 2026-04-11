import { Page, PageHeader, SectionCard } from "@/components/shared/Page"

export default function MetricsPage() {
  return (
    <Page>
      <PageHeader
        title="Metrics"
        description="Charts and dashboards backed by VictoriaMetrics (via the backend)."
      />
      <SectionCard title="Next step" description="First enterprise screen: time range + query + chart.">
        <div className="text-sm text-muted-foreground">
          Placeholder. We’ll add a query builder and Recharts graphs here.
        </div>
      </SectionCard>
    </Page>
  )
}
