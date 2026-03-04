package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/maxitosh/katapult/internal/domain"

	pb "github.com/maxitosh/katapult/api/proto/agent/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TLSConfig holds the TLS certificate paths for the agent client.
type TLSConfig struct {
	CACertPath     string
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
// @cpt-flow:cpt-katapult-flow-agent-system-register:p1
// @cpt-dod:cpt-katapult-dod-agent-system-registration:p1
func (c *Client) Register(ctx context.Context, clusterID, nodeName, jwtToken string, tools domain.ToolVersions, pvcs []domain.PVCInfo) (string, error) {
	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-grpc-register
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
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-grpc-register

	// @cpt-begin:cpt-katapult-flow-agent-system-register:p1:inst-return-registered
	c.logger.Info("registered with control plane", "agent_id", c.agentID)
	return c.agentID, nil
	// @cpt-end:cpt-katapult-flow-agent-system-register:p1:inst-return-registered
}

// SendHeartbeat sends a heartbeat with updated PVC inventory to the control plane.
// @cpt-flow:cpt-katapult-flow-agent-system-heartbeat:p1
// @cpt-dod:cpt-katapult-dod-agent-system-heartbeat:p1
func (c *Client) SendHeartbeat(ctx context.Context, pvcs []domain.PVCInfo) error {
	if c.agentID == "" {
		return fmt.Errorf("agent not registered")
	}

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-grpc-heartbeat
	_, err := c.agent.Heartbeat(ctx, &pb.HeartbeatRequest{
		AgentId: c.agentID,
		Pvcs:    toPBPVCs(pvcs),
	})
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-grpc-heartbeat

	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
	return nil
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-return-ack
}

// RunHeartbeatLoop runs the heartbeat loop, periodically sending heartbeats with
// refreshed PVC inventory. If a heartbeat fails with a disconnect/re-register error,
// regFunc is called to re-register the agent. Blocks until the context is cancelled.
// @cpt-flow:cpt-katapult-flow-agent-system-heartbeat:p1
func (c *Client) RunHeartbeatLoop(ctx context.Context, interval time.Duration, discoverer *PVCDiscoverer, regFunc func() error) {
	// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-wait-interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-wait-interval

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// @cpt-begin:cpt-katapult-flow-agent-system-heartbeat:p1:inst-heartbeat-discover
			pvcs, err := discoverer.Discover(ctx)
			if err != nil {
				c.logger.Error("PVC discovery failed during heartbeat, sending heartbeat with empty PVCs", "error", err)
				pvcs = nil
			}
			// @cpt-end:cpt-katapult-flow-agent-system-heartbeat:p1:inst-heartbeat-discover

			if err := c.SendHeartbeat(ctx, pvcs); err != nil {
				c.logger.Error("heartbeat failed", "error", err)
				if isReregistrationNeeded(err) && regFunc != nil {
					c.logger.Info("disconnect detected, attempting re-registration")
					if rerr := c.retryReregister(ctx, regFunc, 10, 2*time.Second); rerr != nil {
						c.logger.Error("re-registration failed after retries", "error", rerr)
					} else {
						c.logger.Info("re-registration successful", "agent_id", c.agentID)
					}
				}
			}
		}
	}
}

// retryReregister attempts to re-register using regFunc with exponential backoff.
func (c *Client) retryReregister(ctx context.Context, regFunc func() error, maxRetries int, baseDelay time.Duration) error {
	var lastErr error
	for attempt := range maxRetries {
		if err := regFunc(); err != nil {
			lastErr = err
			c.logger.Warn("re-registration attempt failed", "attempt", attempt+1, "error", err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(baseDelay * time.Duration(1<<attempt)):
			}
			continue
		}
		return nil
	}
	return fmt.Errorf("re-registration exhausted %d retries: %w", maxRetries, lastErr)
}

// isReregistrationNeeded checks if the error indicates the agent must re-register.
func isReregistrationNeeded(err error) bool {
	if s, ok := status.FromError(err); ok {
		if s.Code() == codes.FailedPrecondition {
			return true
		}
	}
	return strings.Contains(err.Error(), "must re-register")
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
