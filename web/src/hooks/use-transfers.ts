// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-flow:cpt-katapult-flow-web-ui-create-transfer:p1
// @cpt-flow:cpt-katapult-flow-web-ui-monitor-transfer:p1
// @cpt-flow:cpt-katapult-flow-web-ui-cancel-transfer:p1

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import type { Transfer, ListResponse } from "@/api/types";
import {
  listTransfers,
  getTransfer,
  getTransferEvents,
  createTransfer,
  cancelTransfer,
} from "@/api/transfers";
import type { ListTransfersParams, CreateTransferBody } from "@/api/transfers";

const ACTIVE_STATES = new Set(["pending", "validating", "transferring"]);

function hasActiveTransfers(data: ListResponse<Transfer> | undefined): boolean {
  return !!data?.items.some((t) => ACTIVE_STATES.has(t.state));
}

export function useTransfers(filter?: ListTransfersParams) {
  return useQuery({
    queryKey: ["transfers", filter] as const,
    queryFn: () => listTransfers(filter),
    refetchInterval: (query) =>
      hasActiveTransfers(query.state.data) ? 10_000 : false,
  });
}

export function useTransfer(id: string) {
  return useQuery({
    queryKey: ["transfer", id] as const,
    queryFn: () => getTransfer(id),
    enabled: !!id,
  });
}

export function useTransferEvents(id: string) {
  return useQuery({
    queryKey: ["transfer-events", id] as const,
    queryFn: () => getTransferEvents(id),
  });
}

export function useCreateTransfer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (body: CreateTransferBody) => createTransfer(body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["transfers"] });
    },
  });
}

export function useCancelTransfer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => cancelTransfer(id),
    onSuccess: (_data, id) => {
      queryClient.invalidateQueries({ queryKey: ["transfers"] });
      queryClient.invalidateQueries({ queryKey: ["transfer", id] });
    },
  });
}
