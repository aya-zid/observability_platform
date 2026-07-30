import { isValidElement } from "react"
import ReactMarkdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { Terminal } from "lucide-react"

export default function MarkdownRenderer({ content }: { content: string }) {
  return (
    <div className="space-y-4">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          h2: ({ children }) => {
            const id = String(children).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/(^-|-$)/g, "")
            return (
              <h2 id={id} className="text-lg font-bold text-foreground mt-8 mb-4 pl-4 border-l-[3px] border-primary">
                {children}
              </h2>
            )
          },
          h3: ({ children }) => (
            <h3 className="text-base font-bold text-primary mt-7 mb-3 flex items-center gap-2">
              <span className="w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              {children}
            </h3>
          ),
          h4: ({ children }) => (
            <h4 className="text-sm font-bold text-foreground/80 mt-5 mb-2 uppercase tracking-wider">
              {children}
            </h4>
          ),
          p: ({ children }) => (
            <p className="text-sm text-foreground leading-relaxed mb-3 last:mb-0">
              {children}
            </p>
          ),
          strong: ({ children }) => (
            <strong className="font-semibold text-foreground">{children}</strong>
          ),
          ol: ({ children }) => (
            <ol className="list-decimal pl-5 my-3 space-y-1.5 text-sm text-foreground">{children}</ol>
          ),
          ul: ({ children }) => (
            <ul className="list-disc pl-5 my-3 space-y-1.5 text-sm text-foreground">{children}</ul>
          ),
          li: ({ children }) => (
            <li className="leading-relaxed">{children}</li>
          ),
          blockquote: ({ children }) => (
            <blockquote className="border-l-2 border-primary/30 pl-4 py-2 my-4 text-sm text-muted-foreground bg-muted/30 rounded-r-lg">
              {children}
            </blockquote>
          ),
          code({ className, children }) {
            const isBlock = className?.startsWith("language-")
            if (isBlock) {
              const lang = className!.replace("language-", "")
              return (
                <div className="rounded-lg overflow-hidden border border-border my-4">
                  <div className="px-3 py-1.5 border-b border-border bg-muted flex items-center gap-1.5">
                    <Terminal className="w-3.5 h-3.5 text-muted-foreground" />
                    <span className="text-sm text-muted-foreground uppercase tracking-wider font-semibold">
                      {lang}
                    </span>
                  </div>
                  <pre className="p-4 text-sm font-mono text-foreground whitespace-pre-wrap overflow-x-auto leading-relaxed bg-muted/50">
                    {children}
                  </pre>
                </div>
              )
            }
            return (
              <code className="rounded bg-muted px-1.5 py-0.5 text-sm font-mono text-foreground border border-border/50">
                {children}
              </code>
            )
          },
          pre({ children }) {
            if (isValidElement(children) && children.type === "code") {
              return (
                <div className="rounded-lg overflow-hidden border border-border my-4">
                  <pre className="p-4 text-sm font-mono text-foreground whitespace-pre-wrap overflow-x-auto leading-relaxed bg-muted/50">
                    {children}
                  </pre>
                </div>
              )
            }
            return <>{children}</>
          },
          hr: () => <hr className="my-6 border-border" />,
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
            <div className="my-4 overflow-x-auto rounded-lg border border-border">
              <table className="w-full text-sm">{children}</table>
            </div>
          ),
          thead: ({ children }) => (
            <thead className="bg-muted border-b border-border">{children}</thead>
          ),
          th: ({ children }) => (
            <th className="px-4 py-2.5 text-left font-semibold text-foreground text-sm uppercase tracking-wider">
              {children}
            </th>
          ),
          td: ({ children }) => (
            <td className="px-4 py-2.5 text-sm text-muted-foreground border-t border-border/50">
              {children}
            </td>
          ),
          input({ ...props }) {
            return (
              <input
                {...props}
                className="h-4 w-4 rounded border-border accent-primary"
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
