// @cpt-dod:cpt-katapult-dod-web-ui-validation:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { AlertCircle, AlertTriangle } from "lucide-react"

interface ValidationAlertProps {
  errors: string[]
  warnings: string[]
}

// @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-validation-errors
export function ValidationAlert({ errors, warnings }: ValidationAlertProps) {
  if (errors.length === 0 && warnings.length === 0) return null

  return (
    <div className="flex flex-col gap-2">
      {errors.length > 0 && (
        <Alert variant="destructive">
          <AlertCircle className="size-4" />
          <AlertTitle>Validation Error</AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {errors.map((error) => (
                <li key={error}>{error}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
      {warnings.length > 0 && (
        <Alert>
          <AlertTriangle className="size-4 text-amber-500" />
          <AlertTitle className="text-amber-600 dark:text-amber-400">
            Warning
          </AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
// @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-validation-errors
