package scheduler

import (
	"context"
	"sync"
	"time"
)

// DistributedLock is an optional capability that prevents duplicate job
// execution in multi-instance deployments. Implementations must be safe
// for concurrent use.
type DistributedLock interface {
	// Acquire attempts to acquire a named lock with the given TTL.
	// Returns (true, nil) if acquired, (false, nil) if already held by
	// another instance, or (false, err) on backend failure.
	Acquire(ctx context.Context, name string, ttl time.Duration) (bool, error)
	// Release releases a previously acquired lock.
	Release(ctx context.Context, name string) error
}

// NoOpLock is a DistributedLock that always acquires (single-instance default).
type NoOpLock struct{}

func (n *NoOpLock) Acquire(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return true, nil
}

func (n *NoOpLock) Release(_ context.Context, _ string) error { return nil }

// FakeLock is a thread-safe in-memory DistributedLock for testing.
// It does not simulate TTL expiry; tests that need TTL behavior should use
// a dedicated fake that records or enforces expiration explicitly.
type FakeLock struct {
	mu   sync.Mutex
	held map[string]bool
}

func NewFakeLock() *FakeLock { return &FakeLock{held: make(map[string]bool)} }

func (f *FakeLock) Acquire(_ context.Context, name string, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held[name] {
		return false, nil
	}
	f.held[name] = true
	return true, nil
}

func (f *FakeLock) Release(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.held, name)
	return nil
}
