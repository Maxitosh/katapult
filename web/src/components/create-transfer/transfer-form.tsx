// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-dod:cpt-katapult-dod-web-ui-validation:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1

import { useState, useCallback, useMemo } from "react"
import type { PVCInfo } from "@/api/types"
import { useClusters, useAgents, useAgentPVCs } from "@/hooks/use-agents"
import { useTransferValidation } from "@/hooks/use-transfer-validation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { TooltipHelp } from "@/components/shared/tooltip-help"
import { ClusterSelector } from "./cluster-selector"
import { AgentSelector } from "./agent-selector"
import { PVCSelector } from "./pvc-selector"
import { ValidationAlert } from "./validation-alert"
import { ConfirmationDialog } from "./confirmation-dialog"

interface TransferFormData {
  source_cluster: string
  source_pvc: string
  destination_cluster: string
  destination_pvc: string
}

interface TransferFormProps {
  onSubmit: (data: TransferFormData) => void
  isLoading: boolean
}

export function TransferForm({ onSubmit, isLoading }: TransferFormProps) {
  // --- Source state ---
  const [sourceCluster, setSourceCluster] = useState("")
  const [sourceAgentId, setSourceAgentId] = useState("")
  const [sourcePVCName, setSourcePVCName] = useState("")

  // --- Destination state ---
  const [destCluster, setDestCluster] = useState("")
  const [destAgentId, setDestAgentId] = useState("")
  const [destPVCName, setDestPVCName] = useState("")

  // --- Confirmation dialog ---
  const [confirmOpen, setConfirmOpen] = useState(false)

  // @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-fetch-clusters
  const { data: clusters = [] } = useClusters()

  const { data: sourceAgentsData } = useAgents(
    sourceCluster ? { cluster_id: sourceCluster } : undefined,
  )
  const sourceAgents = sourceAgentsData?.items ?? []

  const { data: sourcePVCs = [] } = useAgentPVCs(sourceAgentId)

  const { data: destAgentsData } = useAgents(
    destCluster ? { cluster_id: destCluster } : undefined,
  )
  const destAgents = destAgentsData?.items ?? []

  const { data: destPVCs = [] } = useAgentPVCs(destAgentId)
  // @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-fetch-clusters

  // --- Cascading resets ---
  const handleSourceClusterChange = useCallback((cluster: string) => {
    setSourceCluster(cluster)
    setSourceAgentId("")
    setSourcePVCName("")
  }, [])

  const handleSourceAgentChange = useCallback((agentId: string) => {
    setSourceAgentId(agentId)
    setSourcePVCName("")
  }, [])

  const handleDestClusterChange = useCallback((cluster: string) => {
    setDestCluster(cluster)
    setDestAgentId("")
    setDestPVCName("")
  }, [])

  const handleDestAgentChange = useCallback((agentId: string) => {
    setDestAgentId(agentId)
    setDestPVCName("")
  }, [])

  // --- Resolve selected PVC objects for validation ---
  const sourcePVC: PVCInfo | null = useMemo(
    () => sourcePVCs.find((p) => p.pvc_name === sourcePVCName) ?? null,
    [sourcePVCs, sourcePVCName],
  )

  const destPVC: PVCInfo | null = useMemo(
    () => destPVCs.find((p) => p.pvc_name === destPVCName) ?? null,
    [destPVCs, destPVCName],
  )

  // @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-run-validation
  const { errors, warnings, strategyExplanation } = useTransferValidation(
    { cluster: sourceCluster, agentId: sourceAgentId, pvc: sourcePVC },
    { cluster: destCluster, agentId: destAgentId, pvc: destPVC },
  )
  // @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-run-validation

  const isFormComplete =
    sourceCluster !== "" &&
    sourcePVCName !== "" &&
    destCluster !== "" &&
    destPVCName !== ""

  const canSubmit = isFormComplete && errors.length === 0

  // @cpt-begin:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-confirmation
  const handleSubmitClick = useCallback(() => {
    setConfirmOpen(true)
  }, [])

  const handleConfirm = useCallback(() => {
    onSubmit({
      source_cluster: sourceCluster,
      source_pvc: sourcePVCName,
      destination_cluster: destCluster,
      destination_pvc: destPVCName,
    })
    setConfirmOpen(false)
  }, [onSubmit, sourceCluster, sourcePVCName, destCluster, destPVCName])
  // @cpt-end:cpt-katapult-flow-web-ui-create-transfer:p1:inst-show-confirmation

  return (
    <Card>
      <CardHeader>
        <CardTitle>Create PVC Transfer</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-6">
          {/* Source section */}
          <fieldset className="flex flex-col gap-3">
            <legend className="mb-1 text-sm font-semibold text-foreground">
              <TooltipHelp content="The cluster, agent, and PVC to transfer data from">
                Source
              </TooltipHelp>
            </legend>

            <ClusterSelector
              clusters={clusters}
              value={sourceCluster}
              onChange={handleSourceClusterChange}
              label="Cluster"
            />

            <TooltipHelp content="Select the agent node that hosts the source PVC">
              <AgentSelector
                agents={sourceAgents}
                value={sourceAgentId}
                onChange={handleSourceAgentChange}
                label="Agent"
                disabled={!sourceCluster}
              />
            </TooltipHelp>

            <TooltipHelp content="The persistent volume claim to read data from">
              <PVCSelector
                pvcs={sourcePVCs}
                value={sourcePVCName}
                onChange={setSourcePVCName}
                label="PVC"
                disabled={!sourceAgentId}
              />
            </TooltipHelp>
          </fieldset>

          {/* Destination section */}
          <fieldset className="flex flex-col gap-3">
            <legend className="mb-1 text-sm font-semibold text-foreground">
              <TooltipHelp content="The cluster, agent, and PVC to transfer data to">
                Destination
              </TooltipHelp>
            </legend>

            <ClusterSelector
              clusters={clusters}
              value={destCluster}
              onChange={handleDestClusterChange}
              label="Cluster"
            />

            <TooltipHelp content="Select the agent node that hosts the destination PVC">
              <AgentSelector
                agents={destAgents}
                value={destAgentId}
                onChange={handleDestAgentChange}
                label="Agent"
                disabled={!destCluster}
              />
            </TooltipHelp>

            <TooltipHelp content="The persistent volume claim to write data to">
              <PVCSelector
                pvcs={destPVCs}
                value={destPVCName}
                onChange={setDestPVCName}
                label="PVC"
                disabled={!destAgentId}
              />
            </TooltipHelp>
          </fieldset>

          <ValidationAlert errors={errors} warnings={warnings} />

          <Button
            onClick={handleSubmitClick}
            disabled={!canSubmit || isLoading}
            className="w-full"
          >
            {isLoading ? "Creating..." : "Create Transfer"}
          </Button>

          <ConfirmationDialog
            open={confirmOpen}
            onOpenChange={setConfirmOpen}
            onConfirm={handleConfirm}
            sourceCluster={sourceCluster}
            sourcePVC={sourcePVCName}
            destCluster={destCluster}
            destPVC={destPVCName}
            strategyExplanation={strategyExplanation}
            warnings={warnings}
            isLoading={isLoading}
          />
        </div>
      </CardContent>
    </Card>
  )
}
