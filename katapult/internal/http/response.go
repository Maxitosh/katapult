package http

import (
	"encoding/json"
	"net/http"
)

// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// listResponse wraps a paginated list with metadata.
type listResponse struct {
	Items      any            `json:"items"`
	Pagination paginationMeta `json:"pagination"`
}

type paginationMeta struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}
