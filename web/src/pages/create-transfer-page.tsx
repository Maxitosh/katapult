// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import { useState } from "react"
import { useNavigate } from "react-router-dom"
import type { PVCInfo } from "@/api/types"
import { useCreateTransfer } from "@/hooks/use-transfers"
import { useTransferValidation } from "@/hooks/use-transfer-validation"
import { useClusters, useAgents, useAgentPVCs } from "@/hooks/use-agents"
import { ClusterSelector } from "@/components/create-transfer/cluster-selector"
import { PVCSelector } from "@/components/create-transfer/pvc-selector"
import { ValidationAlert } from "@/components/create-transfer/validation-alert"
import { ErrorAlert } from "@/components/shared/error-alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

export function CreateTransferPage() {
  const navigate = useNavigate()
  const createMutation = useCreateTransfer()
  const { data: clusters = [] } = useClusters()

  const [sourceCluster, setSourceCluster] = useState("")
  const [destCluster, setDestCluster] = useState("")
  const [sourcePvc, setSourcePvc] = useState("")
  const [destPvc, setDestPvc] = useState("")

  const { data: sourceAgents } = useAgents(
    sourceCluster ? { cluster_id: sourceCluster } : undefined,
  )
  const { data: destAgents } = useAgents(
    destCluster ? { cluster_id: destCluster } : undefined,
  )

  const sourceAgent = sourceAgents?.items[0]
  const destAgent = destAgents?.items[0]

  const { data: sourcePvcs = [] } = useAgentPVCs(sourceAgent?.id ?? "")
  const { data: destPvcs = [] } = useAgentPVCs(destAgent?.id ?? "")

  const selectedSourcePvc: PVCInfo | null =
    sourcePvcs.find((p) => p.pvc_name === sourcePvc) ?? null
  const selectedDestPvc: PVCInfo | null =
    destPvcs.find((p) => p.pvc_name === destPvc) ?? null

  const { errors, warnings, strategyExplanation } = useTransferValidation(
    { cluster: sourceCluster, agentId: sourceAgent?.id ?? "", pvc: selectedSourcePvc },
    { cluster: destCluster, agentId: destAgent?.id ?? "", pvc: selectedDestPvc },
  )

  const canSubmit =
    sourceCluster && destCluster && sourcePvc && destPvc && errors.length === 0

  // @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-navigate-create
  function handleSubmit() {
    createMutation.mutate(
      {
        source_cluster: sourceCluster,
        source_pvc: sourcePvc,
        destination_cluster: destCluster,
        destination_pvc: destPvc,
      },
      {
        onSuccess: (transfer) => {
          navigate(`/transfers/${transfer.id}`)
        },
      },
    )
  }
  // @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-navigate-create

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">New Transfer</h1>
        <Button variant="outline" size="sm" onClick={() => navigate("/")}>
          Cancel
        </Button>
      </div>

      {createMutation.error && (
        <ErrorAlert
          message={
            createMutation.error instanceof Error
              ? createMutation.error.message
              : "Failed to create transfer"
          }
        />
      )}

      <div className="grid gap-6 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Source</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <ClusterSelector
              clusters={clusters}
              value={sourceCluster}
              onChange={(v) => {
                setSourceCluster(v)
                setSourcePvc("")
              }}
              label="Cluster"
            />
            <PVCSelector
              pvcs={sourcePvcs}
              value={sourcePvc}
              onChange={setSourcePvc}
              label="PVC"
              disabled={!sourceCluster}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Destination</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <ClusterSelector
              clusters={clusters}
              value={destCluster}
              onChange={(v) => {
                setDestCluster(v)
                setDestPvc("")
              }}
              label="Cluster"
            />
            <PVCSelector
              pvcs={destPvcs}
              value={destPvc}
              onChange={setDestPvc}
              label="PVC"
              disabled={!destCluster}
            />
          </CardContent>
        </Card>
      </div>

      <ValidationAlert errors={errors} warnings={warnings} />

      {strategyExplanation && (
        <p className="text-xs text-muted-foreground">{strategyExplanation}</p>
      )}

      <div className="flex justify-end">
        <Button
          onClick={handleSubmit}
          disabled={!canSubmit || createMutation.isPending}
        >
          {createMutation.isPending ? "Creating..." : "Create Transfer"}
        </Button>
      </div>
    </div>
  )
}
