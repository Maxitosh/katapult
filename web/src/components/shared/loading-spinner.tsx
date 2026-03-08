// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1

import { cn } from "@/lib/utils"

interface LoadingSpinnerProps {
  className?: string
}

export function LoadingSpinner({ className }: LoadingSpinnerProps) {
  return (
    <div
      className={cn(
        "size-6 animate-spin rounded-full border-2 border-muted border-t-primary",
        className
      )}
      role="status"
      aria-label="Loading"
    />
  )
}
