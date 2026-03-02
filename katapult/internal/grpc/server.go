package grpc

import (
	"context"
	"strings"

	"github.com/maxitosh/katapult/internal/domain"
	"github.com/maxitosh/katapult/internal/registry"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
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

	agentID, err := s.registrySvc.RegisterAgent(ctx, req.ClusterId, req.NodeName, tools, pvcs, jwtNamespace)
	if err != nil {
		if strings.Contains(err.Error(), "namespace mismatch") {
			return nil, status.Errorf(codes.PermissionDenied, "registration failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "registration failed: %v", err)
	}

	return &pb.RegisterResponse{AgentId: agentID.String()}, nil
}

// Heartbeat handles agent heartbeat messages.
func (s *AgentServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	agentID, err := uuid.Parse(req.AgentId)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid agent_id: %v", err)
	}

	// Verify JWT namespace matches the agent's registered namespace.
	if claims, ok := ClaimsFromContext(ctx); ok && claims.Kubernetes != nil {
		agent, err := s.registrySvc.GetAgent(ctx, agentID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "looking up agent: %v", err)
		}
		if agent == nil {
			return nil, status.Errorf(codes.NotFound, "agent %s not found", agentID)
		}
		if agent.JWTNamespace != "" && claims.Kubernetes.Namespace != agent.JWTNamespace {
			return nil, status.Errorf(codes.PermissionDenied, "JWT namespace mismatch for agent %s", agentID)
		}
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
		if strings.Contains(err.Error(), "must re-register") {
			return nil, status.Errorf(codes.FailedPrecondition, "heartbeat failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "heartbeat failed: %v", err)
	}

	return &pb.HeartbeatResponse{}, nil
}
