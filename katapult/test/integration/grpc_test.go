//go:build integration

package integration_test

import (
	"context"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"github.com/maxitosh/katapult/internal/domain"
	katapultgrpc "github.com/maxitosh/katapult/internal/grpc"
	"github.com/maxitosh/katapult/internal/registry"
	pgstore "github.com/maxitosh/katapult/internal/store/postgres"
	"github.com/maxitosh/katapult/internal/transfer"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-component-tests:p2
// @cpt-flow:cpt-katapult-flow-integration-tests-run-component-tests:p2

// @cpt-begin:cpt-katapult-dod-integration-tests-component-tests:p2:inst-grpc-tests

// testCommander is a mock AgentCommander that always succeeds.
type testCommander struct{}

func (testCommander) IsPVCEmpty(_ context.Context, _, _ string) (bool, error) { return true, nil }
func (testCommander) SendTransferCommand(_ context.Context, _, _ string, _ any) error {
	return nil
}
func (testCommander) SendCancelCommand(_ context.Context, _, _ string) error { return nil }

// testPVCFinder always finds PVCs.
type testPVCFinder struct{}

func (testPVCFinder) FindHealthyPVC(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

// startTestGRPCServer wires real repos and starts a gRPC server on a random port.
// Returns the client connection and cleanup function.
func startTestGRPCServer(t *testing.T) (pb.AgentServiceClient, func()) {
	t.Helper()
	pool := getTestPool(t)
	logger := slog.Default()

	agentRepo := pgstore.NewAgentRepository(pool)
	transferRepo := pgstore.NewTransferRepository(pool)

	registrySvc := registry.NewService(agentRepo, logger)

	cmd := testCommander{}
	validator := transfer.NewValidator(testPVCFinder{}, cmd, logger)
	cleaner := transfer.NewCleaner(transfer.NoopCredentialManager{}, cmd, transfer.NoopS3Client{}, logger)
	orchestrator := transfer.NewOrchestrator(
		transferRepo, validator, cleaner, cmd,
		transfer.NoopCredentialManager{}, transfer.S3Config{},
		domain.DefaultTransferConfig(), logger,
	)

	srv := grpc.NewServer()
	agentServer := katapultgrpc.NewAgentServer(registrySvc, orchestrator)
	pb.RegisterAgentServiceServer(srv, agentServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	go func() {
		if err := srv.Serve(lis); err != nil {
			// server stopped
		}
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		srv.GracefulStop()
		t.Fatalf("dialing grpc: %v", err)
	}

	client := pb.NewAgentServiceClient(conn)

	cleanup := func() {
		conn.Close()
		srv.GracefulStop()
	}

	return client, cleanup
}

func TestGRPC_Register_NewAgent(t *testing.T) {
	client, cleanup := startTestGRPCServer(t)
	defer cleanup()

	ctx := context.Background()
	resp, err := client.Register(ctx, &pb.RegisterRequest{
		ClusterId: "grpc-test-cluster",
		NodeName:  "grpc-node-" + uuid.New().String()[:8],
		Tools: &pb.ToolVersions{
			TarVersion:     "1.35",
			ZstdVersion:    "1.5.5",
			StunnelVersion: "5.72",
		},
		Pvcs: []*pb.PVCInfo{
			{PvcName: "default/test-pvc", SizeBytes: 1024, StorageClass: "standard", NodeAffinity: "node-1"},
		},
	})
	if err != nil {
		t.Fatalf("Register RPC failed: %v", err)
	}
	if resp.AgentId == "" {
		t.Fatal("expected non-empty agent_id")
	}

	// Verify agent is in DB.
	agentID, err := uuid.Parse(resp.AgentId)
	if err != nil {
		t.Fatalf("parsing agent_id: %v", err)
	}

	pool := getTestPool(t)
	agentRepo := pgstore.NewAgentRepository(pool)
	agent, err := agentRepo.GetAgentByID(ctx, agentID)
	if err != nil {
		t.Fatalf("getting agent from DB: %v", err)
	}
	if agent == nil {
		t.Fatal("agent not found in DB after registration")
	}
	if agent.State != domain.AgentStateHealthy {
		t.Fatalf("expected agent state %q, got %q", domain.AgentStateHealthy, agent.State)
	}
}

func TestGRPC_Heartbeat_UpdatesTimestamp(t *testing.T) {
	client, cleanup := startTestGRPCServer(t)
	defer cleanup()

	ctx := context.Background()

	// Register agent first.
	nodeName := "hb-node-" + uuid.New().String()[:8]
	regResp, err := client.Register(ctx, &pb.RegisterRequest{
		ClusterId: "grpc-test-cluster",
		NodeName:  nodeName,
		Tools: &pb.ToolVersions{
			TarVersion:     "1.35",
			ZstdVersion:    "1.5.5",
			StunnelVersion: "5.72",
		},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Record time before heartbeat.
	beforeHB := time.Now().Add(-1 * time.Second)

	// Send heartbeat.
	_, err = client.Heartbeat(ctx, &pb.HeartbeatRequest{
		AgentId: regResp.AgentId,
		Pvcs: []*pb.PVCInfo{
			{PvcName: "default/new-pvc", SizeBytes: 2048, StorageClass: "ssd", NodeAffinity: nodeName},
		},
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	// Verify last_seen updated in DB.
	agentID, _ := uuid.Parse(regResp.AgentId)
	pool := getTestPool(t)
	agentRepo := pgstore.NewAgentRepository(pool)
	agent, err := agentRepo.GetAgentByID(ctx, agentID)
	if err != nil {
		t.Fatalf("getting agent: %v", err)
	}
	if agent.LastHeartbeat.Before(beforeHB) {
		t.Fatalf("expected last_heartbeat after %v, got %v", beforeHB, agent.LastHeartbeat)
	}
	if len(agent.PVCs) != 1 {
		t.Fatalf("expected 1 PVC after heartbeat, got %d", len(agent.PVCs))
	}
}

func TestGRPC_Transfer_Lifecycle(t *testing.T) {
	client, cleanup := startTestGRPCServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create transfer.
	resp, err := client.CreateTransfer(ctx, &pb.CreateTransferRequest{
		SourceCluster:      "grpc-test-cluster",
		SourcePvc:          "ns/src-pvc",
		DestinationCluster: "grpc-test-cluster",
		DestinationPvc:     "ns/dst-pvc",
		CreatedBy:          "integration-test",
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}
	if resp.TransferId == "" {
		t.Fatal("expected non-empty transfer_id")
	}
	if resp.State != string(domain.TransferStateTransferring) {
		t.Fatalf("expected state %q, got %q", domain.TransferStateTransferring, resp.State)
	}

	// Verify transfer in DB.
	transferID, _ := uuid.Parse(resp.TransferId)
	pool := getTestPool(t)
	transferRepo := pgstore.NewTransferRepository(pool)
	tr, err := transferRepo.GetTransferByID(ctx, transferID)
	if err != nil {
		t.Fatalf("getting transfer from DB: %v", err)
	}
	if tr == nil {
		t.Fatal("transfer not found in DB")
	}
	if tr.State != domain.TransferStateTransferring {
		t.Fatalf("expected DB state %q, got %q", domain.TransferStateTransferring, tr.State)
	}
}

func TestGRPC_ReportProgress_UpdatesTransfer(t *testing.T) {
	client, cleanup := startTestGRPCServer(t)
	defer cleanup()

	ctx := context.Background()

	// Create a transfer first.
	createResp, err := client.CreateTransfer(ctx, &pb.CreateTransferRequest{
		SourceCluster:      "grpc-test-cluster",
		SourcePvc:          "ns/progress-src",
		DestinationCluster: "grpc-test-cluster",
		DestinationPvc:     "ns/progress-dst",
		CreatedBy:          "integration-test",
	})
	if err != nil {
		t.Fatalf("CreateTransfer: %v", err)
	}

	// Report progress.
	_, err = client.ReportProgress(ctx, &pb.ReportProgressRequest{
		TransferId:       createResp.TransferId,
		BytesTransferred: 500,
		BytesTotal:       1000,
		Speed:            100.0,
		ChunksCompleted:  2,
		ChunksTotal:      4,
		Status:           "in_progress",
	})
	if err != nil {
		t.Fatalf("ReportProgress: %v", err)
	}

	// Verify progress in DB.
	transferID, _ := uuid.Parse(createResp.TransferId)
	pool := getTestPool(t)
	transferRepo := pgstore.NewTransferRepository(pool)
	tr, err := transferRepo.GetTransferByID(ctx, transferID)
	if err != nil {
		t.Fatalf("getting transfer: %v", err)
	}
	if tr.BytesTransferred != 500 {
		t.Fatalf("expected bytes_transferred 500, got %d", tr.BytesTransferred)
	}
	if tr.BytesTotal != 1000 {
		t.Fatalf("expected bytes_total 1000, got %d", tr.BytesTotal)
	}
	if tr.ChunksCompleted != 2 {
		t.Fatalf("expected chunks_completed 2, got %d", tr.ChunksCompleted)
	}
}

// @cpt-end:cpt-katapult-dod-integration-tests-component-tests:p2:inst-grpc-tests
