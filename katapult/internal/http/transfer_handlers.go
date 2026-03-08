package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/transfer"
)

// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1

// handleCreateTransfer handles POST /api/v1alpha1/transfers.
// @cpt-flow:cpt-katapult-flow-api-cli-create-transfer-api:p1
func (s *Server) handleCreateTransfer(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-validate-input
	var input createTransferInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if errs := ValidateCreateTransferInput(input); errs != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(errs)
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-validate-input

	// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-delegate-create
	user := UserFromContext(r.Context())
	createdBy := ""
	if user != nil {
		createdBy = user.Subject
	}

	req := transfer.CreateTransferRequest{
		SourceCluster:      input.SourceCluster,
		SourcePVC:          input.SourcePVC,
		DestinationCluster: input.DestinationCluster,
		DestinationPVC:     input.DestinationPVC,
		StrategyOverride:   input.Strategy,
		AllowOverwrite:     input.AllowOverwrite,
		RetryMax:           input.RetryMax,
		CreatedBy:          createdBy,
	}

	resp, err := s.orchestrator.CreateTransfer(r.Context(), req)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-delegate-create

	// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-return-created
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":     resp.TransferID,
		"state":  resp.State,
		"source": map[string]string{"cluster": input.SourceCluster, "pvc": input.SourcePVC},
		"destination": map[string]string{"cluster": input.DestinationCluster, "pvc": input.DestinationPVC},
	})
	// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-return-created
}

// handleListTransfers handles GET /api/v1alpha1/transfers.
// @cpt-flow:cpt-katapult-flow-api-cli-list-transfers:p1
func (s *Server) handleListTransfers(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-list-transfers:p1:inst-query-transfers
	filter := domain.TransferFilter{}

	if v := r.URL.Query().Get("status"); v != "" {
		state := domain.TransferState(v)
		filter.State = &state
	}
	if v := r.URL.Query().Get("cluster"); v != "" {
		filter.Cluster = &v
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	// @cpt-end:cpt-katapult-flow-api-cli-list-transfers:p1:inst-query-transfers

	// @cpt-begin:cpt-katapult-flow-api-cli-list-transfers:p1:inst-delegate-list
	transfers, total, err := s.orchestrator.ListTransfers(r.Context(), filter)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-list-transfers:p1:inst-delegate-list

	// @cpt-begin:cpt-katapult-flow-api-cli-list-transfers:p1:inst-return-list
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items: transfers,
		Pagination: paginationMeta{
			Total:  total,
			Limit:  limit,
			Offset: filter.Offset,
		},
	})
	// @cpt-end:cpt-katapult-flow-api-cli-list-transfers:p1:inst-return-list
}

// handleGetTransfer handles GET /api/v1alpha1/transfers/{id}.
// @cpt-flow:cpt-katapult-flow-api-cli-get-transfer:p1
func (s *Server) handleGetTransfer(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-get-transfer:p1:inst-query-detail
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid transfer ID format")
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-get-transfer:p1:inst-query-detail

	// @cpt-begin:cpt-katapult-flow-api-cli-get-transfer:p1:inst-delegate-get
	t, err := s.orchestrator.GetTransfer(r.Context(), id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-get-transfer:p1:inst-delegate-get

	// @cpt-begin:cpt-katapult-flow-api-cli-get-transfer:p1:inst-not-found
	if t == nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "transfer not found")
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-get-transfer:p1:inst-not-found

	// @cpt-begin:cpt-katapult-flow-api-cli-get-transfer:p1:inst-return-detail
	writeJSON(w, http.StatusOK, t)
	// @cpt-end:cpt-katapult-flow-api-cli-get-transfer:p1:inst-return-detail
}

// handleCancelTransfer handles DELETE /api/v1alpha1/transfers/{id}.
// @cpt-flow:cpt-katapult-flow-api-cli-cancel-transfer:p1
func (s *Server) handleCancelTransfer(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-request
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid transfer ID format")
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-request

	// @cpt-begin:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-delegate-cancel
	err = s.orchestrator.CancelTransfer(r.Context(), id)
	// @cpt-end:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-delegate-cancel

	if err != nil {
		// @cpt-begin:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-not-found
		if strings.Contains(err.Error(), "not found") {
			writeErrorJSON(w, http.StatusNotFound, "not_found", "transfer not found")
			return
		}
		// @cpt-end:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-not-found

		// @cpt-begin:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-conflict
		if strings.Contains(err.Error(), "terminal state") {
			writeErrorJSON(w, http.StatusConflict, "conflict", err.Error())
			return
		}
		// @cpt-end:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-cancel-conflict

		writeErrorJSON(w, http.StatusInternalServerError, "cancel_failed", err.Error())
		return
	}

	// @cpt-begin:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-return-cancelled
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    id,
		"state": domain.TransferStateCancelled,
	})
	// @cpt-end:cpt-katapult-flow-api-cli-cancel-transfer:p1:inst-return-cancelled
}

// handleGetTransferEvents handles GET /api/v1alpha1/transfers/{id}/events.
// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1
func (s *Server) handleGetTransferEvents(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid transfer ID format")
		return
	}

	events, err := s.orchestrator.GetTransferEvents(r.Context(), id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "get_events_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
