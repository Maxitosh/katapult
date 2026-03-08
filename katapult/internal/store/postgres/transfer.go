package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maxitosh/katapult/internal/domain"
)

// TransferRepository implements transfer.TransferRepository using PostgreSQL.
type TransferRepository struct {
	pool *pgxpool.Pool
}

// NewTransferRepository creates a new PostgreSQL-backed transfer repository.
func NewTransferRepository(pool *pgxpool.Pool) *TransferRepository {
	return &TransferRepository{pool: pool}
}

// @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
func (r *TransferRepository) CreateTransfer(ctx context.Context, transfer *domain.Transfer) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-create-transfer
	_, err := r.pool.Exec(ctx, `
		INSERT INTO transfers (id, source_cluster, source_pvc, destination_cluster, destination_pvc,
			strategy, state, allow_overwrite, bytes_transferred, bytes_total,
			chunks_completed, chunks_total, error_message, retry_count, retry_max,
			created_by, created_at, started_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`, transfer.ID, transfer.SourceCluster, transfer.SourcePVC,
		transfer.DestinationCluster, transfer.DestinationPVC,
		nullableString(string(transfer.Strategy)), string(transfer.State),
		transfer.AllowOverwrite, transfer.BytesTransferred, transfer.BytesTotal,
		transfer.ChunksCompleted, transfer.ChunksTotal, nullableString(transfer.ErrorMessage),
		transfer.RetryCount, transfer.RetryMax, nullableString(transfer.CreatedBy),
		transfer.CreatedAt, transfer.StartedAt, transfer.CompletedAt)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-create-transfer
}

func (r *TransferRepository) GetTransferByID(ctx context.Context, id uuid.UUID) (*domain.Transfer, error) {
	return r.scanTransfer(ctx, `
		SELECT id, source_cluster, source_pvc, destination_cluster, destination_pvc,
			strategy, state, allow_overwrite, bytes_transferred, bytes_total,
			chunks_completed, chunks_total, error_message, retry_count, retry_max,
			created_by, created_at, started_at, completed_at
		FROM transfers WHERE id = $1
	`, id)
}

// @cpt-dod:cpt-katapult-dod-transfer-engine-persistence:p1
func (r *TransferRepository) UpdateTransferState(ctx context.Context, id uuid.UUID, state domain.TransferState) error {
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET state = $2 WHERE id = $1`, id, string(state))
	return err
}

func (r *TransferRepository) UpdateTransferStrategy(ctx context.Context, id uuid.UUID, strategy domain.TransferStrategy) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-strategy
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET strategy = $2 WHERE id = $1`, id, string(strategy))
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-strategy
}

func (r *TransferRepository) UpdateTransferProgress(ctx context.Context, id uuid.UUID, bytesTransferred, bytesTotal int64, chunksCompleted, chunksTotal int) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-update-progress
	_, err := r.pool.Exec(ctx, `
		UPDATE transfers SET bytes_transferred = $2, bytes_total = $3, chunks_completed = $4, chunks_total = $5 WHERE id = $1
	`, id, bytesTransferred, bytesTotal, chunksCompleted, chunksTotal)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-update-progress
}

func (r *TransferRepository) UpdateTransferStarted(ctx context.Context, id uuid.UUID) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-started
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET started_at = $2 WHERE id = $1`, id, now)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-initiate:p1:inst-db-set-started
}

func (r *TransferRepository) UpdateTransferCompleted(ctx context.Context, id uuid.UUID) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-completed
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET completed_at = $2 WHERE id = $1`, id, now)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-completed
}

func (r *TransferRepository) UpdateTransferFailed(ctx context.Context, id uuid.UUID, errorMessage string) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-failed
	now := time.Now()
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET state = 'failed', error_message = $2, completed_at = $3 WHERE id = $1`, id, errorMessage, now)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-set-failed
}

func (r *TransferRepository) IncrementRetryCount(ctx context.Context, id uuid.UUID) error {
	// @cpt-begin:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-increment-retry
	_, err := r.pool.Exec(ctx, `UPDATE transfers SET retry_count = retry_count + 1 WHERE id = $1`, id)
	return err
	// @cpt-end:cpt-katapult-flow-transfer-engine-report-progress:p1:inst-db-increment-retry
}

func (r *TransferRepository) CreateTransferEvent(ctx context.Context, event *domain.TransferEvent) error {
	metadataJSON, err := json.Marshal(event.Metadata)
	if err != nil {
		return err
	}
	if event.Metadata == nil {
		metadataJSON = []byte("{}")
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO transfer_events (id, transfer_id, event_type, message, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, event.ID, event.TransferID, event.EventType, event.Message, metadataJSON, event.CreatedAt)
	return err
}

func (r *TransferRepository) GetTransferEvents(ctx context.Context, transferID uuid.UUID) ([]domain.TransferEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, transfer_id, event_type, message, metadata, created_at
		FROM transfer_events WHERE transfer_id = $1 ORDER BY created_at ASC
	`, transferID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.TransferEvent
	for rows.Next() {
		var e domain.TransferEvent
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.TransferID, &e.EventType, &e.Message, &metadataJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		if len(metadataJSON) > 0 {
			_ = json.Unmarshal(metadataJSON, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// ListTransfers returns a filtered, paginated list of transfers with total count.
// @cpt-flow:cpt-katapult-flow-api-cli-list-transfers:p1
// @cpt-dod:cpt-katapult-dod-api-cli-rest-transfer-endpoints:p1
func (r *TransferRepository) ListTransfers(ctx context.Context, filter domain.TransferFilter) ([]domain.Transfer, int, error) {
	// @cpt-begin:cpt-katapult-flow-api-cli-list-transfers:p1:inst-delegate-list
	query := `
		SELECT id, source_cluster, source_pvc, destination_cluster, destination_pvc,
			strategy, state, allow_overwrite, bytes_transferred, bytes_total,
			chunks_completed, chunks_total, error_message, retry_count, retry_max,
			created_by, created_at, started_at, completed_at,
			COUNT(*) OVER() AS total_count
		FROM transfers WHERE 1=1`
	args := []any{}
	argIdx := 1

	if filter.State != nil {
		query += fmt.Sprintf(" AND state = $%d", argIdx)
		args = append(args, string(*filter.State))
		argIdx++
	}
	if filter.Cluster != nil {
		query += fmt.Sprintf(" AND (source_cluster = $%d OR destination_cluster = $%d)", argIdx, argIdx)
		args = append(args, *filter.Cluster)
		argIdx++
	}

	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIdx)
	args = append(args, limit)
	argIdx++

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
		argIdx++
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var transfers []domain.Transfer
	var totalCount int
	for rows.Next() {
		var t domain.Transfer
		var strategy, state, errorMessage, createdBy *string
		if err := rows.Scan(&t.ID, &t.SourceCluster, &t.SourcePVC,
			&t.DestinationCluster, &t.DestinationPVC,
			&strategy, &state, &t.AllowOverwrite,
			&t.BytesTransferred, &t.BytesTotal,
			&t.ChunksCompleted, &t.ChunksTotal,
			&errorMessage, &t.RetryCount, &t.RetryMax,
			&createdBy, &t.CreatedAt, &t.StartedAt, &t.CompletedAt,
			&totalCount); err != nil {
			return nil, 0, err
		}
		if strategy != nil {
			t.Strategy = domain.TransferStrategy(*strategy)
		}
		if state != nil {
			t.State = domain.TransferState(*state)
		}
		if errorMessage != nil {
			t.ErrorMessage = *errorMessage
		}
		if createdBy != nil {
			t.CreatedBy = *createdBy
		}
		transfers = append(transfers, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return transfers, totalCount, nil
	// @cpt-end:cpt-katapult-flow-api-cli-list-transfers:p1:inst-delegate-list
}

func (r *TransferRepository) scanTransfer(ctx context.Context, query string, args ...any) (*domain.Transfer, error) {
	row := r.pool.QueryRow(ctx, query, args...)

	var t domain.Transfer
	var strategy, state, errorMessage, createdBy *string
	err := row.Scan(&t.ID, &t.SourceCluster, &t.SourcePVC,
		&t.DestinationCluster, &t.DestinationPVC,
		&strategy, &state, &t.AllowOverwrite,
		&t.BytesTransferred, &t.BytesTotal,
		&t.ChunksCompleted, &t.ChunksTotal,
		&errorMessage, &t.RetryCount, &t.RetryMax,
		&createdBy, &t.CreatedAt, &t.StartedAt, &t.CompletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if strategy != nil {
		t.Strategy = domain.TransferStrategy(*strategy)
	}
	if state != nil {
		t.State = domain.TransferState(*state)
	}
	if errorMessage != nil {
		t.ErrorMessage = *errorMessage
	}
	if createdBy != nil {
		t.CreatedBy = *createdBy
	}

	return &t, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
