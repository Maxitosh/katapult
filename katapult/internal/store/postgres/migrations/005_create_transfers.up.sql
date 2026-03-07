-- @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
CREATE TABLE transfers (
    id                 UUID PRIMARY KEY,
    source_cluster     TEXT NOT NULL,
    source_pvc         TEXT NOT NULL,
    destination_cluster TEXT NOT NULL,
    destination_pvc    TEXT NOT NULL,
    strategy           TEXT,
    state              TEXT NOT NULL DEFAULT 'pending'
        CHECK (state IN ('pending', 'validating', 'transferring', 'completed', 'failed', 'cancelled')),
    allow_overwrite    BOOLEAN NOT NULL DEFAULT FALSE,
    bytes_transferred  BIGINT NOT NULL DEFAULT 0,
    bytes_total        BIGINT NOT NULL DEFAULT 0,
    chunks_completed   INT NOT NULL DEFAULT 0,
    chunks_total       INT NOT NULL DEFAULT 0,
    error_message      TEXT,
    retry_count        INT NOT NULL DEFAULT 0,
    retry_max          INT NOT NULL DEFAULT 3,
    created_by         TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at         TIMESTAMPTZ,
    completed_at       TIMESTAMPTZ
);

CREATE INDEX idx_transfers_state ON transfers (state);
CREATE INDEX idx_transfers_created_at ON transfers (created_at);
