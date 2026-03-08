// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1
// @cpt-flow:cpt-katapult-flow-web-ui-cancel-transfer:p1

import { useState } from "react"
import { useParams, useNavigate } from "react-router-dom"
import { useTransfer, useTransferEvents, useCancelTransfer } from "@/hooks/use-transfers"
import { useTransferProgress } from "@/hooks/use-transfer-progress"
import { TransferSummaryCard } from "@/components/transfers/transfer-summary-card"
import { TransferProgressBar } from "@/components/transfers/transfer-progress-bar"
import { TransferEventTimeline } from "@/components/transfers/transfer-event-timeline"
import { CancelTransferDialog } from "@/components/transfers/cancel-transfer-dialog"
import { LoadingSpinner } from "@/components/shared/loading-spinner"
import { ErrorAlert } from "@/components/shared/error-alert"
import { Button } from "@/components/ui/button"

const TERMINAL_STATES = new Set(["completed", "failed", "cancelled"])

export function TransferDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [cancelDialogOpen, setCancelDialogOpen] = useState(false)

  const { data: transfer, isLoading, error, refetch } = useTransfer(id ?? "")
  const { data: events } = useTransferEvents(id ?? "")
  const cancelMutation = useCancelTransfer()

  // @cpt-begin:cpt-katapult-flow-web-ui-monitor-transfer:p1:inst-subscribe-sse
  const { progress } = useTransferProgress(id)
  // @cpt-end:cpt-katapult-flow-web-ui-monitor-transfer:p1:inst-subscribe-sse

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
    )
  }

  if (error || !transfer) {
    return (
      <ErrorAlert
        message={error instanceof Error ? error.message : "Failed to load transfer"}
        onRetry={() => refetch()}
      />
    )
  }

  const isTerminal = TERMINAL_STATES.has(transfer.state)

  const bytesTransferred = progress?.bytes_transferred ?? transfer.bytes_transferred
  const bytesTotal = progress?.bytes_total ?? transfer.bytes_total
  const chunksCompleted = progress?.chunks_completed ?? transfer.chunks_completed
  const chunksTotal = progress?.chunks_total ?? transfer.chunks_total
  const speed = progress?.formatted_speed
  const eta = progress?.estimated_time_remaining

  function handleCancelConfirm() {
    if (!transfer) return
    cancelMutation.mutate(transfer.id, {
      onSuccess: () => {
        setCancelDialogOpen(false)
        refetch()
      },
    })
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <Button variant="outline" size="sm" onClick={() => navigate("/")}>
          Back to Transfers
        </Button>
        {!isTerminal && (
          <Button
            variant="destructive"
            size="sm"
            onClick={() => setCancelDialogOpen(true)}
          >
            Cancel Transfer
          </Button>
        )}
      </div>

      <TransferSummaryCard transfer={transfer} />

      <div>
        <h2 className="mb-2 text-sm font-semibold">Progress</h2>
        <TransferProgressBar
          bytesTransferred={bytesTransferred}
          bytesTotal={bytesTotal}
          speed={speed}
          eta={eta}
          chunksCompleted={chunksCompleted}
          chunksTotal={chunksTotal}
        />
      </div>

      <div>
        <h2 className="mb-3 text-sm font-semibold">Events</h2>
        <TransferEventTimeline events={events ?? []} />
      </div>

      <CancelTransferDialog
        transfer={transfer}
        open={cancelDialogOpen}
        onOpenChange={setCancelDialogOpen}
        onConfirm={handleCancelConfirm}
        isLoading={cancelMutation.isPending}
      />
    </div>
  )
}
