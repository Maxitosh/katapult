// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2

import type { AgentState } from "@/api/types"
import { cn } from "@/lib/utils"

interface AgentHealthIndicatorProps {
  state: AgentState
  healthy: boolean
}

const stateConfig: Record<
  AgentState,
  { label: string; dotClass: string }
> = {
  healthy: {
    label: "Healthy",
    dotClass: "bg-green-500",
  },
  unhealthy: {
    label: "Unhealthy",
    dotClass: "bg-yellow-500",
  },
  disconnected: {
    label: "Disconnected",
    dotClass: "bg-red-500",
  },
  registering: {
    label: "Registering",
    dotClass: "bg-gray-400 animate-pulse",
  },
}

function getIndicatorConfig(state: AgentState, healthy: boolean) {
  if (state === "healthy" && !healthy) {
    return stateConfig.unhealthy
  }
  return stateConfig[state]
}

export function AgentHealthIndicator({ state, healthy }: AgentHealthIndicatorProps) {
  const config = getIndicatorConfig(state, healthy)

  return (
    <span className="inline-flex items-center gap-1.5 text-xs">
      <span
        className={cn("inline-block size-2 rounded-full", config.dotClass)}
        aria-hidden="true"
      />
      <span className="text-muted-foreground">{config.label}</span>
    </span>
  )
}
