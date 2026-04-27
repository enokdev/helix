package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// DefaultMaxCacheEntries is the default maximum number of entries in the cache.
	DefaultMaxCacheEntries = 1000
	// DefaultEvictionStrategy is the default eviction strategy (LRU).
	DefaultEvictionStrategy = "lru"
	// DefaultSweepInterval is the default interval for proactive cache sweep.
	DefaultSweepInterval = 30 * time.Second
	// MaxCacheBodySize is the maximum response body size allowed in the cache (1MB).
	MaxCacheBodySize = 1024 * 1024
)

type cacheEntry struct {
	status    int
	body      []byte // JSON-encoded response body; stored as bytes to prevent mutation.
	expiresAt time.Time
	timestamp time.Time // For LRU tracking (last access) or FIFO (insertion).
}

type flightResult struct {
	status int
	body   []byte
	err    error
}

type flightKey struct {
	method string
	url    string
}

// cacheStore implements a production-grade cache storage.
type cacheStore struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry

	// Single-flight pattern tracking.
	flightMu sync.RWMutex
	inflight map[flightKey]*flightRequest

	// Global default configuration (overridden per-interceptor).
	maxSize          int
	evictionStrategy string
	sweepInterval    time.Duration

	lastAccessTimes   map[string]time.Time // For LRU tracking.
	lastAccessTimesMu sync.RWMutex
	sweepTicker       *time.Ticker
	sweepCtx          context.Context
	sweepCancel       context.CancelFunc
	sweepDone         chan struct{}

	// Metrics.
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

// flightRequest tracks a single in-flight request.
type flightRequest struct {
	done   chan struct{}
	result flightResult
}

func newCacheStore() *cacheStore {
	sweepCtx, sweepCancel := context.WithCancel(context.Background())
	store := &cacheStore{
		entries:          make(map[string]cacheEntry),
		inflight:         make(map[flightKey]*flightRequest),
		maxSize:          DefaultMaxCacheEntries,
		evictionStrategy: DefaultEvictionStrategy,
		sweepInterval:    DefaultSweepInterval,
		lastAccessTimes:  make(map[string]time.Time),
		sweepTicker:      time.NewTicker(DefaultSweepInterval),
		sweepCtx:         sweepCtx,
		sweepCancel:      sweepCancel,
		sweepDone:        make(chan struct{}),
	}
	go store.sweepRoutine()
	return store
}

func (s *cacheStore) sweepRoutine() {
	defer close(s.sweepDone)
	for {
		select {
		case <-s.sweepCtx.Done():
			return
		case <-s.sweepTicker.C:
			s.sweep()
		}
	}
}

func (s *cacheStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
			s.lastAccessTimesMu.Lock()
			delete(s.lastAccessTimes, key)
			s.lastAccessTimesMu.Unlock()
			expiredCount++
		}
	}

	if expiredCount > 0 {
		slog.Debug("cache sweep: removed expired entries", "count", expiredCount, "size", len(s.entries))
	}

	if len(s.entries) > s.maxSize {
		toEvict := len(s.entries) - s.maxSize
		evicted := s.evictEntries(toEvict, s.evictionStrategy)
		if evicted > 0 {
			slog.Debug("cache sweep: evicted entries", "count", evicted, "strategy", s.evictionStrategy, "size", len(s.entries))
			s.evictions.Add(uint64(evicted))
		}
	}
}

// evictEntries removes N entries using O(N log N) sorting.
func (s *cacheStore) evictEntries(toEvict int, strategy string) int {
	if len(s.entries) <= 0 {
		return 0
	}

	type candidate struct {
		key string
		t   time.Time
	}
	candidates := make([]candidate, 0, len(s.entries))

	if strategy == "lru" {
		s.lastAccessTimesMu.RLock()
		for k, t := range s.lastAccessTimes {
			candidates = append(candidates, candidate{k, t})
		}
		s.lastAccessTimesMu.RUnlock()
	} else {
		for k, e := range s.entries {
			candidates = append(candidates, candidate{k, e.timestamp})
		}
	}

	// Efficient O(N log N) sort.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].t.Before(candidates[j].t)
	})

	evicted := 0
	for i := 0; i < toEvict && i < len(candidates); i++ {
		key := candidates[i].key
		delete(s.entries, key)
		s.lastAccessTimesMu.Lock()
		delete(s.lastAccessTimes, key)
		s.lastAccessTimesMu.Unlock()
		evicted++
	}

	return evicted
}

func (s *cacheStore) newInterceptor(ttl time.Duration, maxSize int, strategy string) Interceptor {
	return InterceptorFunc(func(ctx Context, next HandlerFunc) error {
		if ctx.Method() != http.MethodGet {
			return next(ctx)
		}

		key := ctx.Method() + " " + ctx.OriginalURL()
		now := time.Now()

		if entry, ok := s.getCached(key, now); ok {
			s.hits.Add(1)
			ctx.Status(entry.status)
			var body any
			if err := json.Unmarshal(entry.body, &body); err != nil {
				return fmt.Errorf("web: cache unmarshal: %w", err)
			}
			return ctx.JSON(body)
		}

		s.misses.Add(1)

		fk := flightKey{method: ctx.Method(), url: ctx.OriginalURL()}
		flight, isInflight := s.tryRegisterInflight(fk)
		if isInflight {
			// Wait for result OR context cancellation.
			select {
			case <-flight.done:
				if flight.result.err != nil {
					return flight.result.err
				}
				ctx.Status(flight.result.status)
				var body any
				if err := json.Unmarshal(flight.result.body, &body); err != nil {
					return fmt.Errorf("web: cache unmarshal (flight): %w", err)
				}
				return ctx.JSON(body)
			case <-ctx.Context().Done():
				return ctx.Context().Err()
			}
		}

		defer s.unregisterInflight(fk)
		recorder := &responseRecorder{BaseContext: ctx}
		err := next(recorder)
		if err != nil {
			flight.result.err = err
			close(flight.done)
			return err
		}

		if recorder.wroteJSON && recorder.status >= 200 && recorder.status < 300 {
			// OOM Guard: only cache reasonably sized bodies.
			if len(recorder.body) <= MaxCacheBodySize {
				s.setCached(key, cacheEntry{
					status:    recorder.status,
					body:      recorder.body,
					expiresAt: now.Add(ttl),
					timestamp: now,
				}, maxSize, strategy)
				flight.result.status = recorder.status
				flight.result.body = recorder.body
			}
		}

		close(flight.done)
		return nil
	})
}

func (s *cacheStore) getCached(key string, now time.Time) (cacheEntry, bool) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return cacheEntry{}, false
	}

	if now.After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return cacheEntry{}, false
	}

	s.lastAccessTimesMu.Lock()
	s.lastAccessTimes[key] = now
	s.lastAccessTimesMu.Unlock()

	return entry, true
}

func (s *cacheStore) setCached(key string, entry cacheEntry, maxSize int, strategy string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.entries) >= maxSize {
		if evicted := s.evictEntries(1, strategy); evicted > 0 {
			s.evictions.Add(uint64(evicted))
		}
	}

	s.entries[key] = entry
	s.lastAccessTimesMu.Lock()
	s.lastAccessTimes[key] = entry.timestamp
	s.lastAccessTimesMu.Unlock()
}

func (s *cacheStore) tryRegisterInflight(fk flightKey) (*flightRequest, bool) {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if existing, ok := s.inflight[fk]; ok {
		return existing, true
	}
	flight := &flightRequest{done: make(chan struct{})}
	s.inflight[fk] = flight
	return flight, false
}

func (s *cacheStore) unregisterInflight(fk flightKey) {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	delete(s.inflight, fk)
}

func (s *cacheStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *cacheStore) Stop() error {
	s.sweepTicker.Stop()
	s.sweepCancel()
	<-s.sweepDone
	return nil
}

func cacheInterceptorFactory(store *cacheStore) InterceptorFactory {
	return func(argument string) (Interceptor, error) {
		parts := strings.Split(argument, ":")
		if len(parts) == 0 || parts[0] == "" {
			return nil, fmt.Errorf("web: cache config: empty duration")
		}

		ttl, err := time.ParseDuration(parts[0])
		if err != nil || ttl <= 0 {
			return nil, fmt.Errorf("web: cache duration %q: %w", parts[0], ErrInvalidDirective)
		}

		maxSize := DefaultMaxCacheEntries
		strategy := DefaultEvictionStrategy

		for i := 1; i < len(parts); i++ {
			p := parts[i]
			if strings.HasPrefix(p, "max=") {
				n, err := parseIntValue(strings.TrimPrefix(p, "max="))
				if err != nil || n <= 0 {
					return nil, fmt.Errorf("web: cache max %q: %w", p, ErrInvalidDirective)
				}
				maxSize = n
			} else if p == "lru" || p == "fifo" {
				strategy = p
			} else {
				return nil, fmt.Errorf("web: cache config: unknown option %q", p)
			}
		}

		slog.Debug("cache interceptor configured", "ttl", ttl, "max", maxSize, "strategy", strategy)
		return store.newInterceptor(ttl, maxSize, strategy), nil
	}
}

func parseIntValue(s string) (int, error) {
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		n = n*10 + int(ch-'0')
	}
	return n, nil
}

type responseRecorder struct {
	BaseContext Context
	status      int
	body        []byte
	wroteJSON   bool
}

func (r *responseRecorder) Method() string      { return r.BaseContext.Method() }
func (r *responseRecorder) Path() string        { return r.BaseContext.Path() }
func (r *responseRecorder) OriginalURL() string { return r.BaseContext.OriginalURL() }
func (r *responseRecorder) Param(key string) string {
	return r.BaseContext.Param(key)
}
func (r *responseRecorder) Query(key string) string {
	return r.BaseContext.Query(key)
}
func (r *responseRecorder) Header(key string) string {
	return r.BaseContext.Header(key)
}
func (r *responseRecorder) IP() string   { return r.BaseContext.IP() }
func (r *responseRecorder) Body() []byte { return r.BaseContext.Body() }
func (r *responseRecorder) Status(code int) {
	r.status = code
	r.BaseContext.Status(code)
}
func (r *responseRecorder) SetHeader(key, value string) {
	r.BaseContext.SetHeader(key, value)
}
func (r *responseRecorder) Send(body []byte) error {
	return r.BaseContext.Send(body)
}
func (r *responseRecorder) JSON(body any) error {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("web: response recorder marshal: %w", err)
	}
	r.body = encoded
	r.wroteJSON = true
	return r.BaseContext.JSON(body)
}
func (r *responseRecorder) Context() context.Context {
	return r.BaseContext.Context()
}
func (r *responseRecorder) Locals(key string, value ...any) any {
	return r.BaseContext.Locals(key, value...)
}
