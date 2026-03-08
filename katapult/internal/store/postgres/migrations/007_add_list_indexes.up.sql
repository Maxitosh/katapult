CREATE INDEX IF NOT EXISTS idx_transfers_state ON transfers (state);
CREATE INDEX IF NOT EXISTS idx_transfers_source_cluster ON transfers (source_cluster);
CREATE INDEX IF NOT EXISTS idx_transfers_dest_cluster ON transfers (destination_cluster);
CREATE INDEX IF NOT EXISTS idx_agents_state ON agents (state);
CREATE INDEX IF NOT EXISTS idx_agents_cluster_id ON agents (cluster_id);
