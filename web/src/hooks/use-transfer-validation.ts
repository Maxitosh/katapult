// @cpt-dod:cpt-katapult-dod-web-ui-validation:p1
// @cpt-algo:cpt-katapult-algo-web-ui-validate-transfer-params:p1

import { useMemo } from "react";
import type { PVCInfo } from "@/api/types";

interface Endpoint {
  cluster: string;
  agentId: string;
  pvc: PVCInfo | null;
}

interface ValidationResult {
  errors: string[];
  warnings: string[];
  strategyExplanation: string;
}

export function useTransferValidation(
  source: Endpoint,
  dest: Endpoint,
): ValidationResult {
  return useMemo(() => {
    const errors: string[] = [];
    const warnings: string[] = [];
    let strategyExplanation = "";

    // @cpt-begin:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-same-pvc
    if (
      source.cluster === dest.cluster &&
      source.agentId === dest.agentId &&
      source.pvc !== null &&
      dest.pvc !== null &&
      source.pvc.pvc_name === dest.pvc.pvc_name
    ) {
      errors.push("Source and destination must be different");
    }
    // @cpt-end:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-same-pvc

    // @cpt-begin:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-same-agent
    if (
      source.agentId &&
      dest.agentId &&
      source.agentId === dest.agentId
    ) {
      errors.push("Source and destination must be on different nodes");
    }
    // @cpt-end:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-same-agent

    // @cpt-begin:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-size-mismatch
    if (
      source.pvc !== null &&
      dest.pvc !== null &&
      dest.pvc.size_bytes < source.pvc.size_bytes
    ) {
      const srcMB = Math.round(source.pvc.size_bytes / (1024 * 1024));
      const destMB = Math.round(dest.pvc.size_bytes / (1024 * 1024));
      warnings.push(
        `Destination PVC (${destMB} MB) is smaller than source PVC (${srcMB} MB). Data may not fit.`,
      );
    }
    // @cpt-end:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-check-size-mismatch

    // @cpt-begin:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-determine-strategy
    if (source.cluster && dest.cluster) {
      if (source.cluster === dest.cluster) {
        strategyExplanation =
          "Intra-cluster transfer: data will be streamed directly between agents within the same cluster.";
      } else {
        strategyExplanation =
          "Cross-cluster transfer: data will be staged via S3 for transfer between different clusters.";
      }
    }
    // @cpt-end:cpt-katapult-algo-web-ui-validate-transfer-params:p1:inst-determine-strategy

    return { errors, warnings, strategyExplanation };
  }, [source.cluster, source.agentId, source.pvc, dest.cluster, dest.agentId, dest.pvc]);
}
