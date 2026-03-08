// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1

import type { TransferState } from "@/api/types"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

interface TransferStatusBadgeProps {
  state: TransferState
}

const stateConfig: Record<
  TransferState,
  { variant: "default" | "secondary" | "outline" | "destructive"; label: string; className?: string }
> = {
  pending: { variant: "secondary", label: "Pending" },
  validating: { variant: "outline", label: "Validating", className: "text-blue-600 border-blue-300" },
  transferring: { variant: "default", label: "Transferring", className: "animate-pulse bg-blue-600 text-white" },
  completed: { variant: "default", label: "Completed", className: "bg-green-600 text-white" },
  failed: { variant: "destructive", label: "Failed" },
  cancelled: { variant: "secondary", label: "Cancelled", className: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200" },
}

export function TransferStatusBadge({ state }: TransferStatusBadgeProps) {
  const config = stateConfig[state]

  return (
    <Badge variant={config.variant} className={cn(config.className)}>
      {config.label}
    </Badge>
  )
}
