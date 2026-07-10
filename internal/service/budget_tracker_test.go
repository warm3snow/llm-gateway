package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/models"
)

func setupBudgetTrackerTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "budget_tracker.db")
	if err := database.Connect(&database.Config{Driver: "sqlite", DSN: dsn, LogLevel: "silent"}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
		database.DB = nil
	})
	if err := database.Migrate(&models.Tenant{}, &models.VirtualKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestBudgetTrackerEnqueueFlushesBatchedUsage(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	db := database.GetDB()
	key := models.VirtualKey{Name: "tracked", KeyHash: "hash", KeySalt: "salt", HashedKey: "vsk-test", Status: "active"}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}

	tracker := NewBudgetTracker(db, BudgetTrackerOptions{QueueSize: 10, BatchSize: 10, FlushInterval: time.Hour})
	if err := tracker.Enqueue(key.ID, 0.25); err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	if err := tracker.Enqueue(key.ID, 0.75); err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	if got := tracker.Pending(key.ID); got != 1.0 {
		t.Fatalf("pending = %v, want 1.0", got)
	}

	if err := tracker.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	var loaded models.VirtualKey
	if err := db.First(&loaded, key.ID).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if loaded.BudgetUsed != 1.0 {
		t.Fatalf("budget_used = %v, want 1.0", loaded.BudgetUsed)
	}
	if got := tracker.Pending(key.ID); got != 0 {
		t.Fatalf("pending after flush = %v, want 0", got)
	}
}

func TestBudgetTrackerKeepsPendingWhenFlushFails(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	db := database.GetDB()
	tracker := NewBudgetTracker(db, BudgetTrackerOptions{QueueSize: 10, BatchSize: 10, FlushInterval: time.Hour})
	if err := tracker.Enqueue(999, 1.5); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := tracker.Flush(context.Background()); err == nil {
		t.Fatalf("flush succeeded unexpectedly")
	}
	if got := tracker.Pending(999); got != 1.5 {
		t.Fatalf("pending after failed flush = %v, want 1.5", got)
	}
}

func TestBudgetTrackerReturnsQueueFullWithoutLosingPendingAccounting(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	tracker := NewBudgetTracker(database.GetDB(), BudgetTrackerOptions{QueueSize: 1, BatchSize: 10, FlushInterval: time.Hour})
	if err := tracker.Enqueue(1, 1); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	if err := tracker.Enqueue(1, 2); !errors.Is(err, ErrBudgetTrackerQueueFull) {
		t.Fatalf("second enqueue error = %v, want ErrBudgetTrackerQueueFull", err)
	}
	if got := tracker.Pending(1); got != 1 {
		t.Fatalf("pending = %v, want only first enqueue", got)
	}
}

func TestBudgetTrackerIgnoresNoOpEnqueues(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	tracker := NewBudgetTracker(database.GetDB(), BudgetTrackerOptions{QueueSize: 10, BatchSize: 10, FlushInterval: time.Hour})

	if err := tracker.Enqueue(0, 1); err != nil {
		t.Fatalf("zero id enqueue: %v", err)
	}
	if err := tracker.Enqueue(1, 0); err != nil {
		t.Fatalf("zero cost enqueue: %v", err)
	}
	if err := tracker.Enqueue(1, -1); err != nil {
		t.Fatalf("negative cost enqueue: %v", err)
	}
	if got := tracker.Pending(1); got != 0 {
		t.Fatalf("pending = %v, want 0", got)
	}
	if err := tracker.Flush(context.Background()); err != nil {
		t.Fatalf("empty flush: %v", err)
	}
}

func TestBudgetTrackerAppliesDefaultOptions(t *testing.T) {
	tracker := NewBudgetTracker(nil, BudgetTrackerOptions{})

	if cap(tracker.updates) != 10000 {
		t.Fatalf("queue capacity = %d, want 10000", cap(tracker.updates))
	}
	if tracker.batchSize != 500 {
		t.Fatalf("batchSize = %d, want 500", tracker.batchSize)
	}
	if tracker.flushInterval != 250*time.Millisecond {
		t.Fatalf("flushInterval = %s, want 250ms", tracker.flushInterval)
	}
	if tracker.flushTimeout != 5*time.Second {
		t.Fatalf("flushTimeout = %s, want 5s", tracker.flushTimeout)
	}
}

func TestBudgetTrackerShutdownWithoutStartFlushesQueuedUsage(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	db := database.GetDB()
	key := models.VirtualKey{Name: "shutdown", KeyHash: "hash-shutdown", KeySalt: "salt", HashedKey: "vsk-shutdown", Status: "active"}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create key: %v", err)
	}

	tracker := NewBudgetTracker(db, BudgetTrackerOptions{QueueSize: 10, BatchSize: 10, FlushInterval: time.Hour})
	if err := tracker.Enqueue(key.ID, 2.5); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := tracker.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	var loaded models.VirtualKey
	if err := db.First(&loaded, key.ID).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if loaded.BudgetUsed != 2.5 {
		t.Fatalf("budget_used = %v, want 2.5", loaded.BudgetUsed)
	}
	if err := tracker.Enqueue(key.ID, 1); !errors.Is(err, ErrBudgetTrackerQueueFull) {
		t.Fatalf("enqueue after shutdown = %v, want ErrBudgetTrackerQueueFull", err)
	}
}

func TestBudgetTrackerFlushRetryAfterMissingKeyIsCreated(t *testing.T) {
	setupBudgetTrackerTestDB(t)
	db := database.GetDB()
	tracker := NewBudgetTracker(db, BudgetTrackerOptions{QueueSize: 10, BatchSize: 10, FlushInterval: time.Hour})
	if err := tracker.Enqueue(42, 3.25); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	if err := tracker.Flush(context.Background()); err == nil {
		t.Fatalf("first flush succeeded unexpectedly")
	}
	key := models.VirtualKey{ID: 42, Name: "created-later", KeyHash: "hash-later", KeySalt: "salt", HashedKey: "vsk-later", Status: "active"}
	if err := db.Create(&key).Error; err != nil {
		t.Fatalf("create missing key: %v", err)
	}
	if err := tracker.Flush(context.Background()); err != nil {
		t.Fatalf("second flush: %v", err)
	}

	var loaded models.VirtualKey
	if err := db.First(&loaded, uint(42)).Error; err != nil {
		t.Fatalf("load key: %v", err)
	}
	if loaded.BudgetUsed != 3.25 {
		t.Fatalf("budget_used = %v, want 3.25", loaded.BudgetUsed)
	}
	if got := tracker.Pending(42); got != 0 {
		t.Fatalf("pending after retry = %v, want 0", got)
	}
}
