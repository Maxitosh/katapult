// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import type { PVCInfo } from "@/api/types"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"

interface PVCTableProps {
  pvcs: PVCInfo[]
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const k = 1024
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  const value = bytes / Math.pow(k, i)
  return `${value.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

// @cpt-begin:cpt-katapult-flow-web-ui-browse-agents:p2:inst-render-pvcs
export function PVCTable({ pvcs }: PVCTableProps) {
  if (pvcs.length === 0) {
    return (
      <p className="py-4 text-center text-sm text-muted-foreground">
        No PVCs found on this agent.
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>PVC Name</TableHead>
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
            <TableCell>{pvc.node_affinity || "\u2014"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
// @cpt-end:cpt-katapult-flow-web-ui-browse-agents:p2:inst-render-pvcs
