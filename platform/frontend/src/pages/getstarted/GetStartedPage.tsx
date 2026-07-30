import { useState } from "react"
import { Terminal, Copy, Check, Rocket, Server, Box, Activity, Globe, Database, Zap, Mail, Cpu } from "lucide-react"
import { useAuthStore } from "@/store/authStore"

const steps = [
  { icon: Terminal, title: "Install the Agent", desc: "Run the install command on your host to deploy the observability agent." },
  { icon: Server, title: "Auto-Discovery", desc: "The agent detects services, containers, and their log paths automatically." },
  { icon: Activity, title: "Start Monitoring", desc: "Configure collection, set alerts, and view dashboards in real-time." },
]

export default function GetStartedPage() {
  const [copied, setCopied] = useState(false)
  const apiKey = useAuthStore((s) => s.apiKey)

  const cmd = `curl -sSL https://platform.example.com/install.sh | API_KEY=${apiKey || "YOUR_API_KEY"} bash`

  const handleCopy = () => {
    navigator.clipboard.writeText(cmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="flex-1 overflow-y-auto">
      <header className="border-b border-border bg-card/50 backdrop-blur-sm px-6 py-3 shrink-0">
        <h1 className="text-lg font-bold text-foreground flex items-center gap-2">
          <Rocket className="w-4 h-4 text-primary" /> Get Started
        </h1>
        <p className="text-xs text-muted-foreground mt-0.5">Welcome! Install the agent to start monitoring your infrastructure.</p>
      </header>
      <main className="max-w-3xl mx-auto p-4 md:p-6 space-y-5">
        <div className="bg-card border border-primary/20 rounded-xl p-5 shadow-accent-glow animate-fade-in text-center">
          <div className="w-12 h-12 rounded-full bg-gradient-to-br from-primary to-primary/60 flex items-center justify-center mx-auto mb-3">
            <Rocket className="w-6 h-6 text-primary-foreground" />
          </div>
          <h2 className="text-base font-bold text-foreground mb-1">Welcome to Observability Platform</h2>
          <p className="text-xs text-muted-foreground max-w-md mx-auto">
            Get full observability in minutes. Install our lightweight agent on your hosts and we'll handle the rest.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {steps.map((step, i) => (
            <div key={i} className="bg-card border border-border rounded-xl p-3.5 animate-fade-in">
              <div className="w-7 h-7 rounded-lg bg-primary/10 flex items-center justify-center mb-2">
                <step.icon className="w-3.5 h-3.5 text-primary" />
              </div>
              <div className="text-xs text-primary font-semibold mb-0.5">Step {i + 1}</div>
              <h3 className="text-xs font-semibold text-foreground mb-0.5">{step.title}</h3>
              <p className="text-xs text-muted-foreground">{step.desc}</p>
            </div>
          ))}
        </div>

        <div className="bg-card border border-primary/20 rounded-xl p-4 shadow-accent-glow animate-fade-in">
          <h2 className="text-xs font-semibold text-foreground mb-1 flex items-center gap-2">
            <Terminal className="w-3.5 h-3.5 text-primary" /> Install Command
          </h2>
          <p className="text-xs text-muted-foreground mb-3">
            Copy and run this on any host you want to monitor:
          </p>
          <div className="relative">
            <pre className="bg-[hsl(240,15%,5%)] rounded-lg p-3 text-xs text-emerald-400 font-mono whitespace-pre-wrap break-all overflow-x-auto border border-border/50 leading-relaxed">
              {cmd}
            </pre>
            <button
              onClick={handleCopy}
              className="absolute top-1.5 right-1.5 p-1 rounded-md bg-card/80 text-muted-foreground hover:text-foreground transition-colors"
              title="Copy"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>

        <div className="bg-card border border-border rounded-xl p-4 animate-fade-in">
          <h2 className="text-xs font-semibold text-foreground mb-2.5">What the agent detects</h2>
          <div className="grid grid-cols-2 gap-2">
            {[
              { icon: Globe, label: "Web Servers", desc: "Nginx, Apache, Caddy, Traefik" },
              { icon: Database, label: "Databases", desc: "PostgreSQL, MySQL, MariaDB, MongoDB" },
              { icon: Zap, label: "Cache", desc: "Redis, Memcached" },
              { icon: Mail, label: "Message Queues", desc: "RabbitMQ, Kafka" },
              { icon: Cpu, label: "App Servers", desc: "PHP-FPM, Node.js, Gunicorn" },
              { icon: Box, label: "Containers", desc: "Docker containers & services" },
            ].map((item) => (
              <div key={item.label} className="flex items-center gap-2.5 bg-secondary rounded-lg p-2.5">
                <div className="w-7 h-7 rounded-md bg-primary/10 flex items-center justify-center shrink-0">
                  <item.icon className="w-3.5 h-3.5 text-primary" />
                </div>
                <div>
                  <span className="text-xs font-semibold text-foreground">{item.label}</span>
                  <p className="text-xs text-muted-foreground">{item.desc}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </main>
    </div>
  )
}
