//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/maxitosh/katapult/internal/testutil"
)

// @cpt-dod:cpt-katapult-dod-integration-tests-component-tests:p2
// @cpt-flow:cpt-katapult-flow-integration-tests-run-component-tests:p2
// @cpt-algo:cpt-katapult-algo-integration-tests-testcontainers-setup:p2

// @cpt-begin:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-postgres
// @cpt-begin:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-minio

// Package-level shared state for all Tier 2 tests.
var (
	sharedPool     *pgxpool.Pool
	sharedMinIOCfg testutil.MinIOConfig

	pgOnce    sync.Once
	minioOnce sync.Once

	pgCleanup    func()
	minioCleanup func()
)

func TestMain(m *testing.M) {
	code := m.Run()

	if pgCleanup != nil {
		pgCleanup()
	}
	if minioCleanup != nil {
		minioCleanup()
	}

	os.Exit(code)
}

// getTestPool returns the shared PostgreSQL pool, starting the container on first call.
func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgOnce.Do(func() {
		migrationsDir := filepath.Join("..", "..", "internal", "store", "postgres", "migrations")
		sharedPool, pgCleanup = setupPostgresForSuite(t, migrationsDir)
	})
	if sharedPool == nil {
		t.Fatal("shared postgres pool not initialized")
	}
	return sharedPool
}

// getTestMinIO returns the shared MinIO config, starting the container on first call.
func getTestMinIO(t *testing.T) testutil.MinIOConfig {
	t.Helper()
	minioOnce.Do(func() {
		sharedMinIOCfg, minioCleanup = testutil.SetupMinIO(t)
	})
	return sharedMinIOCfg
}

// setupPostgresForSuite starts PostgreSQL and runs all migrations.
func setupPostgresForSuite(t *testing.T, migrationsDir string) (*pgxpool.Pool, func()) {
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

	// Run all up migrations via dynamic discovery.
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("reading migrations directory: %v", err)
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

	cleanup := func() {
		pool.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			fmt.Printf("terminating postgres container: %v\n", err)
		}
	}

	return pool, cleanup
}

// @cpt-end:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-minio
// @cpt-end:cpt-katapult-algo-integration-tests-testcontainers-setup:p2:inst-start-postgres
