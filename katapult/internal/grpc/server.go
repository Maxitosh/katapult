package grpc

import (
	"context"
	"strings"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/maxitosh/katapult/internal/transfer"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
)

// AgentServer implements the AgentService gRPC server.
type AgentServer struct {
	pb.UnimplementedAgentServiceServer
	registrySvc  *registry.Service
	orchestrator *transfer.Orchestrator
}

// NewAgentServer creates a new gRPC server for agent communication.
func NewAgentServer(registrySvc *registry.Service, orchestrator *transfer.Orchestrator) *AgentServer {
	return &AgentServer{registrySvc: registrySvc, orchestrator: orchestrator}
}

// Register handles agent registration.
// @cpt-flow:cpt-katapult-flow-agent-system-register:p1
// @cpt-dod:cpt-katapult-dod-agent-system-registration:p1
func (s *AgentServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.ClusterId == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_id is required")
	}
	if req.NodeName == "" {
		return nil, status.Error(codes.InvalidArgument, "node_name is required")
	}

	jwtNamespace := ""
	if claims, ok := ClaimsFromContext(ctx); ok && claims.Kubernetes != nil {
		jwtNamespace = claims.Kubernetes.Namespace
	}

	tools := domain.ToolVersions{}
	if req.Tools != nil {
		tools.Tar = req.Tools.TarVersion
		tools.Zstd = req.Tools.ZstdVersion
		tools.Stunnel = req.Tools.StunnelVersion
	}

	pvcs := make([]domain.PVCInfo, 0, len(req.Pvcs))
	for _, p := range req.Pvcs {
		pvcs = append(pvcs, domain.PVCInfo{
			PVCName:      p.PvcName,
			SizeBytes:    p.SizeBytes,
			StorageClass: p.StorageClass,
			NodeAffinity: p.NodeAffinity,
		})
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-validate-reg
	agentID, err := s.registrySvc.RegisterAgent(ctx, req.ClusterId, req.NodeName, tools, pvcs, jwtNamespace)
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-validate-reg
	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-reject-reg
	if err != nil {
		if strings.Contains(err.Error(), "namespace mismatch") {
			return nil, status.Errorf(codes.PermissionDenied, "registration failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "registration failed: %v", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-reject-reg

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-return-registered
	return &pb.RegisterResponse{AgentId: agentID.String()}, nil
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-return-registered
}

// Heartbeat handles agent heartbeat messages.
// @cpt-flow:cpt-katapult-flow-agent-system-heartbeat:p1
func (s *AgentServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent_id: %v", err)
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-verify-heartbeat-auth
	// Verify JWT namespace matches the agent's registered namespace.
	if claims, ok := ClaimsFromContext(ctx); ok && claims.Kubernetes != nil {
		agent, err := s.registrySvc.GetAgent(ctx, agentID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "looking up agent: %v", err)
		}
		if agent == nil {
			return nil, status.Errorf(codes.NotFound, "agent %s not found", agentID)
		}
		// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-reject-heartbeat
		if agent.JWTNamespace != "" && claims.Kubernetes.Namespace != agent.JWTNamespace {
			return nil, status.Errorf(codes.PermissionDenied, "JWT namespace mismatch for agent %s", agentID)
		}
		// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-reject-heartbeat
	}
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-verify-heartbeat-auth

	pvcs := make([]domain.PVCInfo, 0, len(req.Pvcs))
	for _, p := range req.Pvcs {
		pvcs = append(pvcs, domain.PVCInfo{
			PVCName:      p.PvcName,
			SizeBytes:    p.SizeBytes,
			StorageClass: p.StorageClass,
			NodeAffinity: p.NodeAffinity,
		})
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-update-heartbeat
	if err := s.registrySvc.Heartbeat(ctx, agentID, pvcs); err != nil {
		if strings.Contains(err.Error(), "must re-register") {
			return nil, status.Errorf(codes.FailedPrecondition, "heartbeat failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "heartbeat failed: %v", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-db-update-heartbeat

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
	return &pb.HeartbeatResponse{}, nil
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
}

// CreateTransfer handles transfer creation requests from operators.
// @cpt-flow:cpt-katapult-flow-transfer-engine-initiate:p1
func (s *AgentServer) CreateTransfer(ctx context.Context, req *pb.CreateTransferRequest) (*pb.CreateTransferResponse, error) {
	if s.orchestrator == nil {
		return nil, status.Error(codes.Unimplemented, "transfer engine not configured")
	}
	if req.SourceCluster == "" {
		return nil, status.Error(codes.InvalidArgument, "source_cluster is required")
	}
	if req.SourcePvc == "" {
		return nil, status.Error(codes.InvalidArgument, "source_pvc is required")
	}
	if req.DestinationCluster == "" {
		return nil, status.Error(codes.InvalidArgument, "destination_cluster is required")
	}
	if req.DestinationPvc == "" {
		return nil, status.Error(codes.InvalidArgument, "destination_pvc is required")
	}

	createReq := transfer.CreateTransferRequest{
		SourceCluster:      req.SourceCluster,
		SourcePVC:          req.SourcePvc,
		DestinationCluster: req.DestinationCluster,
		DestinationPVC:     req.DestinationPvc,
		AllowOverwrite:     req.AllowOverwrite,
		CreatedBy:          req.CreatedBy,
	}
	if req.StrategyOverride != nil {
		override := *req.StrategyOverride
		createReq.StrategyOverride = &override
	}
	if req.RetryMax != nil {
		retryMax := int(*req.RetryMax)
		createReq.RetryMax = &retryMax
	}

	resp, err := s.orchestrator.CreateTransfer(ctx, createReq)
	if err != nil {
		if strings.Contains(err.Error(), "validation failed") {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "creating transfer: %v", err)
	}

	return &pb.CreateTransferResponse{
		TransferId: resp.TransferID.String(),
		State:      string(resp.State),
	}, nil
}

// CancelTransfer handles transfer cancellation requests from operators.
// @cpt-flow:cpt-katapult-flow-transfer-engine-cancel:p1
func (s *AgentServer) CancelTransfer(ctx context.Context, req *pb.CancelTransferRequest) (*pb.CancelTransferResponse, error) {
	if s.orchestrator == nil {
		return nil, status.Error(codes.Unimplemented, "transfer engine not configured")
	}
	transferID, err := uuid.Parse(req.TransferId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transfer_id: %v", err)
	}

	if err := s.orchestrator.CancelTransfer(ctx, transferID); err != nil {
		if strings.Contains(err.Error(), "terminal state") {
			return nil, status.Errorf(codes.FailedPrecondition, "%v", err)
		}
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "cancelling transfer: %v", err)
	}

	return &pb.CancelTransferResponse{
		TransferId: transferID.String(),
		State:      string(domain.TransferStateCancelled),
	}, nil
}

// ReportProgress handles progress reports from agents.
// @cpt-flow:cpt-katapult-flow-transfer-engine-report-progress:p1
func (s *AgentServer) ReportProgress(ctx context.Context, req *pb.ReportProgressRequest) (*pb.ReportProgressResponse, error) {
	if s.orchestrator == nil {
		return nil, status.Error(codes.Unimplemented, "transfer engine not configured")
	}
	transferID, err := uuid.Parse(req.TransferId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid transfer_id: %v", err)
	}

	if err := s.orchestrator.HandleProgress(ctx, transfer.ProgressReport{
		TransferID:       transferID,
		BytesTransferred: req.BytesTransferred,
		BytesTotal:       req.BytesTotal,
		Speed:            req.Speed,
		ChunksCompleted:  int(req.ChunksCompleted),
		ChunksTotal:      int(req.ChunksTotal),
		Status:           req.Status,
		ErrorMessage:     req.ErrorMessage,
	}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, status.Errorf(codes.NotFound, "%v", err)
		}
		return nil, status.Errorf(codes.Internal, "reporting progress: %v", err)
	}

	return &pb.ReportProgressResponse{}, nil
}
