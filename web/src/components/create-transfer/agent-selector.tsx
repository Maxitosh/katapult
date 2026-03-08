// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import type { Agent } from "@/api/types"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { cn } from "@/lib/utils"

interface AgentSelectorProps {
  agents: Agent[]
  value: string
  onChange: (agentId: string) => void
  label: string
  disabled?: boolean
}

export function AgentSelector({
  agents,
  value,
  onChange,
  label,
  disabled = false,
}: AgentSelectorProps) {
  const isDisabled = disabled || agents.length === 0

  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium text-foreground">{label}</label>
      <Select value={value} onValueChange={(val) => { if (val) onChange(val) }} disabled={isDisabled}>
        <SelectTrigger className="w-full">
          <SelectValue
            placeholder={
              agents.length === 0 ? "No agents available" : "Select agent..."
            }
          />
        </SelectTrigger>
        <SelectContent>
          {agents.map((agent) => (
            <SelectItem key={agent.id} value={agent.id}>
              <span className="flex items-center gap-2">
                <span
                  className={cn(
                    "inline-block size-2 rounded-full",
                    agent.healthy
                      ? "bg-emerald-500"
                      : "bg-destructive",
                  )}
                  aria-label={agent.healthy ? "Healthy" : "Unhealthy"}
                />
                {agent.node_name}
              </span>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
