import { useState, type ComponentType, type ReactNode } from "react"
import { ChevronDown, Terminal, CheckCircle } from "lucide-react"
import MarkdownRenderer from "@/components/MarkdownRenderer"

interface GuideStep {
  title: string
  description?: string
  code?: string
  text?: string
  note?: string
}

interface GuideData {
  intro: string
  steps?: GuideStep[]
  result?: string
}

export default function CollapsibleGuide({
  title,
  icon: Icon,
  guide,
  content,
  children,
}: {
  title: string
  icon: ComponentType<any>
  guide: GuideData
  content?: string
  children?: ReactNode
}) {
  const [open, setOpen] = useState(false)
  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden transition-all">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-3 px-5 py-4 text-left hover:bg-muted/50 transition-colors"
      >
        <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary/80 to-primary flex items-center justify-center shrink-0">
          <Icon className="w-4 h-4 text-primary-foreground" />
        </div>
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-foreground">{title}</h3>
          <p className="text-sm text-muted-foreground mt-0.5">{guide.intro}</p>
        </div>
        <ChevronDown
          className={`w-4 h-4 text-muted-foreground transition-transform duration-200 ${open ? "rotate-180" : ""}`}
        />
      </button>

      {open && (
        <div className="px-5 pb-5 pt-3 border-t border-border">
          {children ? (
            children
          ) : content ? (
            <MarkdownRenderer content={content} />
          ) : (
            <div className="space-y-4">
              {guide.steps?.map((step, i) => (
                <div key={i}>
                  <div className="flex items-center gap-3 mb-2">
                    <span className="flex items-center justify-center w-6 h-6 rounded-full bg-gradient-to-br from-primary/80 to-primary text-primary-foreground text-sm font-bold shrink-0">
                      {i + 1}
                    </span>
                    <h4 className="text-sm font-semibold text-foreground">{step.title}</h4>
                  </div>
                  {step.description && (
                    <p className="text-sm text-muted-foreground ml-9 mb-2">{step.description}</p>
                  )}
                  {step.text && (
                    <pre className="ml-9 text-sm text-foreground whitespace-pre-wrap leading-relaxed font-sans overflow-x-auto rounded-lg border border-border bg-background/50 p-3">
                      {step.text}
                    </pre>
                  )}
                  {step.code && (
                    <div className="ml-9 rounded-lg overflow-hidden border border-primary/20">
                      <div className="px-3 py-1.5 border-b border-primary/10 bg-primary/5 flex items-center gap-1.5">
                        <Terminal className="w-3 h-3 text-primary" />
                        <span className="text-sm text-primary uppercase tracking-wider font-semibold">Terminal</span>
                      </div>
                      <pre className="p-3 text-sm font-mono text-foreground whitespace-pre-wrap overflow-x-auto leading-relaxed bg-muted/50">
                        {step.code}
                      </pre>
                    </div>
                  )}
                  {step.note && (
                    <p className="text-sm ml-9 mt-2 flex items-center gap-1.5 text-emerald-600 dark:text-emerald-400">
                      <CheckCircle className="w-3 h-3" />
                      {step.note}
                    </p>
                  )}
                </div>
              ))}
              {guide.result && (
                <div className="ml-9">
                  <p className="text-sm text-muted-foreground mb-2 font-medium">Expected output:</p>
                  <div className="rounded-lg overflow-hidden border border-emerald-600/20">
                    <div className="px-3 py-1.5 border-b border-emerald-600/10 bg-emerald-600/5">
                      <span className="text-sm text-emerald-600 dark:text-emerald-400 uppercase tracking-wider font-semibold">
                        Output
                      </span>
                    </div>
                    <pre className="p-3 text-sm font-mono text-emerald-600 dark:text-emerald-400 whitespace-pre-wrap overflow-x-auto bg-muted/50">
                      {guide.result}
                    </pre>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
