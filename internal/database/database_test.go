package database

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConnectAppliesConnectionPoolLimits(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
		DB = nil
	})

	dsn := filepath.Join(t.TempDir(), "pool.db")
	err := Connect(&Config{
		Driver:          "sqlite",
		DSN:             dsn,
		LogLevel:        "silent",
		MaxOpenConns:    7,
		MaxIdleConns:    3,
		ConnMaxLifetime: time.Minute,
	})
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("DB.DB returned error: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 7 {
		t.Fatalf("MaxOpenConnections = %d, want 7", got)
	}
}
