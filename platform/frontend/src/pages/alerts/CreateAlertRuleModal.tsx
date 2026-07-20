import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { useState } from "react"

import { getAlertTemplatesByService, createAlertRule, type AlertTemplate, type ScopeInput, type ThresholdInput } from "@/api/alerts"
import { getDiscoveryHosts, getDiscoveryServices, type DiscoveryHost, type DiscoveryService } from "@/api/discovery"
import { getNginxDomains } from "@/api/nginx"
import { getApacheDomains } from "@/api/apache"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const serviceTypes = [
  { key: "host", label: "Host" },
  { key: "nginx", label: "Nginx" },
  { key: "apache", label: "Apache" },
  { key: "postgres", label: "PostgreSQL" },
  { key: "mariadb", label: "MariaDB" },
  { key: "phpfpm", label: "PHP-FPM" },
  { key: "redis", label: "Redis" },
]

type Step = "pick-service" | "pick-template" | "configure"

export default function CreateAlertRuleModal({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState<Step>("pick-service")
  const [serviceType, setServiceType] = useState("")
  const [selectedTemplate, setSelectedTemplate] = useState<AlertTemplate | null>(null)

  const [selectedHostIds, setSelectedHostIds] = useState<number[]>([])
  const [selectedServiceIds, setSelectedServiceIds] = useState<number[]>([])
  const [selectedDomain, setSelectedDomain] = useState("")
  const [thresholds, setThresholds] = useState<ThresholdInput[]>([{ label: "", threshold: 0, severity: "warning" }])
  const [forDuration, setForDuration] = useState("5m")
  const [showHostPicker, setShowHostPicker] = useState(false)
  const [showServicePicker, setShowServicePicker] = useState(false)

  const hostsQuery = useQuery({
    queryKey: ["discovery-hosts"],
    queryFn: getDiscoveryHosts,
  })

  const servicesQuery = useQuery({
    queryKey: ["discovery-services"],
    queryFn: () => getDiscoveryServices(),
  })

  const templatesQuery = useQuery({
    queryKey: ["alert-templates", serviceType],
    queryFn: () => getAlertTemplatesByService(serviceType),
    enabled: !!serviceType,
  })

  const filteredServices = servicesQuery.data?.filter(
    (s) => s.service_name === serviceType
  ) ?? []

  const isWebService = serviceType === "nginx" || serviceType === "apache"

  const domainsQuery = useQuery({
    queryKey: [serviceType === "nginx" ? "nginx-domains" : "apache-domains"],
    queryFn: () => {
      const id = filteredServices.length > 0 ? String(filteredServices[0].id) : ""
      if (!id) return Promise.resolve([])
      return serviceType === "nginx" ? getNginxDomains(id) : getApacheDomains(id)
    },
    enabled: isWebService && filteredServices.length > 0,
  })

  const createMutation = useMutation({
    mutationFn: () => {
      const scopes: ScopeInput[] = []
      if (selectedHostIds.length > 0) {
        selectedHostIds.forEach((id) => scopes.push({ scope_type: "host", scope_id: id }))
      } else {
        scopes.push({ scope_type: "all_hosts" })
      }
      if (selectedServiceIds.length > 0) {
        selectedServiceIds.forEach((id) => scopes.push({ scope_type: "service", scope_id: id }))
      } else if (serviceType !== "host") {
        scopes.push({ scope_type: "all_services_of_type" })
      }

      return createAlertRule({
        template_id: selectedTemplate!.id,
        domain: selectedDomain || undefined,
        scopes,
        thresholds,
        for_duration: forDuration,
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["alert-rules"] })
      onClose()
    },
    onError: (err) => {
      console.error("[CreateAlertRule] API error:", err)
    },
  })

  function handlePickService(key: string) {
    setServiceType(key)
    setStep("pick-template")
  }

  function handlePickTemplate(tpl: AlertTemplate) {
    setSelectedTemplate(tpl)
    setThresholds([{ label: "", threshold: tpl.default_threshold, severity: tpl.default_severity }])
    setForDuration(tpl.default_for_duration)
    setStep("configure")
  }

  function toggleHost(hostId: number) {
    setSelectedHostIds((prev) =>
      prev.includes(hostId) ? prev.filter((id) => id !== hostId) : [...prev, hostId]
    )
  }

  function toggleService(serviceId: number) {
    setSelectedServiceIds((prev) =>
      prev.includes(serviceId) ? prev.filter((id) => id !== serviceId) : [...prev, serviceId]
    )
  }

  function updateThreshold(index: number, field: keyof ThresholdInput, value: string | number) {
    setThresholds((prev) => {
      const copy = [...prev]
      copy[index] = { ...copy[index], [field]: value }
      return copy
    })
  }

  function addThreshold() {
    setThresholds((prev) => [
      ...prev,
      { label: "", threshold: selectedTemplate?.default_threshold ?? 0, severity: "warning" },
    ])
  }

  function removeThreshold(index: number) {
    setThresholds((prev) => prev.filter((_, i) => i !== index))
  }

  const selectedHosts = hostsQuery.data?.filter((h) => selectedHostIds.includes(h.id)) ?? []
  const selectedServices = filteredServices.filter((s) => selectedServiceIds.includes(s.id))

  if (step === "pick-service") {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-lg">
          <h3 className="font-semibold text-lg mb-4">Select Service Type</h3>
          <div className="grid grid-cols-2 gap-3">
            {serviceTypes.map((st) => (
              <button
                key={st.key}
                onClick={() => handlePickService(st.key)}
                className="rounded-xl border border-border p-4 text-left hover:bg-muted transition-colors"
              >
                <div className="font-medium">{st.label}</div>
              </button>
            ))}
          </div>
          <div className="mt-4 flex justify-end">
            <Button variant="outline" onClick={onClose}>Cancel</Button>
          </div>
        </div>
      </div>
    )
  }

  if (step === "pick-template") {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-lg max-h-[80vh] overflow-y-auto">
          <h3 className="font-semibold text-lg mb-4">Select Alert Template</h3>
          {templatesQuery.isLoading ? (
            <div className="text-sm text-muted-foreground">Loading templates...</div>
          ) : (
            <div className="grid gap-3">
              {templatesQuery.data?.map((tpl) => (
                <button
                  key={tpl.id}
                  onClick={() => handlePickTemplate(tpl)}
                  className="rounded-xl border border-border p-4 text-left hover:bg-muted transition-colors"
                >
                  <div className="font-medium">{tpl.name}</div>
                  <div className="text-sm text-muted-foreground mt-1">{tpl.description}</div>
                  <div className="text-xs text-muted-foreground mt-1">
                    Default threshold: {tpl.default_threshold}{tpl.unit}
                  </div>
                </button>
              ))}
            </div>
          )}
          <div className="mt-4 flex gap-2 justify-end">
            <Button variant="outline" onClick={() => setStep("pick-service")}>Back</Button>
            <Button variant="outline" onClick={onClose}>Cancel</Button>
          </div>
        </div>
      </div>
    )
  }

  if (step === "configure" && selectedTemplate) {
    return (
      <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
        <div className="bg-card rounded-2xl border border-border p-6 w-full max-w-xl max-h-[90vh] overflow-y-auto">
          <h3 className="font-semibold text-lg mb-1">Configure Alert</h3>
          <p className="text-sm text-muted-foreground mb-4">
            {selectedTemplate.name} &middot; {selectedTemplate.description}
          </p>

          <div className="grid gap-5">
            {/* Hosts */}
            <div>
              <label className="text-sm font-medium text-muted-foreground">Hosts</label>
              <div className="mt-1.5">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowHostPicker(!showHostPicker)}
                >
                  {selectedHostIds.length === 0
                    ? "All Hosts (click to select specific)"
                    : `Selected ${selectedHostIds.length} host(s)`}
                </Button>
                {showHostPicker && (
                  <div className="mt-2 border border-border rounded-xl p-3 max-h-48 overflow-y-auto bg-background">
                    {hostsQuery.isLoading ? (
                      <div className="text-sm text-muted-foreground">Loading hosts...</div>
                    ) : (
                      <div className="grid gap-1">
                        <label className="flex items-center gap-2 text-sm cursor-pointer">
                          <input
                            type="checkbox"
                            checked={selectedHostIds.length === 0}
                            onChange={() => setSelectedHostIds([])}
                          />
                          All Hosts
                        </label>
                        {hostsQuery.data?.map((host) => (
                          <label
                            key={host.id}
                            className="flex items-center gap-2 text-sm cursor-pointer pl-4"
                          >
                            <input
                              type="checkbox"
                              checked={selectedHostIds.includes(host.id)}
                              onChange={() => toggleHost(host.id)}
                            />
                            {host.hostname} ({host.machine_id})
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                )}
                {selectedHostIds.length > 0 && (
                  <div className="mt-2 flex flex-wrap gap-1">
                    {selectedHosts.map((host) => (
                      <span
                        key={host.id}
                        className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted text-xs"
                      >
                        {host.hostname}
                        <button onClick={() => toggleHost(host.id)} className="text-muted-foreground hover:text-foreground">&times;</button>
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Services (not for host service type) */}
            {serviceType !== "host" && (
              <div>
                <label className="text-sm font-medium text-muted-foreground">Services</label>
                <div className="mt-1.5">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setShowServicePicker(!showServicePicker)}
                  >
                    {selectedServiceIds.length === 0
                      ? `All ${selectedTemplate.name} services (click to select specific)`
                      : `Selected ${selectedServiceIds.length} service(s)`}
                  </Button>
                  {showServicePicker && (
                    <div className="mt-2 border border-border rounded-xl p-3 max-h-48 overflow-y-auto bg-background">
                      {servicesQuery.isLoading ? (
                        <div className="text-sm text-muted-foreground">Loading services...</div>
                      ) : filteredServices.length === 0 ? (
                        <div className="text-sm text-muted-foreground">No {serviceType} services found.</div>
                      ) : (
                        <div className="grid gap-1">
                          <label className="flex items-center gap-2 text-sm cursor-pointer">
                            <input
                              type="checkbox"
                              checked={selectedServiceIds.length === 0}
                              onChange={() => setSelectedServiceIds([])}
                            />
                            All {serviceType} Services
                          </label>
                          {filteredServices.map((svc) => (
                            <label
                              key={svc.id}
                              className="flex items-center gap-2 text-sm cursor-pointer pl-4"
                            >
                              <input
                                type="checkbox"
                                checked={selectedServiceIds.includes(svc.id)}
                                onChange={() => toggleService(svc.id)}
                              />
                              {svc.service_name} ({svc.machine_id})
                            </label>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                  {selectedServiceIds.length > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {selectedServices.map((svc) => (
                        <span
                          key={svc.id}
                          className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md bg-muted text-xs"
                        >
                          {svc.service_name}
                          <button onClick={() => toggleService(svc.id)} className="text-muted-foreground hover:text-foreground">&times;</button>
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            )}

            {/* Duration */}
            <div>
              <label className="text-sm font-medium text-muted-foreground">For Duration</label>
              <select
                value={forDuration}
                onChange={(e) => setForDuration(e.target.value)}
                className="w-full border border-border rounded-lg px-3 py-2 bg-background mt-1"
              >
                <option value="1m">1 minute</option>
                <option value="5m">5 minutes</option>
                <option value="10m">10 minutes</option>
                <option value="15m">15 minutes</option>
                <option value="30m">30 minutes</option>
                <option value="1h">1 hour</option>
              </select>
            </div>

            {/* Domain (nginx / apache only) */}
            {isWebService && (
              <div>
                <label className="text-sm font-medium text-muted-foreground">Domain (optional)</label>
                <div className="mt-1.5">
                  <select
                    value={selectedDomain}
                    onChange={(e) => setSelectedDomain(e.target.value)}
                    className="w-full border border-border rounded-lg px-3 py-2 bg-background"
                  >
                    <option value="">All domains</option>
                    {domainsQuery.data?.map((d) => (
                      <option key={d.domain} value={d.domain}>
                        {d.domain} {d.enabled ? "" : "(disabled)"}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            {/* Thresholds */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-sm font-medium text-muted-foreground">Thresholds</label>
                <Button variant="outline" size="sm" onClick={addThreshold}>
                  + Add Threshold
                </Button>
              </div>
              <div className="grid gap-3">
                {thresholds.map((t, i) => (
                  <div key={i} className="border border-border rounded-xl p-3">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-xs text-muted-foreground">Threshold #{i + 1}</span>
                      {thresholds.length > 1 && (
                        <button
                          onClick={() => removeThreshold(i)}
                          className="text-xs text-destructive hover:underline"
                        >
                          Remove
                        </button>
                      )}
                    </div>
                    <div className="grid grid-cols-2 gap-2">
                      <div>
                        <label className="text-xs text-muted-foreground">Label</label>
                        <input
                          type="text"
                          value={t.label}
                          onChange={(e) => updateThreshold(i, "label", e.target.value)}
                          placeholder="e.g. High priority hosts"
                          className="w-full border border-border rounded-lg px-2 py-1.5 bg-background mt-0.5 text-sm"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-muted-foreground">
                          Threshold ({selectedTemplate.unit})
                        </label>
                        <input
                          type="number"
                          value={t.threshold}
                          onChange={(e) => updateThreshold(i, "threshold", parseFloat(e.target.value))}
                          className="w-full border border-border rounded-lg px-2 py-1.5 bg-background mt-0.5 text-sm"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-muted-foreground">Severity</label>
                        <select
                          value={t.severity}
                          onChange={(e) => updateThreshold(i, "severity", e.target.value)}
                          className="w-full border border-border rounded-lg px-2 py-1.5 bg-background mt-0.5 text-sm"
                        >
                          <option value="info">Info</option>
                          <option value="warning">Warning</option>
                          <option value="critical">Critical</option>
                        </select>
                      </div>
                      <div>
                        <label className="text-xs text-muted-foreground">For Duration</label>
                        <select
                          value={t.for_duration ?? forDuration}
                          onChange={(e) => updateThreshold(i, "for_duration", e.target.value)}
                          className="w-full border border-border rounded-lg px-2 py-1.5 bg-background mt-0.5 text-sm"
                        >
                          <option value="">Same as rule default</option>
                          <option value="1m">1 minute</option>
                          <option value="5m">5 minutes</option>
                          <option value="10m">10 minutes</option>
                          <option value="15m">15 minutes</option>
                          <option value="30m">30 minutes</option>
                          <option value="1h">1 hour</option>
                        </select>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          {createMutation.isError && (
            <div className="mt-3 p-3 rounded-xl bg-destructive/10 border border-destructive/20 text-sm text-destructive">
              {createMutation.error instanceof Error
                ? createMutation.error.message
                : "Failed to create rule. Check console for details."}
            </div>
          )}
          <div className="mt-6 flex gap-2 justify-end">
            <Button variant="outline" onClick={() => setStep("pick-template")}>Back</Button>
            <Button variant="outline" onClick={onClose}>Cancel</Button>
            <Button onClick={() => createMutation.mutate()} disabled={createMutation.isPending}>
              {createMutation.isPending ? "Creating..." : "Create Rule"}
            </Button>
          </div>
        </div>
      </div>
    )
  }

  return null
}
