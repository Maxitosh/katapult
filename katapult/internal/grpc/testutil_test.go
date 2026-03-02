package grpc

import (
	"log/slog"
	"testing"

	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/testutil"
)

func setupTestServer(t *testing.T) *AgentServer {
	t.Helper()
	repo := testutil.NewMemRepo()
	svc := registry.NewService(repo, slog.Default())
	return NewAgentServer(svc)
}
