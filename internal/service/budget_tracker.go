package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/warm3snow/llm-gateway/internal/metrics"
	"github.com/warm3snow/llm-gateway/internal/models"
	"gorm.io/gorm"
)

var ErrBudgetTrackerQueueFull = errors.New("budget tracker queue full")

type BudgetTrackerOptions struct {
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
	FlushTimeout  time.Duration
}

type BudgetTracker struct {
	db *gorm.DB

	updates chan budgetUpdate
	stop    chan struct{}
	done    chan struct{}

	batchMu sync.Mutex
	batch   map[uint]float64

	pendingMu    sync.RWMutex
	pending      map[uint]float64
	pendingTotal float64

	batchSize     int
	flushInterval time.Duration
	flushTimeout  time.Duration

	startOnce sync.Once
	closeOnce sync.Once
	closedMu  sync.RWMutex
	closed    bool
	started   bool
}

type budgetUpdate struct {
	virtualKeyID uint
	cost         float64
}

func budgetTrackerOptionsWithDefaults(opts BudgetTrackerOptions) BudgetTrackerOptions {
	if opts.QueueSize <= 0 {
		opts.QueueSize = 10000
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 500
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = 250 * time.Millisecond
	}
	if opts.FlushTimeout <= 0 {
		opts.FlushTimeout = 5 * time.Second
	}
	return opts
}

func NewBudgetTracker(db *gorm.DB, opts BudgetTrackerOptions) *BudgetTracker {
	opts = budgetTrackerOptionsWithDefaults(opts)
	return &BudgetTracker{
		db:            db,
		updates:       make(chan budgetUpdate, opts.QueueSize),
		stop:          make(chan struct{}),
		done:          make(chan struct{}),
		batch:         make(map[uint]float64),
		pending:       make(map[uint]float64),
		batchSize:     opts.BatchSize,
		flushInterval: opts.FlushInterval,
		flushTimeout:  opts.FlushTimeout,
	}
}

func (t *BudgetTracker) Start() {
	if t == nil {
		return
	}
	t.startOnce.Do(func() {
		t.closedMu.Lock()
		t.started = true
		t.closedMu.Unlock()
		go t.run()
	})
}

func (t *BudgetTracker) Enqueue(virtualKeyID uint, cost float64) error {
	if t == nil || cost <= 0 || virtualKeyID == 0 {
		return nil
	}
	if t.isClosed() {
		return ErrBudgetTrackerQueueFull
	}
	t.addPending(virtualKeyID, cost)
	select {
	case t.updates <- budgetUpdate{virtualKeyID: virtualKeyID, cost: cost}:
		metrics.RecordBudgetTrackerEnqueue("queued", len(t.updates), t.PendingTotal())
		return nil
	default:
		t.addPending(virtualKeyID, -cost)
		metrics.RecordBudgetTrackerEnqueue("queue_full", len(t.updates), t.PendingTotal())
		return ErrBudgetTrackerQueueFull
	}
}

func (t *BudgetTracker) Pending(virtualKeyID uint) float64 {
	if t == nil || virtualKeyID == 0 {
		return 0
	}
	t.pendingMu.RLock()
	defer t.pendingMu.RUnlock()
	return t.pending[virtualKeyID]
}

func (t *BudgetTracker) PendingTotal() float64 {
	if t == nil {
		return 0
	}
	t.pendingMu.RLock()
	defer t.pendingMu.RUnlock()
	return t.pendingTotal
}

func (t *BudgetTracker) Flush(ctx context.Context) error {
	if t == nil {
		return nil
	}
	t.drainQueue()
	updates := t.takeBatch()
	if len(updates) == 0 {
		metrics.RecordBudgetTrackerState(len(t.updates), t.PendingTotal())
		return nil
	}
	start := time.Now()
	err := t.flushBatch(ctx, updates)
	result := "success"
	if err != nil {
		result = "error"
		t.restoreBatch(updates)
	} else {
		for id, cost := range updates {
			t.addPending(id, -cost)
		}
	}
	metrics.RecordBudgetTrackerFlush(result, len(updates), time.Since(start), len(t.updates), t.PendingTotal())
	return err
}

func (t *BudgetTracker) Shutdown(ctx context.Context) error {
	if t == nil {
		return nil
	}
	if !t.markClosed() {
		return t.Flush(ctx)
	}
	t.closeOnce.Do(func() {
		close(t.stop)
	})
	select {
	case <-t.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return t.Flush(ctx)
}

func (t *BudgetTracker) run() {
	defer close(t.done)
	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case update := <-t.updates:
			t.addToBatch(update)
			if t.batchLen() >= t.batchSize {
				t.flushWithTimeout()
			}
		case <-ticker.C:
			t.flushWithTimeout()
		case <-t.stop:
			t.drainQueue()
			t.flushWithTimeout()
			return
		}
	}
}

func (t *BudgetTracker) flushWithTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), t.flushTimeout)
	defer cancel()
	_ = t.Flush(ctx)
}

func (t *BudgetTracker) flushBatch(ctx context.Context, updates map[uint]float64) error {
	if t.db == nil {
		return errors.New("budget tracker database is nil")
	}
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for id, cost := range updates {
			result := tx.Model(&models.VirtualKey{}).Where("id = ?", id).
				UpdateColumn("budget_used", gorm.Expr("budget_used + ?", cost))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

func (t *BudgetTracker) drainQueue() {
	for {
		select {
		case update := <-t.updates:
			t.addToBatch(update)
		default:
			return
		}
	}
}

func (t *BudgetTracker) addToBatch(update budgetUpdate) {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()
	t.batch[update.virtualKeyID] += update.cost
}

func (t *BudgetTracker) batchLen() int {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()
	return len(t.batch)
}

func (t *BudgetTracker) takeBatch() map[uint]float64 {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()
	if len(t.batch) == 0 {
		return nil
	}
	updates := t.batch
	t.batch = make(map[uint]float64)
	return updates
}

func (t *BudgetTracker) restoreBatch(updates map[uint]float64) {
	t.batchMu.Lock()
	defer t.batchMu.Unlock()
	for id, cost := range updates {
		t.batch[id] += cost
	}
}

func (t *BudgetTracker) addPending(virtualKeyID uint, delta float64) {
	t.pendingMu.Lock()
	defer t.pendingMu.Unlock()
	next := t.pending[virtualKeyID] + delta
	if next <= 0 {
		delete(t.pending, virtualKeyID)
	} else {
		t.pending[virtualKeyID] = next
	}
	t.pendingTotal += delta
	if t.pendingTotal < 0 {
		t.pendingTotal = 0
	}
}

func (t *BudgetTracker) markClosed() bool {
	t.closedMu.Lock()
	defer t.closedMu.Unlock()
	if t.closed {
		return t.started
	}
	t.closed = true
	return t.started
}

func (t *BudgetTracker) isClosed() bool {
	t.closedMu.RLock()
	defer t.closedMu.RUnlock()
	return t.closed
}
