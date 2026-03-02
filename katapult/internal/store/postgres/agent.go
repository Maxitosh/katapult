package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/chainstack/katapult/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AgentRepository implements registry.AgentRepository using PostgreSQL.
type AgentRepository struct {
	pool *pgxpool.Pool
}

// NewAgentRepository creates a new PostgreSQL-backed agent repository.
func NewAgentRepository(pool *pgxpool.Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

func (r *AgentRepository) UpsertAgent(ctx context.Context, agent *domain.Agent) error {
	toolsJSON, err := json.Marshal(agent.Tools)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO agents (id, cluster_id, node_name, healthy, last_heartbeat, tools, registered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (cluster_id, node_name) DO UPDATE SET
			id = EXCLUDED.id,
			healthy = EXCLUDED.healthy,
			last_heartbeat = EXCLUDED.last_heartbeat,
			tools = EXCLUDED.tools
	`, agent.ID, agent.ClusterID, agent.NodeName, agent.Healthy, agent.LastHeartbeat, toolsJSON, agent.RegisteredAt)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM agent_pvcs WHERE agent_id = $1`, agent.ID)
	if err != nil {
		return err
	}

	for _, pvc := range agent.PVCs {
		_, err = tx.Exec(ctx, `
			INSERT INTO agent_pvcs (agent_id, pvc_name, size_bytes, storage_class, node_affinity, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`, agent.ID, pvc.PVCName, pvc.SizeBytes, pvc.StorageClass, pvc.NodeAffinity)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *AgentRepository) GetAgentByID(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
	agent, err := r.scanAgent(ctx, `SELECT id, cluster_id, node_name, healthy, last_heartbeat, tools, registered_at FROM agents WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}

	pvcs, err := r.getPVCs(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	agent.PVCs = pvcs

	return agent, nil
}

func (r *AgentRepository) GetAgentByClusterAndNode(ctx context.Context, clusterID, nodeName string) (*domain.Agent, error) {
	agent, err := r.scanAgent(ctx, `SELECT id, cluster_id, node_name, healthy, last_heartbeat, tools, registered_at FROM agents WHERE cluster_id = $1 AND node_name = $2`, clusterID, nodeName)
	if err != nil {
		return nil, err
	}

	pvcs, err := r.getPVCs(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	agent.PVCs = pvcs

	return agent, nil
}

func (r *AgentRepository) UpdateHeartbeat(ctx context.Context, agentID uuid.UUID, pvcs []domain.PVCInfo) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `UPDATE agents SET healthy = TRUE, last_heartbeat = NOW() WHERE id = $1`, agentID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM agent_pvcs WHERE agent_id = $1`, agentID)
	if err != nil {
		return err
	}

	for _, pvc := range pvcs {
		_, err = tx.Exec(ctx, `
			INSERT INTO agent_pvcs (agent_id, pvc_name, size_bytes, storage_class, node_affinity, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
		`, agentID, pvc.PVCName, pvc.SizeBytes, pvc.StorageClass, pvc.NodeAffinity)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *AgentRepository) MarkUnhealthy(ctx context.Context, cutoff time.Time) (int, error) {
	tag, err := r.pool.Exec(ctx, `UPDATE agents SET healthy = FALSE WHERE healthy = TRUE AND last_heartbeat < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *AgentRepository) MarkDisconnected(ctx context.Context, cutoff time.Time) (int, error) {
	// Disconnected agents are tracked by setting healthy=false with a sufficiently old heartbeat.
	// In a production system this would update a state column; for now the health evaluator
	// uses the cutoff times to distinguish unhealthy from disconnected.
	// This is a no-op placeholder since the agents table doesn't yet have a state column.
	// The health evaluator in the registry service handles the distinction via time windows.
	_ = cutoff
	return 0, nil
}

func (r *AgentRepository) scanAgent(ctx context.Context, query string, args ...any) (*domain.Agent, error) {
	row := r.pool.QueryRow(ctx, query, args...)

	var a domain.Agent
	var toolsJSON []byte
	err := row.Scan(&a.ID, &a.ClusterID, &a.NodeName, &a.Healthy, &a.LastHeartbeat, &toolsJSON, &a.RegisteredAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(toolsJSON, &a.Tools); err != nil {
		return nil, err
	}

	if a.Healthy {
		a.State = domain.AgentStateHealthy
	} else {
		a.State = domain.AgentStateUnhealthy
	}

	return &a, nil
}

func (r *AgentRepository) getPVCs(ctx context.Context, agentID uuid.UUID) ([]domain.PVCInfo, error) {
	rows, err := r.pool.Query(ctx, `SELECT pvc_name, size_bytes, storage_class, node_affinity FROM agent_pvcs WHERE agent_id = $1`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pvcs []domain.PVCInfo
	for rows.Next() {
		var p domain.PVCInfo
		if err := rows.Scan(&p.PVCName, &p.SizeBytes, &p.StorageClass, &p.NodeAffinity); err != nil {
			return nil, err
		}
		pvcs = append(pvcs, p)
	}

	return pvcs, rows.Err()
}
