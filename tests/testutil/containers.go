//go:build integration || e2e

package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func containerContext(t *testing.T) context.Context {
	t.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func StartRedisContainer(t *testing.T) string {
	t.Helper()
	ctx := containerContext(t)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := container.Terminate(shutdownCtx); err != nil {
			t.Logf("terminate redis container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get redis host: %v", err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatalf("get redis mapped port: %v", err)
	}
	return host + ":" + port.Port()
}

func StartPostgresContainer(t *testing.T) string {
	t.Helper()
	ctx := containerContext(t)
	const (
		user     = "llmgw"
		password = "llmgw-pass"
		dbName   = "llmgw_test"
	)

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     user,
				"POSTGRES_PASSWORD": password,
				"POSTGRES_DB":       dbName,
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(time.Minute),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := container.Terminate(shutdownCtx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("get postgres host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("get postgres mapped port: %v", err)
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", host, port.Port(), user, password, dbName)
}
