import { ArrowRight, Layers, LineChart, ShieldCheck, Sparkles } from "lucide-react"
import { type ReactNode } from "react"
import { Link } from "react-router-dom"

import ThemeToggle from "@/components/shared/ThemeToggle"
import { Button } from "@/components/ui/button"

import "@/styles/landing.css"

function FeatureCard({
  title,
  description,
  icon,
}: {
  title: string
  description: string
  icon: ReactNode
}) {
  return (
    <div className="rounded-xl border border-border bg-card/70 backdrop-blur p-5">
      <div className="flex items-start gap-3">
        <div className="mt-0.5 text-primary">{icon}</div>
        <div className="min-w-0">
          <div className="font-medium">{title}</div>
          <div className="mt-1 text-sm text-muted-foreground">{description}</div>
        </div>
      </div>
    </div>
  )
}

export default function WelcomePage() {
  return (
    <div className="landing min-h-dvh bg-background">
      <header className="relative z-10">
        <div className="mx-auto max-w-6xl px-4 py-5 md:px-6 flex items-center justify-between">
          <Link to="/" className="flex items-center gap-2">
            <div className="size-9 rounded-lg bg-primary text-primary-foreground grid place-items-center font-semibold">
              OP
            </div>
            <div className="leading-tight">
              <div className="font-semibold tracking-tight">Observability Platform</div>
              <div className="text-xs text-muted-foreground">Metrics. Logs. Traces.</div>
            </div>
          </Link>

          <div className="flex items-center gap-2">
            <ThemeToggle />
            <Button render={<Link to="/login" />} variant="outline">
              Sign in
            </Button>
            <Button render={<Link to="/signup" />}>
              Get started <ArrowRight className="ml-1 size-4" />
            </Button>
          </div>
        </div>
      </header>

      <main className="relative z-10">
        <section className="mx-auto max-w-6xl px-4 pb-14 pt-10 md:px-6 md:pt-14">
          <div className="grid gap-10 md:grid-cols-[1.2fr_0.8fr] md:items-start">
            <div>
              <div className="inline-flex items-center gap-2 rounded-full border border-border bg-card/70 backdrop-blur px-3 py-1 text-xs text-muted-foreground">
                <Sparkles className="size-3.5" />
                Enterprise-ready, jury-friendly
              </div>

              <h1 className="mt-4 text-4xl md:text-5xl font-semibold tracking-tight">
                A cloud-grade observability suite,
                <span className="text-primary"> with soul</span>.
              </h1>

              <p className="mt-4 text-base md:text-lg text-muted-foreground max-w-xl">
                One place to explore metrics, logs and traces per tenant. Built to
                scale, designed to be a pleasure to use.
              </p>

              <div className="mt-6 flex flex-wrap gap-3">
                <Button
                  render={<Link to="/signup" />}
                  size="lg"
                >
                  Create your tenant <ArrowRight className="ml-1 size-4" />
                </Button>
                <Button
                  render={<Link to="/login" />}
                  size="lg"
                  variant="outline"
                >
                  I already have an account
                </Button>
              </div>

              <div className="mt-8 grid gap-3 sm:grid-cols-3">
                <div className="rounded-xl border border-border bg-card/70 backdrop-blur p-4">
                  <div className="text-sm font-medium">Multi-tenant</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    Isolation baked in.
                  </div>
                </div>
                <div className="rounded-xl border border-border bg-card/70 backdrop-blur p-4">
                  <div className="text-sm font-medium">Secure by default</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    HttpOnly cookies + rate limits.
                  </div>
                </div>
                <div className="rounded-xl border border-border bg-card/70 backdrop-blur p-4">
                  <div className="text-sm font-medium">Fast UX</div>
                  <div className="mt-1 text-xs text-muted-foreground">
                    React Query everywhere.
                  </div>
                </div>
              </div>
            </div>

            <div className="grid gap-4">
              <FeatureCard
                title="Metrics"
                description="Dashboards and charts backed by VictoriaMetrics."
                icon={<LineChart className="size-5" />}
              />
              <FeatureCard
                title="Logs"
                description="Search and filter logs with Loki-powered queries."
                icon={<Layers className="size-5" />}
              />
              <FeatureCard
                title="Traces"
                description="Distributed tracing with Tempo, ready for a waterfall view."
                icon={<ShieldCheck className="size-5" />}
              />
            </div>
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-4 pb-16 md:px-6">
          <div className="rounded-2xl border border-border bg-card/70 backdrop-blur p-6 md:p-8">
            <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
              <div>
                <div className="text-lg font-semibold tracking-tight">
                  Ready to impress?
                </div>
                <div className="text-sm text-muted-foreground">
                  Start with a tenant, then connect an agent using your API key.
                </div>
              </div>
              <div className="flex gap-2">
                <Button render={<Link to="/login" />} variant="outline">
                  Sign in
                </Button>
                <Button render={<Link to="/signup" />}>Create tenant</Button>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer className="relative z-10 border-t border-border">
        <div className="mx-auto max-w-6xl px-4 py-6 md:px-6 text-xs text-muted-foreground flex items-center justify-between">
          <div>© {new Date().getFullYear()} Observability Platform</div>
          <div className="hidden sm:block">Built for a final-year project demo</div>
        </div>
      </footer>
    </div>
  )
}
