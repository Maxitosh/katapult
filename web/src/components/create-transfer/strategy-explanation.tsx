// @cpt-dod:cpt-katapult-dod-web-ui-validation:p1
// @cpt-dod:cpt-katapult-dod-web-ui-documentation:p2

import { Info } from "lucide-react"

interface StrategyExplanationProps {
  explanation: string
}

// @cpt-begin:cpt-katapult-dod-web-ui-documentation:p2:inst-show-strategy-explanation
export function StrategyExplanation({
  explanation,
}: StrategyExplanationProps) {
  if (!explanation) return null

  return (
    <div className="flex items-start gap-2 rounded-md bg-muted/60 px-3 py-2.5 text-sm text-muted-foreground">
      <Info className="mt-0.5 size-4 shrink-0" />
      <span>{explanation}</span>
    </div>
  )
}
// @cpt-end:cpt-katapult-dod-web-ui-documentation:p2:inst-show-strategy-explanation
