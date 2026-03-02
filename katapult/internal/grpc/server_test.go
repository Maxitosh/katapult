package grpc

import (
	"context"
	"log/slog"
	"testing"

	"github.com/chainstack/katapult/internal/registry"

	pb "github.com/chainstack/katapult/api/proto/agent/v1alpha1"
)

func setupTestServer(t *testing.T) *AgentServer {
	t.Helper()
	repo := newMemRepo()
	svc := registry.NewService(repo, slog.Default())
	return NewAgentServer(svc)
}

func TestRegister_Success(t *testing.T) {
	srv := setupTestServer(t)

	resp, err := srv.Register(context.Background(), &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
		Pvcs: []*pb.PVCInfo{
			{PvcName: "ns/pvc1", SizeBytes: 1024, StorageClass: "local", NodeAffinity: "node-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AgentId == "" {
		t.Fatal("expected non-empty agent_id")
	}
}

func TestRegister_MissingClusterID(t *testing.T) {
	srv := setupTestServer(t)

	_, err := srv.Register(context.Background(), &pb.RegisterRequest{
		NodeName: "node-1",
		Tools:    &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err == nil {
		t.Fatal("expected error for missing cluster_id")
	}
}

func TestRegister_InvalidTools(t *testing.T) {
	srv := setupTestServer(t)

	_, err := srv.Register(context.Background(), &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.20", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err == nil {
		t.Fatal("expected error for insufficient tar version")
	}
}

func TestHeartbeat_Success(t *testing.T) {
	srv := setupTestServer(t)

	resp, _ := srv.Register(context.Background(), &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})

	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		AgentId: resp.AgentId,
		Pvcs: []*pb.PVCInfo{
			{PvcName: "ns/pvc-new", SizeBytes: 2048, StorageClass: "local", NodeAffinity: "node-1"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHeartbeat_InvalidAgentID(t *testing.T) {
	srv := setupTestServer(t)

	_, err := srv.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		AgentId: "not-a-uuid",
	})
	if err == nil {
		t.Fatal("expected error for invalid agent_id")
	}
}
