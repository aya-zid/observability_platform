import { DashboardStatCard, DashboardSectionHeader } from "@/components/shared/DashboardComponents"
import { formatPercent } from "@/lib/queryHelpers"
import { useTranslation } from "@/i18n/LanguageContext"

export function PostgresStatusSection({
  up,
  scrapeError,
  replicaRole,
  cacheHitRatioPct,
}: {
  up: number | undefined
  scrapeError: number | undefined
  replicaRole: number | undefined
  cacheHitRatioPct: number | undefined
}) {
  const { t } = useTranslation()
  return (
    <>
      <DashboardSectionHeader title="Status" description="Instance health and replication role at a glance" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.up")} value={up === 1 ? "Up" : up === 0 ? "Down" : "-"} tone={up === 1 ? "good" : up === 0 ? "bad" : "neutral"} hint={t("pgDashboard.upHint")} />
        <DashboardStatCard label={t("pgDashboard.scrape")} value={scrapeError === 1 ? "Error" : scrapeError === 0 ? "Healthy" : "-"} tone={scrapeError === 1 ? "bad" : scrapeError === 0 ? "good" : "neutral"} hint={t("pgDashboard.scrapeHint")} />
        <DashboardStatCard label="Replica Role" value={replicaRole != null ? (replicaRole === 1 ? "Replica" : "Primary") : "-"} tone={replicaRole === 1 ? "warn" : "good"} hint={replicaRole === 1 ? "This is a read replica" : "This is the primary server"} />
        <DashboardStatCard label={t("pgDashboard.cacheHit")} value={cacheHitRatioPct != null ? formatPercent(cacheHitRatioPct) : "-"} tone={(cacheHitRatioPct ?? 100) < 95 ? "warn" : "good"} hint={t("pgDashboard.cacheHitHint")} />
      </div>
    </>
  )
}
