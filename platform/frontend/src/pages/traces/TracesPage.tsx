import { Page, PageHeader, SectionCard } from "@/components/shared/Page"

export default function TracesPage() {
  return (
    <Page>
      <PageHeader
        title="Traces"
        description="Distributed tracing (Tempo). Explore traces per service and drill down into spans."
      />
      <SectionCard title="Next step" description="Trace list + waterfall detail.">
        <div className="text-sm text-muted-foreground">
          Placeholder. We’ll add a traces table and a trace detail page.
        </div>
      </SectionCard>
    </Page>
  )
}
