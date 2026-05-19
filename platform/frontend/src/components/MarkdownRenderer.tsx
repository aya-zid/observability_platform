import { isValidElement } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { Terminal } from "lucide-react"

export default function MarkdownRenderer({ content }: { content: string }) {
  return (
    <div className="space-y-3">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h2: ({ children }) => (
            <h2 className="text-sm font-bold text-foreground mt-5 mb-3 pb-2 border-b border-border">
              {children}
            </h2>
          ),
          h3: ({ children }) => (
            <h3 className="text-xs font-semibold text-foreground mt-4 mb-2">
              {children}
            </h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-xs font-semibold text-foreground mt-3 mb-1.5">
              {children}
            </h4>
          ),
          p: ({ children }) => (
            <p className="text-xs text-foreground leading-relaxed mb-3 last:mb-0">
              {children}
            </p>
          ),
          strong: ({ children }) => (
            <strong className="font-semibold text-foreground">{children}</strong>
          ),
          ol: ({ children }) => (
            <ol className="list-decimal pl-5 my-2 space-y-1 text-xs text-foreground">{children}</ol>
          ),
          ul: ({ children }) => (
            <ul className="list-disc pl-5 my-2 space-y-1 text-xs text-foreground">{children}</ul>
          ),
          li: ({ children }) => (
            <li className="leading-relaxed">{children}</li>
          ),
          blockquote: ({ children }) => (
            <blockquote className="border-l-2 border-primary/30 pl-4 py-1 my-3 text-xs text-muted-foreground">
              {children}
            </blockquote>
          ),
          code({ className, children }) {
            const isBlock = className?.startsWith("language-")
            if (isBlock) {
              const lang = className!.replace("language-", "")
              return (
                <div className="rounded-lg overflow-hidden border border-primary/20 my-3">
                  <div className="px-3 py-1.5 border-b border-primary/10 bg-primary/5 flex items-center gap-1.5">
                    <Terminal className="w-3 h-3 text-primary" />
                    <span className="text-[10px] text-primary uppercase tracking-wider font-semibold">
                      {lang}
                    </span>
                  </div>
                  <pre className="p-3 text-xs font-mono text-foreground whitespace-pre-wrap overflow-x-auto leading-relaxed bg-[hsl(240,15%,5%)]">
                    {children}
                  </pre>
                </div>
              )
            }
            return (
              <code className="rounded bg-muted px-1.5 py-0.5 text-xs font-mono text-foreground border border-border/50">
                {children}
              </code>
            )
          },
          pre({ children }) {
            if (isValidElement(children) && children.type === "code") {
              return (
                <div className="rounded-lg overflow-hidden border border-primary/20 my-3">
                  <pre className="p-3 text-xs font-mono text-foreground whitespace-pre-wrap overflow-x-auto leading-relaxed bg-[hsl(240,15%,5%)]">
                    {children}
                  </pre>
                </div>
              )
            }
            return <>{children}</>
          },
          hr: () => <hr className="my-4 border-border" />,
          a: ({ children, href }) => (
            <a
              href={href}
              className="text-primary underline underline-offset-2 hover:text-primary/80"
              target="_blank"
              rel="noreferrer"
            >
              {children}
            </a>
          ),
          table: ({ children }) => (
            <div className="my-3 overflow-x-auto rounded-lg border border-border">
              <table className="w-full text-xs">{children}</table>
            </div>
          ),
          thead: ({ children }) => (
            <thead className="bg-muted/50 border-b border-border">{children}</thead>
          ),
          th: ({ children }) => (
            <th className="px-3 py-2 text-left font-semibold text-foreground">{children}</th>
          ),
          td: ({ children }) => (
            <td className="px-3 py-2 text-muted-foreground border-t border-border/50">{children}</td>
          ),
          input({ ...props }) {
            return (
              <input
                {...props}
                className="h-3.5 w-3.5 rounded border-border accent-primary"
              />
            )
          },
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}
