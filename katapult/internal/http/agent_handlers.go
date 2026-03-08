package http

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/maxitosh/katapult/internal/domain"
)

// @cpt-dod:cpt-katapult-dod-api-cli-rest-agent-endpoints:p1

// handleListAgents handles GET /api/v1alpha1/agents.
// @cpt-flow:cpt-katapult-flow-api-cli-list-agents:p1
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-query-agents
	filter := domain.AgentFilter{}

	if v := r.URL.Query().Get("cluster"); v != "" {
		filter.ClusterID = &v
	}
	if v := r.URL.Query().Get("state"); v != "" {
		state := domain.AgentState(v)
		filter.State = &state
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
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-query-agents

	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-delegate-agents
	agents, total, err := s.registrySvc.ListAgents(r.Context(), filter)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-delegate-agents

	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-return-agents
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	writeJSON(w, http.StatusOK, listResponse{
		Items: agents,
		Pagination: paginationMeta{
			Total:  total,
			Limit:  limit,
			Offset: filter.Offset,
		},
	})
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-return-agents
}

// handleGetAgent handles GET /api/v1alpha1/agents/{id}.
// @cpt-flow:cpt-katapult-flow-api-cli-list-agents:p1
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid agent ID format")
		return
	}

	agent, err := s.registrySvc.GetAgent(r.Context(), id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	if agent == nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}

	writeJSON(w, http.StatusOK, agent)
}

// handleGetAgentPVCs handles GET /api/v1alpha1/agents/{id}/pvcs.
// @cpt-flow:cpt-katapult-flow-api-cli-list-agents:p1
func (s *Server) handleGetAgentPVCs(w http.ResponseWriter, r *http.Request) {
	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-query-pvcs
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeErrorJSON(w, http.StatusBadRequest, "invalid_id", "invalid agent ID format")
		return
	}

	agent, err := s.registrySvc.GetAgent(r.Context(), id)
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	if agent == nil {
		writeErrorJSON(w, http.StatusNotFound, "not_found", "agent not found")
		return
	}
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-query-pvcs

	// @cpt-begin:cpt-katapult-flow-api-cli-list-agents:p1:inst-return-pvcs
	writeJSON(w, http.StatusOK, map[string]any{"pvcs": agent.PVCs})
	// @cpt-end:cpt-katapult-flow-api-cli-list-agents:p1:inst-return-pvcs
}

// handleListClusters handles GET /api/v1alpha1/clusters.
// @cpt-dod:cpt-katapult-dod-api-cli-rest-agent-endpoints:p1
func (s *Server) handleListClusters(w http.ResponseWriter, r *http.Request) {
	clusters, err := s.registrySvc.ListClusters(r.Context())
	if err != nil {
		writeErrorJSON(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"clusters": clusters})
}
