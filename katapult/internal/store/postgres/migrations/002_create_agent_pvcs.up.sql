CREATE TABLE agent_pvcs (
    agent_id        UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    pvc_name        TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    storage_class   TEXT NOT NULL,
    node_affinity   TEXT NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (agent_id, pvc_name)
);
