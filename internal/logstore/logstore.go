// Package logstore provides an asynchronous, buffered writer for UsageRecord
// entries. It replaces the previous fire-and-forget `go db.Create(...)` per
// request, which spawned an unbounded number of goroutines under load and
// silently dropped records when the database was slow or unavailable.
//
// Design:
//   - A single buffered channel decouples request handling from DB writes.
//   - A small pool of workers drains the channel and writes in batches.
//   - When the buffer is full, writes are dropped (and counted) rather than
//     blocking the request path — observability must never add latency to the
//     proxied LLM call. The drop counter is exported so operators can alert on
//     it via /metrics.
package logstore

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/warm3snow/llm-gateway/internal/database"
	"github.com/warm3snow/llm-gateway/internal/metrics"
	"github.com/warm3snow/llm-gateway/internal/models"
)

// Writer is an asynchronous batching writer for usage records.
type Writer struct {
	ch        chan *models.UsageRecord
	wg        sync.WaitGroup
	batchSize int
	flushIval time.Duration

	dropped   atomic.Int64
	written   atomic.Int64
	closeOnce sync.Once
}

// Options configures a Writer. Zero values fall back to sensible defaults.
type Options struct {
	BufferSize    int           // channel capacity; default 4096
	Workers       int           // number of drain workers; default 2
	BatchSize     int           // max rows per DB write; default 100
	FlushInterval time.Duration // max time a partial batch waits; default 1s
}

func (o Options) withDefaults() Options {
	if o.BufferSize <= 0 {
		o.BufferSize = 4096
	}
	if o.Workers <= 0 {
		o.Workers = 2
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 100
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = time.Second
	}
	return o
}

// defaultWriter is the process-wide writer used by the Enqueue helper.
var (
	defaultWriter *Writer
	defaultMu     sync.RWMutex
)

// Init creates the process-wide writer and starts its workers. It is safe to
// call once at startup. Calling it again replaces (and does not stop) the
// previous writer, so callers should Shutdown the old one first.
func Init(opts Options) *Writer {
	w := New(opts)
	defaultMu.Lock()
	defaultWriter = w
	defaultMu.Unlock()
	return w
}

// New creates and starts a Writer.
func New(opts Options) *Writer {
	opts = opts.withDefaults()
	w := &Writer{
		ch:        make(chan *models.UsageRecord, opts.BufferSize),
		batchSize: opts.BatchSize,
		flushIval: opts.FlushInterval,
	}
	for i := 0; i < opts.Workers; i++ {
		w.wg.Add(1)
		go w.worker()
	}
	return w
}

// Enqueue submits a usage record to the process-wide writer. If no writer has
// been initialized, it falls back to a synchronous best-effort write so records
// are never silently lost during early startup or in tests.
func Enqueue(entry *models.UsageRecord) {
	defaultMu.RLock()
	w := defaultWriter
	defaultMu.RUnlock()
	if w == nil {
		if db := database.GetDB(); db != nil {
			db.Create(entry)
		}
		return
	}
	w.Enqueue(entry)
}

// Enqueue submits a usage record without blocking. If the buffer is full the
// entry is dropped and the drop counter is incremented.
func (w *Writer) Enqueue(entry *models.UsageRecord) {
	select {
	case w.ch <- entry:
	default:
		metrics.RecordsDropped.Set(float64(w.dropped.Add(1)))
	}
}

// Dropped returns the number of usage records dropped due to a full buffer.
func (w *Writer) Dropped() int64 { return w.dropped.Load() }

// Written returns the number of log entries successfully persisted.
func (w *Writer) Written() int64 { return w.written.Load() }

func (w *Writer) worker() {
	defer w.wg.Done()

	batch := make([]*models.UsageRecord, 0, w.batchSize)
	ticker := time.NewTicker(w.flushIval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if db := database.GetDB(); db != nil {
			if err := db.CreateInBatches(batch, len(batch)).Error; err != nil {
				log.Printf("[LOGSTORE] failed to persist %d usage records: %v", len(batch), err)
			} else {
				w.written.Add(int64(len(batch)))
			}
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-w.ch:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= w.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// Shutdown stops accepting new entries, drains the buffer, and waits for
// workers to finish (or the context to expire).
func (w *Writer) Shutdown(ctx context.Context) {
	w.closeOnce.Do(func() {
		close(w.ch)
	})

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		log.Printf("[LOGSTORE] shutdown timed out; %d entries may be unflushed", len(w.ch))
	}
}

// ShutdownDefault drains the process-wide writer, if any.
func ShutdownDefault(ctx context.Context) {
	defaultMu.RLock()
	w := defaultWriter
	defaultMu.RUnlock()
	if w != nil {
		w.Shutdown(ctx)
	}
}
