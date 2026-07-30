import { type PostgresDatabaseHealth } from "@/api/postgres"
import { SectionCard } from "@/components/shared/Page"
import { cn } from "@/lib/utils"
import { formatBytes, formatNumber, formatPercent } from "@/lib/queryHelpers"
import { useTranslation } from "@/i18n/LanguageContext"

export function PostgresDatabaseHealthTable({ rows }: { rows: PostgresDatabaseHealth[] }) {
  const { t } = useTranslation()
  return (
    <SectionCard title={t("pgDashboard.healthTitle")} description={t("pgDashboard.healthDesc")}>
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-sm uppercase text-muted-foreground">
            <tr className="border-b border-border">
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColDb")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColSize")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColConns")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColCache")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColTempFiles")}</th>
              <th className="py-2 pr-4 text-left">{t("pgDashboard.healthColTempBytes")}</th>
              <th className="py-2 text-left">{t("pgDashboard.healthColRollback")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length ? rows.map((row) => (
              <tr key={row.database} className="border-b border-border/60">
                <td className="py-2 pr-4 font-mono text-sm">{row.database}</td>
                <td className="py-2 pr-4">{formatBytes(row.size_bytes)}</td>
                <td className="py-2 pr-4">{formatNumber(row.connections, 0)}</td>
                <td className={cn("py-2 pr-4", row.cache_hit_ratio_pct < 95 ? "text-amber-600 dark:text-amber-400" : "")}>{formatPercent(row.cache_hit_ratio_pct)}</td>
                <td className={cn("py-2 pr-4", row.temp_files_per_second > 0 ? "text-amber-600 dark:text-amber-400" : "")}>{formatNumber(row.temp_files_per_second, 3)}</td>
                <td className={cn("py-2 pr-4", row.temp_bytes_per_second > 0 ? "text-amber-600 dark:text-amber-400" : "")}>{formatBytes(row.temp_bytes_per_second)}/s</td>
                <td className={cn("py-2", row.rollback_rate_pct > 2 ? "text-red-500" : row.rollback_rate_pct > 1 ? "text-amber-600 dark:text-amber-400" : "")}>{formatPercent(row.rollback_rate_pct)}</td>
              </tr>
            )) : (
              <tr>
                <td colSpan={7} className="py-6 text-center text-sm text-muted-foreground">{t("pgDashboard.healthEmpty")}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </SectionCard>
  )
}
