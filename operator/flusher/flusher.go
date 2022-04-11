package flusher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// These are vars so they can be overridden in tests
var maxRetryInterval = time.Minute
var maxElapsedTime = time.Hour

// Config holds the configuration to build a new flusher
type Config struct {
	// MaxConcurrent is the maximum number of goroutines flushing entries concurrently.
	// Defaults to 16.
	MaxConcurrent    int           `json:"max_concurrent" yaml:"max_concurrent"`
	MaxRetryInterval time.Duration `json:"max_retry_interval" yaml:"max_retry_interval"`
	MaxElapsedTime   time.Duration `json:"max_elapsed_time" yaml:"max_elapsed_time"`
}

type Flusher struct {
	chunkIDCounter uint64
	ctx            context.Context
	cancel         context.CancelFunc
	sem            *semaphore.Weighted
	wg             sync.WaitGroup
	config         Config
	*zap.SugaredLogger
}

// NewConfig creates a new default flusher config
func NewConfig() Config {
	return Config{
		MaxConcurrent: 16,
	}
}

// Build uses a Config to build a new Flusher
func (c Config) Build(logger *zap.SugaredLogger) *Flusher {
	maxConcurrent := c.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 16
	}

	ctx, cancel := context.WithCancel(context.Background())

	if c.MaxRetryInterval == 0 {
		c.MaxRetryInterval = maxRetryInterval
	}

	if c.MaxElapsedTime == 0 {
		c.MaxElapsedTime = maxElapsedTime
	}

	return &Flusher{
		ctx:           ctx,
		cancel:        cancel,
		sem:           semaphore.NewWeighted(int64(maxConcurrent)),
		config:        c,
		SugaredLogger: logger,
	}
}

// FlushFunc is any function that flushes
type FlushFunc func(context.Context) error

// Do executes the flusher function in a goroutine
func (f *Flusher) Do(flush FlushFunc) {
	// Wait until we have free flusher goroutines
	if err := f.sem.Acquire(f.ctx, 1); err != nil {
		// Context cancelled
		return
	}

	// Start a new flusher goroutine
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer f.sem.Release(1)
		f.flushWithRetry(f.ctx, flush)
	}()
}

// Stop cancels all the in-progress flushers and waits until they have returned
func (f *Flusher) Stop() {
	f.cancel()
	f.wg.Wait()
}

func (f *Flusher) flushWithRetry(ctx context.Context, flush FlushFunc) {
	chunkID := f.nextChunkID()
	b := f.newExponentialBackoff()
	for {
		err := flush(ctx)
		if err == nil {
			return
		}

		waitTime := b.NextBackOff()
		if waitTime == b.Stop {
			f.Errorw("Reached max backoff time during chunk flush retry. Dropping logs in chunk", "chunk_id", chunkID)
			return
		}

		// Only log the error if the context hasn't been canceled
		// This protects from flooding the logs with "context canceled" messages on clean shutdown
		select {
		case <-ctx.Done():
			return
		default:
			f.Warnw("Failed flushing chunk. Waiting before retry", "error", err, "wait_time", waitTime)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(waitTime):
		}
	}
}

func (f *Flusher) nextChunkID() uint64 {
	return atomic.AddUint64(&f.chunkIDCounter, 1)
}

// newExponentialBackoff returns a default ExponentialBackOff
func (f *Flusher) newExponentialBackoff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     50 * time.Millisecond,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         f.config.MaxRetryInterval,
		MaxElapsedTime:      f.config.MaxElapsedTime,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}
