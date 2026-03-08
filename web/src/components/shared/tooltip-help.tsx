// @cpt-dod:cpt-katapult-dod-web-ui-documentation:p2

import type { ReactNode } from "react"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

interface TooltipHelpProps {
  content: string
  children: ReactNode
}

export function TooltipHelp({ content, children }: TooltipHelpProps) {
  return (
    <span className="inline-flex items-center gap-1">
      {children}
      <TooltipProvider>
        <Tooltip>
          <TooltipTrigger
            render={<span
              className="inline-flex size-4 cursor-help items-center justify-center rounded-full border border-muted-foreground/40 text-[10px] font-medium text-muted-foreground transition-colors hover:border-foreground hover:text-foreground"
              aria-label="Help"
            />}
          >
            ?
          </TooltipTrigger>
          <TooltipContent>{content}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </span>
  )
}
