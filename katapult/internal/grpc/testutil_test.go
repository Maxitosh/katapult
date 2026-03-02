package grpc

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/testutil"
)

func setupTestServer(t *testing.T) *AgentServer {
	t.Helper()
	repo := testutil.NewMemRepo()
	svc := registry.NewService(repo, slog.Default())
	return NewAgentServer(svc)
}

// contextWithClaims returns a context with JWT claims for the given namespace.
func contextWithClaims(namespace string) context.Context {
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Kubernetes: &KubernetesClaims{
			Namespace: namespace,
			ServiceAccount: &ServiceAccountID{
				Name: "katapult-agent",
				UID:  "test-uid",
			},
		},
	}
	return context.WithValue(context.Background(), claimsKey{}, claims)
}
