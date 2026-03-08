// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-cancel-transfer:p1

import type { Transfer } from "@/api/types"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"

interface CancelTransferDialogProps {
  transfer: Transfer
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  isLoading: boolean
}

// @cpt-begin:cpt-katapult-flow-web-ui-cancel-transfer:p1:inst-show-cancel-confirm
export function CancelTransferDialog({
  transfer,
  open,
  onOpenChange,
  onConfirm,
  isLoading,
}: CancelTransferDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Cancel Transfer</DialogTitle>
          <DialogDescription>
            Are you sure you want to cancel this transfer? This action cannot be
            undone. Partial data may remain on the destination PVC.
          </DialogDescription>
        </DialogHeader>

        <div className="rounded-lg border p-3 text-xs">
          <div className="grid grid-cols-2 gap-2">
            <div>
              <span className="text-muted-foreground">Transfer ID</span>
              <p className="font-mono font-medium">{transfer.id.slice(0, 8)}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Strategy</span>
              <p className="capitalize font-medium">{transfer.strategy}</p>
            </div>
            <div>
              <span className="text-muted-foreground">Source</span>
              <p className="font-medium">
                {transfer.source_cluster}/{transfer.source_pvc}
              </p>
            </div>
            <div>
              <span className="text-muted-foreground">Destination</span>
              <p className="font-medium">
                {transfer.destination_cluster}/{transfer.destination_pvc}
              </p>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={isLoading}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={isLoading}>
            {isLoading ? "Cancelling..." : "Confirm Cancellation"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
// @cpt-end:cpt-katapult-flow-web-ui-cancel-transfer:p1:inst-show-cancel-confirm
