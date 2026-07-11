import { DashboardStatCard, DashboardSectionHeader, DashboardLinePanel, DashboardBarPanel } from "@/components/shared/DashboardComponents"
import { PG_CHART_COLORS } from "./PostgresColors"
import { formatPercent, formatNumber, formatDuration } from "@/lib/queryHelpers"
import { useTranslation } from "@/i18n/LanguageContext"

export function PostgresConnectionsSection({
  connectionSaturationPct,
  maxConnections,
  idleInTx,
  maxTxDurationSeconds,
  connectionsData,
  activityData,
  connByDatabase,
  connSaturationByDb,
}: {
  connectionSaturationPct: number | undefined
  maxConnections: number | undefined
  idleInTx: number | null
  maxTxDurationSeconds: number | undefined
  connectionsData: Record<string, number | string>[]
  activityData: Record<string, number | string>[]
  connByDatabase: Array<{ label: string; count: number }>
  connSaturationByDb: Array<{ label: string; count: number }>
}) {
  const { t } = useTranslation()
  return (
    <>
      <DashboardSectionHeader title="Connections" description="Connection pool saturation, per-database breakdown, and idle-in-transaction tracking" />
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        <DashboardStatCard label={t("pgDashboard.connSat")} value={connectionSaturationPct != null ? formatPercent(connectionSaturationPct) : "-"} tone={(connectionSaturationPct ?? 0) > 85 ? "bad" : (connectionSaturationPct ?? 0) > 70 ? "warn" : "good"} hint={t("pgDashboard.connSatHint", maxConnections ? formatNumber(maxConnections, 0) : "-")} />
        <DashboardStatCard label={t("pgDashboard.idleTx")} value={idleInTx != null ? formatNumber(idleInTx, 0) : "-"} tone={(idleInTx ?? 0) > 0 ? "warn" : "good"} hint={t("pgDashboard.idleTxHint")} />
        <DashboardStatCard label={t("pgDashboard.maxTxDuration")} value={maxTxDurationSeconds != null ? formatDuration(maxTxDurationSeconds) : "-"} tone={(maxTxDurationSeconds ?? 0) > 30 ? "warn" : "good"} hint="Longest active transaction" />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardLinePanel title={t("pgDashboard.chartConnections")} data={connectionsData} lines={[{ dataKey: "connections", name: "connections", color: PG_CHART_COLORS.blue }]} />
        <DashboardLinePanel
          title={t("pgDashboard.chartActivity")}
          data={activityData}
          lines={[
            { dataKey: "active", name: "active", color: PG_CHART_COLORS.green },
            { dataKey: "idle", name: "idle", color: PG_CHART_COLORS.slate },
            { dataKey: "idle in transaction", name: "idle in transaction", color: PG_CHART_COLORS.amber },
          ]}
        />
      </div>
      <div className="grid gap-4 xl:grid-cols-2">
        <DashboardBarPanel title={t("pgDashboard.chartConnsByDb")} data={connByDatabase.map((d) => ({ name: d.label, value: d.count }))} />
        <DashboardBarPanel title="Conn Saturation % by DB" data={connSaturationByDb.map((d) => ({ name: d.label, value: d.count }))} formatter={(v) => formatPercent(v)} />
      </div>
    </>
  )
}
