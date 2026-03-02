ALTER TABLE agents ADD COLUMN state TEXT NOT NULL DEFAULT 'healthy';
UPDATE agents SET state = 'unhealthy' WHERE healthy = FALSE;
CREATE INDEX idx_agents_state ON agents (state);
