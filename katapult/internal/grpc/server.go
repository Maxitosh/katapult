package grpc

import (
	"context"
	"fmt"

	"github.com/chainstack/katapult/internal/domain"
	"github.com/chainstack/katapult/internal/registry"
	"github.com/google/uuid"

	pb "github.com/chainstack/katapult/api/proto/agent/v1alpha1"
)

// AgentServer implements the AgentService gRPC server.
type AgentServer struct {
	pb.UnimplementedAgentServiceServer
	registrySvc *registry.Service
}

// NewAgentServer creates a new gRPC server for agent communication.
func NewAgentServer(registrySvc *registry.Service) *AgentServer {
	return &AgentServer{registrySvc: registrySvc}
}

// Register handles agent registration.
func (s *AgentServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.ClusterId == "" {
		return nil, fmt.Errorf("cluster_id is required")
	}
	if req.NodeName == "" {
		return nil, fmt.Errorf("node_name is required")
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

	agentID, err := s.registrySvc.RegisterAgent(ctx, req.ClusterId, req.NodeName, tools, pvcs)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	return &pb.RegisterResponse{AgentId: agentID.String()}, nil
}

// Heartbeat handles agent heartbeat messages.
func (s *AgentServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, fmt.Errorf("invalid agent_id: %w", err)
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

	if err := s.registrySvc.Heartbeat(ctx, agentID, pvcs); err != nil {
		return nil, fmt.Errorf("heartbeat failed: %w", err)
	}

	return &pb.HeartbeatResponse{}, nil
}
