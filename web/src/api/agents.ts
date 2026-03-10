// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-flow:cpt-katapult-flow-web-ui-browse-agents:p2

import { apiGet } from "./client";
import type { Agent, PVCInfo, ListResponse } from "./types";

export interface ListAgentsParams {
  cluster_id?: string;
  state?: string;
  limit?: number;
  offset?: number;
}

function buildQuery(params?: Record<string, string | number | undefined>): string {
  if (!params) return "";
  const entries = Object.entries(params).filter(
    ([, v]) => v !== undefined && v !== "",
  );
  if (entries.length === 0) return "";
  const qs = new URLSearchParams(
    entries.map(([k, v]) => [k, String(v)]),
  ).toString();
  return `?${qs}`;
}

export function listAgents(
  params?: ListAgentsParams,
): Promise<ListResponse<Agent>> {
  return apiGet<ListResponse<Agent>>(`/agents${buildQuery(params ? { ...params } : undefined)}`);
}

export function getAgent(id: string): Promise<Agent> {
  return apiGet<Agent>(`/agents/${id}`);
}

export async function getAgentPVCs(id: string): Promise<PVCInfo[]> {
  const resp = await apiGet<{ pvcs: PVCInfo[] }>(`/agents/${id}/pvcs`);
  return resp.pvcs;
}

export async function listClusters(): Promise<string[]> {
  const resp = await apiGet<{ clusters: string[] }>("/clusters");
  return resp.clusters;
}
