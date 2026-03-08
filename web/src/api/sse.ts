// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-algo:cpt-katapult-algo-web-ui-render-progress:p2

import type { EnrichedProgress } from "./types";

const BASE_URL = "/api/v1alpha1";
const MAX_BACKOFF_MS = 30_000;
const INITIAL_BACKOFF_MS = 1_000;

function getAuthToken(): string {
  return import.meta.env.VITE_API_TOKEN ?? "";
}

/**
 * Parse SSE frames from a text chunk. Handles `event: progress\ndata: {...}\n\n`.
 * Yields parsed EnrichedProgress objects for each complete frame.
 */
function* parseSseFrames(
  buffer: string,
): Generator<{ rest: string; event: EnrichedProgress | null }> {
  const frameEnd = buffer.indexOf("\n\n");
  if (frameEnd === -1) {
    yield { rest: buffer, event: null };
    return;
  }

  let remaining = buffer;
  while (remaining.includes("\n\n")) {
    const idx = remaining.indexOf("\n\n");
    const frame = remaining.slice(0, idx);
    remaining = remaining.slice(idx + 2);

    let eventType = "";
    let data = "";

    for (const line of frame.split("\n")) {
      if (line.startsWith("event:")) {
        eventType = line.slice("event:".length).trim();
      } else if (line.startsWith("data:")) {
        data = line.slice("data:".length).trim();
      }
    }

    if (eventType === "progress" && data) {
      try {
        const parsed = JSON.parse(data) as EnrichedProgress;
        yield { rest: remaining, event: parsed };
      } catch {
        yield { rest: remaining, event: null };
      }
    } else {
      yield { rest: remaining, event: null };
    }
  }

  yield { rest: remaining, event: null };
}

/**
 * Fetch-based SSE reader that supports Authorization headers.
 * Yields EnrichedProgress events as they arrive from the stream.
 * Implements exponential backoff reconnection (1s -> 2s -> 4s, max 30s).
 */
export async function* streamProgress(
  transferId: string,
): AsyncGenerator<EnrichedProgress> {
  let backoff = INITIAL_BACKOFF_MS;

  while (true) {
    const response = await fetch(
      `${BASE_URL}/transfers/${transferId}/progress`,
      {
        method: "GET",
        headers: {
          Authorization: `Bearer ${getAuthToken()}`,
          Accept: "text/event-stream",
        },
      },
    );

    if (!response.ok) {
      throw new Error(
        `SSE connection failed: ${response.status} ${response.statusText}`,
      );
    }

    const reader = response.body?.getReader();
    if (!reader) {
      throw new Error("Response body is not readable");
    }

    const decoder = new TextDecoder();
    let buffer = "";
    let connectionClosed = false;

    try {
      while (true) {
        const { done, value } = await reader.read();
        if (done) {
          connectionClosed = true;
          break;
        }

        buffer += decoder.decode(value, { stream: true });

        for (const result of parseSseFrames(buffer)) {
          buffer = result.rest;
          if (result.event) {
            backoff = INITIAL_BACKOFF_MS;
            yield result.event;
          }
        }
      }
    } finally {
      reader.releaseLock();
    }

    if (connectionClosed) {
      // Wait with exponential backoff before reconnecting
      await new Promise((resolve) => setTimeout(resolve, backoff));
      backoff = Math.min(backoff * 2, MAX_BACKOFF_MS);
    }
  }
}
