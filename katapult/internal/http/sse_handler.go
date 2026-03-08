package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/observability"
)

// @cpt-flow:cpt-katapult-flow-observability-stream-progress:p1
// @cpt-dod:cpt-katapult-dod-observability-realtime-progress:p1

const sseKeepaliveInterval = 15 * time.Second

// handleStreamProgress handles GET /api/v1alpha1/transfers/{id}/progress (SSE).
func (s *Server) handleStreamProgress(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-request-stream
	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-sse-connect
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid transfer ID format")
		return
	}
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-sse-connect
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-request-stream

	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-check-transfer-exists
	transfer, err := s.orchestrator.GetTransfer(r.Context(), id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	if transfer == nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "transfer not found")
		return
	}
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-check-transfer-exists

	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-check-terminal
	if transfer.IsTerminal() {
		snapshot := observability.EnrichedProgress{
			TransferID:       transfer.ID,
			BytesTransferred: transfer.BytesTransferred,
			BytesTotal:       transfer.BytesTotal,
			ChunksCompleted:  transfer.ChunksCompleted,
			ChunksTotal:      transfer.ChunksTotal,
			Status:           string(transfer.State),
			CorrelationID:    transfer.ID.String(),
		}
		if transfer.BytesTotal > 0 {
			snapshot.PercentComplete = float64(transfer.BytesTransferred) / float64(transfer.BytesTotal) * 100
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		writeSSEEvent(w, "progress", snapshot)
		return
	}
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-check-terminal

	if s.progressHub == nil {
		writeErrorJSON(w, http.StatusServiceUnavailable, "unavailable", "progress streaming not available")
		return
	}

	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-register-subscriber
	ch, unsub := s.progressHub.Subscribe(id)
	defer unsub()
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-register-subscriber

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErrorJSON(w, http.StatusInternalServerError, "sse_unsupported", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Send current state as the first event.
	initial := observability.EnrichedProgress{
		TransferID:       transfer.ID,
		BytesTransferred: transfer.BytesTransferred,
		BytesTotal:       transfer.BytesTotal,
		ChunksCompleted:  transfer.ChunksCompleted,
		ChunksTotal:      transfer.ChunksTotal,
		Status:           string(transfer.State),
		CorrelationID:    transfer.ID.String(),
	}
	if transfer.BytesTotal > 0 {
		initial.PercentComplete = float64(transfer.BytesTransferred) / float64(transfer.BytesTotal) * 100
	}
	writeSSEEvent(w, "progress", initial)
	flusher.Flush()

	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-push-sse
	// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-return-stream
	keepalive := time.NewTicker(sseKeepaliveInterval)
	defer keepalive.Stop()

	for {
		select {
		case progress, ok := <-ch:
			if !ok {
				// @cpt-begin:cpt-katapult-flow-observability-stream-progress:p1:inst-stream-complete
				return
				// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-stream-complete
			}
			writeSSEEvent(w, "progress", progress)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-return-stream
	// @cpt-end:cpt-katapult-flow-observability-stream-progress:p1:inst-push-sse
}

func writeSSEEvent(w http.ResponseWriter, eventType string, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload)
}
