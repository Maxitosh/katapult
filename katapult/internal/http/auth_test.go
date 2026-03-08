package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubValidator is a test double for TokenValidator.
type stubValidator struct {
	tokens map[string]UserInfo
}

func (v *stubValidator) ValidateToken(_ context.Context, token string) (*UserInfo, error) {
	u, ok := v.tokens[token]
	if !ok {
		return nil, errInvalidToken
	}
	return &u, nil
}

var errInvalidToken = &tokenError{msg: "invalid token"}

type tokenError struct{ msg string }

func (e *tokenError) Error() string { return e.msg }

func newTestValidator() *stubValidator {
	return &stubValidator{
		tokens: map[string]UserInfo{
			"op-token":     {Subject: "alice", Role: RoleOperator},
			"viewer-token": {Subject: "bob", Role: RoleViewer},
		},
	}
}

// echoUserHandler writes the authenticated user's subject and role into the response.
func echoUserHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"subject": user.Subject,
			"role":    string(user.Role),
		})
	})
}

func TestAuthMiddleware(t *testing.T) {
	validator := newTestValidator()
	mw := AuthMiddleware(validator)
	handler := mw(echoUserHandler())

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing authorization header returns 401",
			authHeader: "",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "malformed authorization header returns 401",
			authHeader: "Token abc",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "invalid token returns 401",
			authHeader: "Bearer bad-token",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name:       "valid operator token returns 200",
			authHeader: "Bearer op-token",
			wantStatus: http.StatusOK,
		},
		{
			name:       "valid viewer token returns 200",
			authHeader: "Bearer viewer-token",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantCode != "" {
				var body apiError
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode error body: %v", err)
				}
				if body.Error.Code != tt.wantCode {
					t.Fatalf("error code = %q, want %q", body.Error.Code, tt.wantCode)
				}
			}
		})
	}
}

func TestAuthMiddleware_SetsUserInfo(t *testing.T) {
	validator := newTestValidator()
	mw := AuthMiddleware(validator)

	var captured *UserInfo
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = UserFromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer op-token")
	rr := httptest.NewRecorder()

	mw(inner).ServeHTTP(rr, req)

	if captured == nil {
		t.Fatal("expected UserInfo in context, got nil")
	}
	if captured.Subject != "alice" {
		t.Fatalf("subject = %q, want %q", captured.Subject, "alice")
	}
	if captured.Role != RoleOperator {
		t.Fatalf("role = %q, want %q", captured.Role, RoleOperator)
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name         string
		userRole     Role
		requiredRole Role
		wantStatus   int
	}{
		{
			name:         "operator allowed for operator endpoint",
			userRole:     RoleOperator,
			requiredRole: RoleOperator,
			wantStatus:   http.StatusOK,
		},
		{
			name:         "viewer denied for operator endpoint",
			userRole:     RoleViewer,
			requiredRole: RoleOperator,
			wantStatus:   http.StatusForbidden,
		},
		{
			name:         "operator allowed for viewer endpoint",
			userRole:     RoleOperator,
			requiredRole: RoleViewer,
			wantStatus:   http.StatusOK,
		},
		{
			name:         "viewer allowed for viewer endpoint",
			userRole:     RoleViewer,
			requiredRole: RoleViewer,
			wantStatus:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := RequireRole(tt.requiredRole)(inner)

			user := &UserInfo{Subject: "test-user", Role: tt.userRole}
			ctx := context.WithValue(context.Background(), userInfoKey, user)
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantStatus == http.StatusForbidden {
				var body apiError
				if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode error body: %v", err)
				}
				if body.Error.Code != "forbidden" {
					t.Fatalf("error code = %q, want %q", body.Error.Code, "forbidden")
				}
			}
		})
	}
}

func TestRequireRole_NoUserInContext(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireRole(RoleViewer)(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestUserFromContext(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		user := &UserInfo{Subject: "alice", Role: RoleOperator}
		ctx := context.WithValue(context.Background(), userInfoKey, user)

		got := UserFromContext(ctx)
		if got == nil {
			t.Fatal("expected user, got nil")
		}
		if got.Subject != "alice" {
			t.Fatalf("subject = %q, want %q", got.Subject, "alice")
		}
	})

	t.Run("absent", func(t *testing.T) {
		got := UserFromContext(context.Background())
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})
}

func TestStaticTokenValidator(t *testing.T) {
	tokens := map[string]UserInfo{
		"secret": {Subject: "admin", Role: RoleOperator},
	}
	v := NewStaticTokenValidator(tokens)

	t.Run("valid token", func(t *testing.T) {
		user, err := v.ValidateToken(context.Background(), "secret")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.Subject != "admin" {
			t.Fatalf("subject = %q, want %q", user.Subject, "admin")
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := v.ValidateToken(context.Background(), "wrong")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})

	t.Run("mutation safety", func(t *testing.T) {
		tokens["injected"] = UserInfo{Subject: "hacker", Role: RoleOperator}
		_, err := v.ValidateToken(context.Background(), "injected")
		if err == nil {
			t.Fatal("external map mutation should not affect validator")
		}
	})
}
