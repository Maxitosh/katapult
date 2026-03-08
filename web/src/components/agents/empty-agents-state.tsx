// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-dod:cpt-katapult-dod-web-ui-documentation:p2

import { ServerOff } from "lucide-react"

export function EmptyAgentsState() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-16 text-center">
      <div className="flex size-16 items-center justify-center rounded-full bg-muted">
        <ServerOff className="size-8 text-muted-foreground" />
      </div>
      <div className="space-y-1">
        <h3 className="text-lg font-semibold text-foreground">No Agents Registered</h3>
        <p className="max-w-sm text-sm text-muted-foreground">
          Deploy the Katapult agent DaemonSet to your Kubernetes clusters to get started.
        </p>
      </div>
      <a
        href="/docs/agents"
        className="text-sm font-medium text-primary underline-offset-4 hover:underline"
      >
        View deployment guide
      </a>
    </div>
  )
}
