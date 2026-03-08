// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-algo:cpt-katapult-algo-web-ui-render-progress:p2

import { useState, useEffect, useRef, useCallback } from "react";
import type { EnrichedProgress } from "@/api/types";
import { streamProgress } from "@/api/sse";

const INITIAL_BACKOFF_MS = 1_000;
const MAX_BACKOFF_MS = 30_000;

interface TransferProgressState {
  progress: EnrichedProgress | null;
  isConnected: boolean;
  error: string | null;
}

export function useTransferProgress(
  transferId: string | undefined,
): TransferProgressState {
  const [progress, setProgress] = useState<EnrichedProgress | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const abortRef = useRef<AbortController | null>(null);

  const connect = useCallback(async (id: string, signal: AbortSignal) => {
    let backoff = INITIAL_BACKOFF_MS;

    while (!signal.aborted) {
      try {
        setIsConnected(true);
        setError(null);

        for await (const event of streamProgress(id)) {
          if (signal.aborted) return;
          backoff = INITIAL_BACKOFF_MS;
          setProgress(event);
        }
      } catch (err) {
        if (signal.aborted) return;
        setIsConnected(false);
        const message =
          err instanceof Error ? err.message : "SSE connection error";
        setError(message);

        // Exponential backoff before reconnect
        await new Promise<void>((resolve) => {
          const timer = setTimeout(resolve, backoff);
          signal.addEventListener("abort", () => {
            clearTimeout(timer);
            resolve();
          });
        });

        backoff = Math.min(backoff * 2, MAX_BACKOFF_MS);
      }
    }
  }, []);

  useEffect(() => {
    if (!transferId) {
      setProgress(null);
      setIsConnected(false);
      setError(null);
      return;
    }

    const controller = new AbortController();
    abortRef.current = controller;

    connect(transferId, controller.signal);

    return () => {
      controller.abort();
      abortRef.current = null;
      setIsConnected(false);
    };
  }, [transferId, connect]);

  return { progress, isConnected, error };
}
