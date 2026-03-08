// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1

import type { TransferEvent } from "@/api/types"
import { Badge } from "@/components/ui/badge"

interface TransferEventTimelineProps {
  events: TransferEvent[]
}

export function TransferEventTimeline({ events }: TransferEventTimelineProps) {
  if (events.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">No events recorded yet.</p>
    )
  }

  return (
    <div className="relative flex flex-col gap-0">
      {events.map((event, index) => (
        <div key={event.id} className="relative flex gap-3 pb-6 last:pb-0">
          {/* Vertical line */}
          {index < events.length - 1 && (
            <div className="absolute left-[7px] top-4 h-full w-px bg-border" />
          )}
          {/* Dot */}
          <div className="relative z-10 mt-1.5 h-[15px] w-[15px] shrink-0 rounded-full border-2 border-primary bg-background" />
          {/* Content */}
          <div className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <Badge variant="outline" className="text-xs">
                {event.event_type}
              </Badge>
              <span className="text-xs text-muted-foreground">
                {new Date(event.created_at).toLocaleString()}
              </span>
            </div>
            <p className="text-sm">{event.message}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
