package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
)

func TestRegister_Success(t *testing.T) {
	srv := setupTestServer(t)
	ctx := contextWithClaims("katapult-ns")

	resp, err := srv.Register(ctx, &pb.RegisterRequest{
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
	if s, ok := status.FromError(err); ok {
		if s.Code() != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", s.Code())
		}
	}
}

func TestRegister_InvalidTools(t *testing.T) {
	srv := setupTestServer(t)
	ctx := contextWithClaims("katapult-ns")

	_, err := srv.Register(ctx, &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.20", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err == nil {
		t.Fatal("expected error for insufficient tar version")
	}
}

func TestRegister_WithClaims(t *testing.T) {
	srv := setupTestServer(t)
	ctx := contextWithClaims("production-ns")

	resp, err := srv.Register(ctx, &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AgentId == "" {
		t.Fatal("expected non-empty agent_id")
	}

	// Re-register with different namespace should fail.
	ctx2 := contextWithClaims("other-ns")
	_, err = srv.Register(ctx2, &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err == nil {
		t.Fatal("expected error for namespace mismatch")
	}
	if s, ok := status.FromError(err); ok {
		if s.Code() != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", s.Code())
		}
	}
}

func TestHeartbeat_Success(t *testing.T) {
	srv := setupTestServer(t)
	ctx := contextWithClaims("katapult-ns")

	resp, err := srv.Register(ctx, &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	_, err = srv.Heartbeat(ctx, &pb.HeartbeatRequest{
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
	if s, ok := status.FromError(err); ok {
		if s.Code() != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", s.Code())
		}
	}
}

func TestHeartbeat_JWTMismatch(t *testing.T) {
	srv := setupTestServer(t)
	ctx := contextWithClaims("katapult-ns")

	// Register agent with namespace "katapult-ns".
	resp, err := srv.Register(ctx, &pb.RegisterRequest{
		ClusterId: "cluster-a",
		NodeName:  "node-1",
		Tools:     &pb.ToolVersions{TarVersion: "1.35", ZstdVersion: "1.5.5", StunnelVersion: "5.72"},
	})
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Heartbeat with different namespace should fail.
	ctxOther := contextWithClaims("other-ns")
	_, err = srv.Heartbeat(ctxOther, &pb.HeartbeatRequest{
		AgentId: resp.AgentId,
	})
	if err == nil {
		t.Fatal("expected error for JWT namespace mismatch on heartbeat")
	}
	if s, ok := status.FromError(err); ok {
		if s.Code() != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", s.Code())
		}
	}
}
