// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-algo:cpt-katapult-algo-web-ui-render-progress:p2

import { Progress } from "@/components/ui/progress"

interface TransferProgressBarProps {
  bytesTransferred: number
  bytesTotal: number
  speed?: string
  eta?: string
  chunksCompleted?: number
  chunksTotal?: number
}

export function TransferProgressBar({
  bytesTransferred,
  bytesTotal,
  speed,
  eta,
  chunksCompleted,
  chunksTotal,
}: TransferProgressBarProps) {
  // @cpt-begin:cpt-katapult-algo-web-ui-render-progress:p2:inst-compute-percentage
  const percentage = bytesTotal > 0 ? Math.min(100, Math.round((bytesTransferred / bytesTotal) * 100)) : 0
  // @cpt-end:cpt-katapult-algo-web-ui-render-progress:p2:inst-compute-percentage

  return (
    <div className="flex w-full flex-col gap-1">
      <Progress value={percentage} />
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>{percentage}%</span>
        <div className="flex items-center gap-2">
          {speed && <span>{speed}</span>}
          {eta && <span>ETA: {eta}</span>}
        </div>
      </div>
      {/* @cpt-begin:cpt-katapult-algo-web-ui-render-progress:p2:inst-render-chunks */}
      {chunksTotal != null && chunksTotal > 0 && (
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span>
            Chunks: {chunksCompleted ?? 0} / {chunksTotal}
          </span>
          <div className="flex gap-0.5">
            {Array.from({ length: chunksTotal }, (_, i) => (
              <div
                key={i}
                className={`h-1.5 w-1.5 rounded-full ${
                  i < (chunksCompleted ?? 0) ? "bg-primary" : "bg-muted"
                }`}
              />
            ))}
          </div>
        </div>
      )}
      {/* @cpt-end:cpt-katapult-algo-web-ui-render-progress:p2:inst-render-chunks */}
    </div>
  )
}
