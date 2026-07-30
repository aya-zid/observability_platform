import { useTranslation } from "@/i18n/LanguageContext"
import { cn } from "@/lib/utils"

export function LogPagination({
  page,
  total,
  pageSize,
  hasMore,
  onChange,
}: {
  page: number
  total: number
  pageSize: number
  hasMore: boolean
  onChange: (page: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.ceil(total / pageSize)

  if (total <= 0) return null

  return (
    <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
      <span>
        {t("logs.showing")} {Math.min((page - 1) * pageSize + 1, total)}–{Math.min(page * pageSize, total)} / {total}
      </span>
      <div className="flex items-center gap-1.5">
        <button
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className={cn("px-1.5 py-0.5 rounded border border-border text-xs", page <= 1 ? "opacity-30 cursor-not-allowed" : "hover:bg-muted")}
        >
          {t("logs.previous")}
        </button>
        <span>
          {page} / {totalPages}
        </span>
        <button
          onClick={() => onChange(page + 1)}
          disabled={!hasMore && page >= totalPages}
          className={cn("px-1.5 py-0.5 rounded border border-border text-xs", !hasMore && page >= totalPages ? "opacity-30 cursor-not-allowed" : "hover:bg-muted")}
        >
          {t("logs.next")}
        </button>
      </div>
    </div>
  )
}
