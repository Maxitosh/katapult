// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1

import type { Transfer } from "@/api/types"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { TransferStatusBadge } from "@/components/transfers/transfer-status-badge"

interface TransferSummaryCardProps {
  transfer: Transfer
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between py-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-xs font-medium">{children}</span>
    </div>
  )
}

function formatTimestamp(value: string | null): string {
  if (!value) return "-"
  return new Date(value).toLocaleString()
}

export function TransferSummaryCard({ transfer }: TransferSummaryCardProps) {
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center justify-between">
          <span className="font-mono text-sm">{transfer.id}</span>
          <TransferStatusBadge state={transfer.state} />
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-x-6 gap-y-0 divide-x">
          <div className="flex flex-col">
            <DetailRow label="Source Cluster">{transfer.source_cluster}</DetailRow>
            <DetailRow label="Source PVC">{transfer.source_pvc}</DetailRow>
            <DetailRow label="Dest Cluster">{transfer.destination_cluster}</DetailRow>
            <DetailRow label="Dest PVC">{transfer.destination_pvc}</DetailRow>
            <DetailRow label="Strategy">
              <span className="capitalize">{transfer.strategy}</span>
            </DetailRow>
          </div>
          <div className="flex flex-col pl-6">
            <DetailRow label="Created By">{transfer.created_by}</DetailRow>
            <DetailRow label="Created">{formatTimestamp(transfer.created_at)}</DetailRow>
            <DetailRow label="Started">{formatTimestamp(transfer.started_at)}</DetailRow>
            <DetailRow label="Completed">{formatTimestamp(transfer.completed_at)}</DetailRow>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
