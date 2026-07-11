import WebServerVirtualHostsSection, { type VirtualHostEntry } from "@/pages/service/components/WebServerVirtualHostsSection"

type Props = {
  hosts: VirtualHostEntry[]
  onChange: (hosts: VirtualHostEntry[]) => void
}

export type { VirtualHostEntry }

export default function ApacheVirtualHostsSection(props: Props) {
  return <WebServerVirtualHostsSection {...props} service="apache" />
}
