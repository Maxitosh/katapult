package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/maxitosh/katapult/internal/domain"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// TLSConfig holds the TLS certificate paths for the agent client.
type TLSConfig struct {
	CACertPath    string
	ClientCertPath string
	ClientKeyPath  string
}

// Client manages communication with the control plane.
type Client struct {
	conn    *grpc.ClientConn
	agent   pb.AgentServiceClient
	agentID string
	logger  *slog.Logger
}

// NewClient creates a new gRPC client to the control plane.
// If tlsCfg is non-nil and CACertPath is set, TLS is used; otherwise insecure credentials are used.
func NewClient(address string, tlsCfg *TLSConfig, logger *slog.Logger) (*Client, error) {
	var dialOpt grpc.DialOption

	if tlsCfg != nil && tlsCfg.CACertPath != "" {
		caCert, err := os.ReadFile(tlsCfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("reading CA cert: %w", err)
		}
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA cert to pool")
		}

		tlsConf := &tls.Config{
			RootCAs: certPool,
		}

		if tlsCfg.ClientCertPath != "" && tlsCfg.ClientKeyPath != "" {
			clientCert, err := tls.LoadX509KeyPair(tlsCfg.ClientCertPath, tlsCfg.ClientKeyPath)
			if err != nil {
				return nil, fmt.Errorf("loading client cert/key: %w", err)
			}
			tlsConf.Certificates = []tls.Certificate{clientCert}
		}

		dialOpt = grpc.WithTransportCredentials(credentials.NewTLS(tlsConf))
		logger.Info("using TLS for control plane connection")
	} else {
		dialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
		logger.Warn("using insecure connection to control plane")
	}

	conn, err := grpc.NewClient(address, dialOpt)
	if err != nil {
		return nil, fmt.Errorf("connecting to control plane: %w", err)
	}
	return &Client{
		conn:   conn,
		agent:  pb.NewAgentServiceClient(conn),
		logger: logger,
	}, nil
}

// Register sends a registration request to the control plane and stores the returned agent ID.
func (c *Client) Register(ctx context.Context, clusterID, nodeName, jwtToken string, tools domain.ToolVersions, pvcs []domain.PVCInfo) (string, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+jwtToken)

	req := &pb.RegisterRequest{
		ClusterId: clusterID,
		NodeName:  nodeName,
		Tools: &pb.ToolVersions{
			TarVersion:     tools.Tar,
			ZstdVersion:    tools.Zstd,
			StunnelVersion: tools.Stunnel,
		},
		Pvcs:     toPBPVCs(pvcs),
		JwtToken: jwtToken,
	}

	resp, err := c.agent.Register(ctx, req)
	if err != nil {
		return "", fmt.Errorf("registration failed: %w", err)
	}

	c.agentID = resp.AgentId
	c.logger.Info("registered with control plane", "agent_id", c.agentID)
	return c.agentID, nil
}

// SendHeartbeat sends a heartbeat with updated PVC inventory to the control plane.
func (c *Client) SendHeartbeat(ctx context.Context, pvcs []domain.PVCInfo) error {
	if c.agentID == "" {
		return fmt.Errorf("agent not registered")
	}

	_, err := c.agent.Heartbeat(ctx, &pb.HeartbeatRequest{
		AgentId: c.agentID,
		Pvcs:    toPBPVCs(pvcs),
	})
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	return nil
}

// RunHeartbeatLoop runs the heartbeat loop, periodically sending heartbeats with
// refreshed PVC inventory. Blocks until the context is cancelled.
func (c *Client) RunHeartbeatLoop(ctx context.Context, interval time.Duration, discoverer *PVCDiscoverer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pvcs, err := discoverer.Discover(ctx)
			if err != nil {
				c.logger.Error("PVC discovery failed during heartbeat", "error", err)
				continue
			}

			if err := c.SendHeartbeat(ctx, pvcs); err != nil {
				c.logger.Error("heartbeat failed", "error", err)
			}
		}
	}
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

func toPBPVCs(pvcs []domain.PVCInfo) []*pb.PVCInfo {
	result := make([]*pb.PVCInfo, 0, len(pvcs))
	for _, p := range pvcs {
		result = append(result, &pb.PVCInfo{
			PvcName:      p.PVCName,
			SizeBytes:    p.SizeBytes,
			StorageClass: p.StorageClass,
			NodeAffinity: p.NodeAffinity,
		})
	}
	return result
}
