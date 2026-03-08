// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1

import { useState } from "react"
import { useNavigate } from "react-router-dom"
import type { TransferState } from "@/api/types"
import { useTransfers } from "@/hooks/use-transfers"
import { TransferList } from "@/components/transfers/transfer-list"
import { LoadingSpinner } from "@/components/shared/loading-spinner"
import { ErrorAlert } from "@/components/shared/error-alert"
import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

const ALL_STATES: TransferState[] = [
  "pending",
  "validating",
  "transferring",
  "completed",
  "failed",
  "cancelled",
]

export function TransfersPage() {
  const navigate = useNavigate()
  const [stateFilter, setStateFilter] = useState<string>("")
  const [clusterFilter, setClusterFilter] = useState("")

  // @cpt-begin:cpt-katapult-flow-web-ui-monitor-transfer:p1:inst-fetch-transfers
  const { data, isLoading, error, refetch } = useTransfers({
    state: stateFilter || undefined,
    cluster: clusterFilter || undefined,
  })
  // @cpt-end:cpt-katapult-flow-web-ui-monitor-transfer:p1:inst-fetch-transfers

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Transfers</h1>
        <Button onClick={() => navigate("/transfers/new")}>New Transfer</Button>
      </div>

      <div className="flex items-center gap-3">
        <Select value={stateFilter} onValueChange={(val) => setStateFilter(val ?? "")}>
          <SelectTrigger className="w-44">
            <SelectValue placeholder="All states" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="">All states</SelectItem>
            {ALL_STATES.map((s) => (
              <SelectItem key={s} value={s}>
                <span className="capitalize">{s}</span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <input
          type="text"
          placeholder="Filter by cluster..."
          value={clusterFilter}
          onChange={(e) => setClusterFilter(e.target.value)}
          className="h-8 rounded-lg border border-input bg-transparent px-2.5 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </div>

      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <LoadingSpinner />
        </div>
      )}

      {error && (
        <ErrorAlert
          message={error instanceof Error ? error.message : "Failed to load transfers"}
          onRetry={() => refetch()}
        />
      )}

      {data && data.items.length > 0 && (
        <TransferList
          transfers={data.items}
          onSelect={(id) => navigate(`/transfers/${id}`)}
        />
      )}

      {data && data.items.length === 0 && !isLoading && (
        <p className="py-12 text-center text-sm text-muted-foreground">
          No transfers found.
        </p>
      )}
    </div>
  )
}
