// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1

import type { Transfer } from "@/api/types"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { TransferStatusBadge } from "@/components/transfers/transfer-status-badge"
import { TransferProgressBar } from "@/components/transfers/transfer-progress-bar"

interface TransferListProps {
  transfers: Transfer[]
  onSelect: (id: string) => void
}

export function TransferList({ transfers, onSelect }: TransferListProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Source</TableHead>
          <TableHead>Destination</TableHead>
          <TableHead>Strategy</TableHead>
          <TableHead>State</TableHead>
          <TableHead className="min-w-[160px]">Progress</TableHead>
          <TableHead>Created</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {transfers.map((transfer) => (
          <TableRow
            key={transfer.id}
            className="cursor-pointer"
            onClick={() => onSelect(transfer.id)}
          >
            <TableCell className="font-mono text-xs">
              {transfer.id.slice(0, 8)}
            </TableCell>
            <TableCell>
              <div className="flex flex-col">
                <span className="text-xs font-medium">{transfer.source_cluster}</span>
                <span className="text-xs text-muted-foreground">{transfer.source_pvc}</span>
              </div>
            </TableCell>
            <TableCell>
              <div className="flex flex-col">
                <span className="text-xs font-medium">{transfer.destination_cluster}</span>
                <span className="text-xs text-muted-foreground">{transfer.destination_pvc}</span>
              </div>
            </TableCell>
            <TableCell className="capitalize">{transfer.strategy}</TableCell>
            <TableCell>
              <TransferStatusBadge state={transfer.state} />
            </TableCell>
            <TableCell>
              <TransferProgressBar
                bytesTransferred={transfer.bytes_transferred}
                bytesTotal={transfer.bytes_total}
              />
            </TableCell>
            <TableCell className="text-xs text-muted-foreground">
              {new Date(transfer.created_at).toLocaleString()}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
