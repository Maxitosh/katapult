package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// @cpt-dod:cpt-katapult-dod-api-cli-cli-tool:p1
// @cpt-algo:cpt-katapult-algo-api-cli-resolve-cli-command:p1

// APIClient communicates with the Katapult REST API.
type APIClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewAPIClient creates a new API client.
func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{},
	}
}

// @cpt-begin:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-construct-http
func (c *APIClient) do(method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// @cpt-end:cpt-katapult-algo-api-cli-resolve-cli-command:p1:inst-construct-http

// CreateTransfer sends POST /api/v1alpha1/transfers.
// @cpt-flow:cpt-katapult-flow-api-cli-create-transfer-cli:p1
func (c *APIClient) CreateTransfer(input map[string]any) ([]byte, int, error) {
	return c.do("POST", "/api/v1alpha1/transfers", input)
}

// ListTransfers sends GET /api/v1alpha1/transfers with query params.
func (c *APIClient) ListTransfers(query string) ([]byte, int, error) {
	path := "/api/v1alpha1/transfers"
	if query != "" {
		path += "?" + query
	}
	return c.do("GET", path, nil)
}

// GetTransfer sends GET /api/v1alpha1/transfers/{id}.
func (c *APIClient) GetTransfer(id string) ([]byte, int, error) {
	return c.do("GET", "/api/v1alpha1/transfers/"+id, nil)
}

// CancelTransfer sends DELETE /api/v1alpha1/transfers/{id}.
func (c *APIClient) CancelTransfer(id string) ([]byte, int, error) {
	return c.do("DELETE", "/api/v1alpha1/transfers/"+id, nil)
}

// ListAgents sends GET /api/v1alpha1/agents with query params.
func (c *APIClient) ListAgents(query string) ([]byte, int, error) {
	path := "/api/v1alpha1/agents"
	if query != "" {
		path += "?" + query
	}
	return c.do("GET", path, nil)
}

// GetAgent sends GET /api/v1alpha1/agents/{id}.
func (c *APIClient) GetAgent(id string) ([]byte, int, error) {
	return c.do("GET", "/api/v1alpha1/agents/"+id, nil)
}
