//go:build integration

package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-shared-helpers:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-testcontainers-setup:p2

// @cpt-begin:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-postgres

// SetupPostgres starts a PostgreSQL testcontainer, runs all migrations via dynamic
// file discovery, and returns a connection pool with a cleanup function.
func SetupPostgres(t *testing.T, migrationsDir string) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("katapult_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("starting postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("getting connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("creating pool: %v", err)
	}

	// @cpt-begin:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-run-migrations
	runMigrations(t, pool, migrationsDir)
	// @cpt-end:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-run-migrations

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminating postgres container: %v", err)
		}
	}

	return pool, cleanup
}

// @cpt-end:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-postgres

// runMigrations dynamically discovers and runs all *.up.sql migrations in order.
func runMigrations(t *testing.T, pool *pgxpool.Pool, migrationsDir string) {
	t.Helper()
	ctx := context.Background()

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading migrations directory %s: %v", migrationsDir, err)
	}

	var upMigrations []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upMigrations = append(upMigrations, entry.Name())
		}
	}
	sort.Strings(upMigrations)

	for _, m := range upMigrations {
		data, err := os.ReadFile(filepath.Join(migrationsDir, m))
		if err != nil {
			t.Fatalf("reading migration %s: %v", m, err)
		}
		if _, err := pool.Exec(ctx, string(data)); err != nil {
			t.Fatalf("running migration %s: %v", m, err)
		}
	}
}

// @cpt-begin:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-minio

// MinIOConfig holds MinIO connection details returned by SetupMinIO.
type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

// SetupMinIO starts a MinIO testcontainer with a preconfigured bucket
// and returns connection details with a cleanup function.
func SetupMinIO(t *testing.T) (MinIOConfig, func()) {
	t.Helper()
	ctx := context.Background()

	const (
		accessKey = "minioadmin"
		secretKey = "minioadmin"
		bucket    = "katapult-test"
	)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "minio/minio:latest",
			ExposedPorts: []string{"9000/tcp"},
			Cmd:          []string{"server", "/data"},
			Env: map[string]string{
				"MINIO_ROOT_USER":     accessKey,
				"MINIO_ROOT_PASSWORD": secretKey,
			},
			WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("starting minio container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("getting minio host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("getting minio port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	cfg := MinIOConfig{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    bucket,
	}

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminating minio container: %v", err)
		}
	}

	return cfg, cleanup
}

// @cpt-end:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-minio
