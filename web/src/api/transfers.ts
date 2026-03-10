// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1
// @cpt-flow:cpt-katapult-flow-web-ui-cancel-transfer:p1

import { apiGet, apiPost, apiDelete } from "./client";
import type { Transfer, TransferEvent, ListResponse } from "./types";

export interface ListTransfersParams {
  state?: string;
  cluster?: string;
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

export function listTransfers(
  params?: ListTransfersParams,
): Promise<ListResponse<Transfer>> {
  return apiGet<ListResponse<Transfer>>(`/transfers${buildQuery(params ? { ...params } : undefined)}`);
}

export function getTransfer(id: string): Promise<Transfer> {
  return apiGet<Transfer>(`/transfers/${id}`);
}

export interface CreateTransferBody {
  source_cluster: string;
  source_pvc: string;
  destination_cluster: string;
  destination_pvc: string;
}

export function createTransfer(body: CreateTransferBody): Promise<Transfer> {
  return apiPost<Transfer>("/transfers", body);
}

export function cancelTransfer(id: string): Promise<void> {
  return apiDelete<void>(`/transfers/${id}`);
}

export async function getTransferEvents(id: string): Promise<TransferEvent[]> {
  const resp = await apiGet<{ events: TransferEvent[] }>(`/transfers/${id}/events`);
  return resp.events;
}
