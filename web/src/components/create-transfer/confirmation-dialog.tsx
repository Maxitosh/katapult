// @cpt-dod:cpt-katapult-dod-web-ui-validation:p1
// @cpt-dod:cpt-katapult-dod-web-ui-documentation:p2
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import { useState } from "react"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { AlertTriangle } from "lucide-react"
import { StrategyExplanation } from "./strategy-explanation"

interface ConfirmationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
  sourceCluster: string
  sourcePVC: string
  destCluster: string
  destPVC: string
  strategyExplanation: string
  warnings: string[]
  isLoading: boolean
}

// @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-confirmation
export function ConfirmationDialog({
  open,
  onOpenChange,
  onConfirm,
  sourceCluster,
  sourcePVC,
  destCluster,
  destPVC,
  strategyExplanation,
  warnings,
  isLoading,
}: ConfirmationDialogProps) {
  const [acknowledged, setAcknowledged] = useState(false)
  const hasWarnings = warnings.length > 0
  const canConfirm = !hasWarnings || acknowledged

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Confirm Transfer</DialogTitle>
          <DialogDescription>
            Review the transfer details before proceeding.
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-3">
          <div className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
            <span className="font-medium text-muted-foreground">Source cluster</span>
            <span>{sourceCluster}</span>
            <span className="font-medium text-muted-foreground">Source PVC</span>
            <span>{sourcePVC}</span>
            <span className="font-medium text-muted-foreground">Dest cluster</span>
            <span>{destCluster}</span>
            <span className="font-medium text-muted-foreground">Dest PVC</span>
            <span>{destPVC}</span>
          </div>

          <StrategyExplanation explanation={strategyExplanation} />

          {hasWarnings && (
            <div className="flex flex-col gap-2">
              <Alert>
                <AlertTriangle className="size-4 text-amber-500" />
                <AlertDescription>
                  <ul className="list-inside list-disc">
                    {warnings.map((w) => (
                      <li key={w}>{w}</li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  checked={acknowledged}
                  onChange={(e) => setAcknowledged(e.target.checked)}
                  className="size-4 rounded border-input"
                />
                I acknowledge these warnings and want to proceed
              </label>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            onClick={onConfirm}
            disabled={!canConfirm || isLoading}
          >
            {isLoading ? "Creating..." : "Confirm Transfer"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
// @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-confirmation
