import type { MariaDBDashboardSummary } from "@/api/mariadb"
import { SectionCard } from "@/components/shared/Page"
import { useTranslation } from "@/i18n/LanguageContext"
import { formatDuration, formatNumber } from "@/lib/queryHelpers"

export function MariaDBDigestsTable({
  title,
  digests,
  valueFn,
  valueLabel,
  valueFormatter,
}: {
  title: string
  digests: MariaDBDashboardSummary["top_query_digests"]
  valueFn: (d: MariaDBDashboardSummary["top_query_digests"][number]) => number
  valueLabel: string
  valueFormatter: (v: number) => string
}) {
  const { t } = useTranslation()
  return (
    <SectionCard title={title}>
      <div className="overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-xs uppercase text-muted-foreground">
            <tr className="border-b border-border">
              <th className="py-2 pr-4 text-left">Query</th>
              <th className="py-2 pr-4 text-left">DB</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.execs")}</th>
              <th className="py-2 pr-4 text-right">{valueLabel}</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.avg")}</th>
              <th className="py-2 pr-4 text-right">{t("mdbDashboard.rowsExam")}</th>
              <th className="py-2 text-right">{t("mdbDashboard.rowsSent")}</th>
            </tr>
          </thead>
          <tbody>
            {digests.length ? digests.map((d, i) => (
              <tr key={d.digest || i} className="border-b border-border/60 hover:bg-muted/30">
                <td className="max-w-[360px] truncate py-2 pr-4 font-mono text-xs" title={d.digest_text || d.digest}>{d.digest_text || `${d.digest.slice(0, 20)}…`}</td>
                <td className="py-2 pr-4 font-mono text-xs">{d.database}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatNumber(d.exec_count, 0)}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{valueFormatter(valueFn(d))}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatDuration(d.avg_time_seconds)}</td>
                <td className="py-2 pr-4 text-right tabular-nums">{formatNumber(d.rows_examined, 0)}</td>
                <td className="py-2 text-right tabular-nums">{formatNumber(d.rows_sent, 0)}</td>
              </tr>
            )) : (
              <tr><td colSpan={7} className="py-6 text-center text-sm text-muted-foreground">{t("mdbDashboard.noDigests")}</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </SectionCard>
  )
}
