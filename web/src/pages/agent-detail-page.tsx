// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import { useParams, useNavigate } from "react-router-dom"
import { useAgent, useAgentPVCs } from "@/hooks/use-agents"
import { AgentHealthIndicator } from "@/components/agents/agent-health-indicator"
import { LoadingSpinner } from "@/components/shared/loading-spinner"
import { ErrorAlert } from "@/components/shared/error-alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const size = bytes / Math.pow(1024, i)
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatTools(tools: { tar: boolean; zstd: boolean; stunnel: boolean }): string {
  const available = Object.entries(tools)
    .filter(([, v]) => v)
    .map(([k]) => k)
  return available.length > 0 ? available.join(", ") : "none"
}

export function AgentDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const { data: agent, isLoading, error, refetch } = useAgent(id ?? "")

  // @cpt-begin:cpt-katapult-flow-web-ui-browse-agents:p2:inst-fetch-agent-pvcs
  const { data: pvcs } = useAgentPVCs(id ?? "")
  // @cpt-end:cpt-katapult-flow-web-ui-browse-agents:p2:inst-fetch-agent-pvcs

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
    )
  }

  if (error || !agent) {
    return (
      <ErrorAlert
        message={error instanceof Error ? error.message : "Failed to load agent"}
        onRetry={() => refetch()}
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <Button
        variant="outline"
        size="sm"
        className="self-start"
        onClick={() => navigate("/agents")}
      >
        Back to Agents
      </Button>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between">
            <span>{agent.node_name}</span>
            <AgentHealthIndicator state={agent.state} healthy={agent.healthy} />
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
            <div>
              <span className="text-xs text-muted-foreground">Cluster</span>
              <p className="font-medium">{agent.cluster_id}</p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">State</span>
              <p className="capitalize font-medium">{agent.state}</p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Last Heartbeat</span>
              <p className="font-medium">
                {new Date(agent.last_heartbeat).toLocaleString()}
              </p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Registered At</span>
              <p className="font-medium">
                {new Date(agent.registered_at).toLocaleString()}
              </p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Tools</span>
              <p className="font-medium">{formatTools(agent.tools)}</p>
            </div>
            <div>
              <span className="text-xs text-muted-foreground">Health</span>
              <p className="font-medium">{agent.healthy ? "Healthy" : "Unhealthy"}</p>
            </div>
          </div>
        </CardContent>
      </Card>

      <div>
        <h2 className="mb-3 text-sm font-semibold">PVCs</h2>
        {pvcs && pvcs.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Size</TableHead>
                <TableHead>Storage Class</TableHead>
                <TableHead>Node Affinity</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {pvcs.map((pvc) => (
                <TableRow key={pvc.pvc_name}>
                  <TableCell className="font-medium">{pvc.pvc_name}</TableCell>
                  <TableCell>{formatBytes(pvc.size_bytes)}</TableCell>
                  <TableCell>{pvc.storage_class}</TableCell>
                  <TableCell>{pvc.node_affinity || "-"}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ) : (
          <p className="text-sm text-muted-foreground">No PVCs found for this agent.</p>
        )}
      </div>
    </div>
  )
}
