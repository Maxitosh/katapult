// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import type { Agent } from "@/api/types"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { AgentHealthIndicator } from "@/components/agents/agent-health-indicator"

interface AgentCardProps {
  agent: Agent
  onClick: () => void
}

function formatRelativeTime(dateString: string): string {
  const now = Date.now()
  const then = new Date(dateString).getTime()
  const diffSeconds = Math.floor((now - then) / 1000)

  if (diffSeconds < 60) return `${diffSeconds}s ago`
  const diffMinutes = Math.floor(diffSeconds / 60)
  if (diffMinutes < 60) return `${diffMinutes}m ago`
  const diffHours = Math.floor(diffMinutes / 60)
  if (diffHours < 24) return `${diffHours}h ago`
  const diffDays = Math.floor(diffHours / 24)
  return `${diffDays}d ago`
}

function formatTools(tools: Agent["tools"]): string {
  const available = Object.entries(tools)
    .filter(([, v]) => v)
    .map(([k]) => k)
  return available.length > 0 ? available.join(", ") : "none"
}

export function AgentCard({ agent, onClick }: AgentCardProps) {
  return (
    <Card
      size="sm"
      className="cursor-pointer transition-shadow hover:ring-2 hover:ring-primary/30"
      onClick={onClick}
      role="button"
      tabIndex={0}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault()
          onClick()
        }
      }}
    >
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span className="truncate">{agent.node_name}</span>
          <AgentHealthIndicator state={agent.state} healthy={agent.healthy} />
        </CardTitle>
      </CardHeader>
      <CardContent className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-muted-foreground">
        <span>Last heartbeat</span>
        <span className="text-right">{formatRelativeTime(agent.last_heartbeat)}</span>
        <span>Tools</span>
        <span className="text-right">{formatTools(agent.tools)}</span>
        <span>PVCs</span>
        <span className="text-right">{agent.pvcs.length}</span>
      </CardContent>
    </Card>
  )
}
