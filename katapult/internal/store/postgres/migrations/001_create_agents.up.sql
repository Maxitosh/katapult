CREATE TABLE agents (
    id              UUID PRIMARY KEY,
    cluster_id      TEXT NOT NULL,
    node_name       TEXT NOT NULL,
    healthy         BOOLEAN NOT NULL DEFAULT TRUE,
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    tools           JSONB NOT NULL DEFAULT '{}',
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_agents_cluster_node UNIQUE (cluster_id, node_name)
);

CREATE INDEX idx_agents_cluster ON agents (cluster_id);
CREATE INDEX idx_agents_healthy ON agents (healthy);
