// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import { useMemo } from "react"
import { useNavigate } from "react-router-dom"
import type { Agent } from "@/api/types"
import { useAgents, useClusters } from "@/hooks/use-agents"
import { AgentClusterGroup } from "@/components/agents/agent-cluster-group"
import { LoadingSpinner } from "@/components/shared/loading-spinner"
import { ErrorAlert } from "@/components/shared/error-alert"

export function AgentsPage() {
  const navigate = useNavigate()

  // @cpt-begin:cpt-katapult-flow-web-ui-browse-agents:p2:inst-fetch-agents
  const { data: agentsData, isLoading, error, refetch } = useAgents()
  const { data: clusters } = useClusters()
  // @cpt-end:cpt-katapult-flow-web-ui-browse-agents:p2:inst-fetch-agents

  const groupedAgents = useMemo(() => {
    if (!agentsData?.items) return new Map<string, Agent[]>()
    const groups = new Map<string, Agent[]>()
    for (const agent of agentsData.items) {
      const list = groups.get(agent.cluster_id) ?? []
      list.push(agent)
      groups.set(agent.cluster_id, list)
    }
    return groups
  }, [agentsData])

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
    )
  }

  if (error) {
    return (
      <ErrorAlert
        message={error instanceof Error ? error.message : "Failed to load agents"}
        onRetry={() => refetch()}
      />
    )
  }

  if (!agentsData || agentsData.items.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-2 py-12">
        <p className="text-sm font-medium text-foreground">No agents registered</p>
        <p className="text-xs text-muted-foreground">
          Agents will appear here once they connect to the control plane.
        </p>
      </div>
    )
  }

  const clusterOrder = clusters ?? [...groupedAgents.keys()]

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-lg font-semibold">Agents</h1>
      <div className="flex flex-col gap-4">
        {clusterOrder.map((cluster) => {
          const agents = groupedAgents.get(cluster)
          if (!agents || agents.length === 0) return null
          return (
            <AgentClusterGroup
              key={cluster}
              cluster={cluster}
              agents={agents}
              onSelectAgent={(id) => navigate(`/agents/${id}`)}
            />
          )
        })}
      </div>
    </div>
  )
}
