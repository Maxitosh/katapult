// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import type { Agent } from "@/api/types"
import { AgentCard } from "@/components/agents/agent-card"

interface AgentClusterGroupProps {
  cluster: string
  agents: Agent[]
  onSelectAgent: (id: string) => void
}

export function AgentClusterGroup({ cluster, agents, onSelectAgent }: AgentClusterGroupProps) {
  return (
    <section className="rounded-xl border border-border bg-card/50 p-4">
      <h3 className="mb-3 text-sm font-semibold text-foreground">{cluster}</h3>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {agents.map((agent) => (
          <AgentCard
            key={agent.id}
            agent={agent}
            onClick={() => onSelectAgent(agent.id)}
          />
        ))}
      </div>
    </section>
  )
}
