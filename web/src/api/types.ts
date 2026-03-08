// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2

// --- Transfer domain types ---

export type TransferStrategy = "stream" | "s3" | "direct";

export type TransferState =
  | "pending"
  | "validating"
  | "transferring"
  | "completed"
  | "failed"
  | "cancelled";

export interface Transfer {
  id: string;
  source_cluster: string;
  source_pvc: string;
  destination_cluster: string;
  destination_pvc: string;
  strategy: TransferStrategy;
  state: TransferState;
  allow_overwrite: boolean;
  bytes_transferred: number;
  bytes_total: number;
  chunks_completed: number;
  chunks_total: number;
  error_message: string;
  retry_count: number;
  retry_max: number;
  created_by: string;
  created_at: string;
  started_at: string | null;
  completed_at: string | null;
}

export interface TransferEvent {
  id: string;
  transfer_id: string;
  event_type: string;
  message: string;
  metadata: Record<string, string>;
  created_at: string;
}

// --- Agent domain types ---

export type AgentState =
  | "registering"
  | "healthy"
  | "unhealthy"
  | "disconnected";

export interface PVCInfo {
  pvc_name: string;
  size_bytes: number;
  storage_class: string;
  node_affinity: string;
}

export interface Agent {
  id: string;
  cluster_id: string;
  node_name: string;
  state: AgentState;
  healthy: boolean;
  last_heartbeat: string;
  tools: {
    tar: boolean;
    zstd: boolean;
    stunnel: boolean;
  };
  registered_at: string;
  pvcs: PVCInfo[];
  jwt_namespace: string;
}

// --- Progress types ---

export interface EnrichedProgress {
  transfer_id: string;
  bytes_transferred: number;
  bytes_total: number;
  percent_complete: number;
  speed_bytes_sec: number;
  formatted_speed: string;
  estimated_time_remaining: string;
  chunks_completed: number;
  chunks_total: number;
  correlation_id: string;
  status: string;
  error_message?: string;
}

// --- Generic response types ---

export interface ListResponse<T> {
  items: T[];
  total: number;
}
