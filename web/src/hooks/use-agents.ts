// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import { useQuery } from "@tanstack/react-query";
import {
  listAgents,
  getAgent,
  getAgentPVCs,
  listClusters,
} from "@/api/agents";
import type { ListAgentsParams } from "@/api/agents";

export function useAgents(filter?: ListAgentsParams) {
  return useQuery({
    queryKey: ["agents", filter] as const,
    queryFn: () => listAgents(filter),
  });
}

export function useAgent(id: string) {
  return useQuery({
    queryKey: ["agent", id] as const,
    queryFn: () => getAgent(id),
    enabled: !!id,
  });
}

export function useAgentPVCs(id: string) {
  return useQuery({
    queryKey: ["agent-pvcs", id] as const,
    queryFn: () => getAgentPVCs(id),
    enabled: !!id,
  });
}

export function useClusters() {
  return useQuery({
    queryKey: ["clusters"] as const,
    queryFn: () => listClusters(),
  });
}
