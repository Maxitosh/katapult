-- @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
CREATE TABLE transfer_events (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transfer_id UUID NOT NULL REFERENCES transfers(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    message     TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transfer_events_transfer_id ON transfer_events (transfer_id);
CREATE INDEX idx_transfer_events_created_at ON transfer_events (created_at);
