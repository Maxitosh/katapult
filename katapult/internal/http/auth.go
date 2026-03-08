// Package http provides HTTP middleware and handlers for the Katapult REST API.
// @cpt-dod:cpt-katapult-dod-api-cli-auth:p1
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Role represents an authorization role for API access.
// @cpt-dod:cpt-katapult-dod-api-cli-auth:p1
type Role string

const (
	// RoleOperator grants full read-write access to all API endpoints.
	RoleOperator Role = "operator"
	// RoleViewer grants read-only access to API endpoints.
	RoleViewer Role = "viewer"
)

// UserInfo holds the authenticated identity and role extracted from a token.
// @cpt-dod:cpt-katapult-dod-api-cli-auth:p1
type UserInfo struct {
	Subject string
	Role    Role
}

// TokenValidator validates bearer tokens and returns the associated user info.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*UserInfo, error)
}

// StaticTokenValidator validates tokens against a pre-configured map.
type StaticTokenValidator struct {
	tokens map[string]UserInfo
}

// NewStaticTokenValidator creates a validator backed by a static token-to-user map.
func NewStaticTokenValidator(tokens map[string]UserInfo) *StaticTokenValidator {
	m := make(map[string]UserInfo, len(tokens))
	for k, v := range tokens {
		m[k] = v
	}
	return &StaticTokenValidator{tokens: m}
}

// ValidateToken checks whether the token exists in the static map.
func (v *StaticTokenValidator) ValidateToken(_ context.Context, token string) (*UserInfo, error) {
	u, ok := v.tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return &u, nil
}

type userInfoKeyType struct{}

var userInfoKey = userInfoKeyType{}

// AuthMiddleware returns HTTP middleware that extracts and validates a Bearer
// token from the Authorization header, storing the resulting UserInfo in the
// request context.
// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-authenticate
func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := extractBearerToken(r)
			if !ok {
				writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "missing or malformed Authorization header")
				return
			}

			user, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
				return
			}

			ctx := context.WithValue(r.Context(), userInfoKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-authenticate

// RequireRole returns HTTP middleware that enforces the caller has the required
// role. An operator role satisfies any role requirement. A viewer role only
// satisfies viewer requirements.
// @cpt-begin:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-authorize
func RequireRole(role Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				writeErrorJSON(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}

			if !hasRole(user.Role, role) {
				writeErrorJSON(w, http.StatusForbidden, "forbidden", fmt.Sprintf("role %q required", role))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// @cpt-end:cpt-katapult-flow-api-cli-create-transfer-api:p1:inst-authorize

// UserFromContext extracts the authenticated UserInfo from a context.
// Returns nil when no user is present.
func UserFromContext(ctx context.Context) *UserInfo {
	u, _ := ctx.Value(userInfoKey).(*UserInfo)
	return u
}

// hasRole checks whether the actual role satisfies the required role.
// Operator satisfies any requirement; viewer satisfies only viewer.
func hasRole(actual, required Role) bool {
	if actual == RoleOperator {
		return true
	}
	return actual == required
}

// extractBearerToken pulls the Bearer token from the Authorization header.
func extractBearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", false
	}
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return "", false
	}
	return token, true
}

// apiError is the standard error envelope for REST API responses.
type apiError struct {
	Error apiErrorDetail `json:"error"`
}

type apiErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeErrorJSON(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiError{
		Error: apiErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}
