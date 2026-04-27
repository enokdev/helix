package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
)

type cacheEntry struct {
	status    int
	body      []byte // JSON-encoded response body; stored as bytes to prevent mutation by the caller.
	expiresAt time.Time
	timestamp time.Time // For LRU tracking.
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

// cacheStore implements a production-grade cache with:
// - Single-flight pattern (AC 1): N concurrent requests on cold cache key execute handler once
// - Max size + proactive sweep (AC 2): Cache never exceeds maxSize; expired entries swept proactively
// - Flexible configuration (AC 3): Supports "ttl:max=N:strategy" parsing and metrics
type cacheStore struct {
	// Entries: main cache storage, protected by mu.
	mu      sync.RWMutex
	entries map[string]cacheEntry

	// Single-flight pattern: track in-flight requests by (method, url).
	flightMu      sync.RWMutex
	inflight      map[flightKey]*flightRequest
	flightResults map[flightKey]flightResult

	// Cache configuration.
	maxSize             int
	evictionStrategy    string
	sweepInterval       time.Duration
	lastAccessTimes     map[string]time.Time // For LRU tracking.
	lastAccessTimesMu   sync.RWMutex
	sweepTicker         *time.Ticker
	sweepCtx            context.Context
	sweepCancel         context.CancelFunc
	sweepDone           chan struct{}

	// Metrics.
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

// flightRequest tracks a single in-flight request result.
type flightRequest struct {
	done   chan struct{}
	result flightResult
}

func newCacheStore() *cacheStore {
	sweepCtx, sweepCancel := context.WithCancel(context.Background())
	store := &cacheStore{
		entries:           make(map[string]cacheEntry),
		inflight:          make(map[flightKey]*flightRequest),
		flightResults:     make(map[flightKey]flightResult),
		maxSize:           DefaultMaxCacheEntries,
		evictionStrategy:  DefaultEvictionStrategy,
		sweepInterval:     DefaultSweepInterval,
		lastAccessTimes:   make(map[string]time.Time),
		sweepTicker:       time.NewTicker(DefaultSweepInterval),
		sweepCtx:          sweepCtx,
		sweepCancel:       sweepCancel,
		sweepDone:         make(chan struct{}),
	}
	go store.sweepRoutine()
	return store
}

// sweepRoutine periodically evicts expired entries and enforces max size.
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

// sweep removes expired entries and evicts oldest entries if cache exceeds maxSize.
func (s *cacheStore) sweep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	// Remove expired entries.
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

	// Enforce max size with eviction if necessary.
	if len(s.entries) > s.maxSize {
		toEvict := len(s.entries) - s.maxSize
		evicted := s.evictEntries(toEvict, now)
		if evicted > 0 {
			slog.Debug("cache sweep: evicted entries", "count", evicted, "strategy", s.evictionStrategy, "size", len(s.entries))
			s.evictions.Add(uint64(evicted))
		}
	}
}

// evictEntries removes toEvict entries using the configured strategy (LRU or FIFO).
func (s *cacheStore) evictEntries(toEvict int, now time.Time) int {
	if len(s.entries) == 0 {
		return 0
	}

	evicted := 0
	if s.evictionStrategy == "lru" {
		// LRU: remove least recently accessed entries.
		type keyTime struct {
			key       string
			timestamp time.Time
		}
		var candidates []keyTime

		s.lastAccessTimesMu.RLock()
		for key, accessTime := range s.lastAccessTimes {
			candidates = append(candidates, keyTime{key, accessTime})
		}
		s.lastAccessTimesMu.RUnlock()

		// Sort by access time (ascending = least recent first).
		for i := 0; i < len(candidates)-1; i++ {
			for j := i + 1; j < len(candidates); j++ {
				if candidates[i].timestamp.After(candidates[j].timestamp) {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}

		for i := 0; i < toEvict && i < len(candidates); i++ {
			key := candidates[i].key
			delete(s.entries, key)
			s.lastAccessTimesMu.Lock()
			delete(s.lastAccessTimes, key)
			s.lastAccessTimesMu.Unlock()
			evicted++
		}
	} else {
		// FIFO: remove oldest entries by insertion time.
		type keyTime struct {
			key       string
			timestamp time.Time
		}
		var candidates []keyTime

		for key, entry := range s.entries {
			candidates = append(candidates, keyTime{key, entry.timestamp})
		}

		// Sort by insertion time (ascending = oldest first).
		for i := 0; i < len(candidates)-1; i++ {
			for j := i + 1; j < len(candidates); j++ {
				if candidates[i].timestamp.After(candidates[j].timestamp) {
					candidates[i], candidates[j] = candidates[j], candidates[i]
				}
			}
		}

		for i := 0; i < toEvict && i < len(candidates); i++ {
			key := candidates[i].key
			delete(s.entries, key)
			s.lastAccessTimesMu.Lock()
			delete(s.lastAccessTimes, key)
			s.lastAccessTimesMu.Unlock()
			evicted++
		}
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

		// Check cache first.
		if entry, ok := s.getCached(key, now); ok {
			s.hits.Add(1)
			ctx.Status(entry.status)
			var body any
			if err := json.Unmarshal(entry.body, &body); err != nil {
				return fmt.Errorf("web: cache interceptor unmarshal: %w", err)
			}
			return ctx.JSON(body)
		}

		s.misses.Add(1)

		// Single-flight: try to register as the handler for this request.
		fk := flightKey{method: ctx.Method(), url: ctx.OriginalURL()}
		flight, isInflight := s.tryRegisterInflight(fk)
		if isInflight {
			// Another request is already fetching this key. Wait for its result.
			<-flight.done
			if flight.result.err != nil {
				return flight.result.err
			}
			ctx.Status(flight.result.status)
			var body any
			if err := json.Unmarshal(flight.result.body, &body); err != nil {
				return fmt.Errorf("web: cache interceptor unmarshal (flight): %w", err)
			}
			return ctx.JSON(body)
		}

		// We own this flight. Execute the handler.
		defer s.unregisterInflight(fk)
		recorder := &responseRecorder{Context: ctx}
		err := next(recorder)
		if err != nil {
			flight.result.err = err
			close(flight.done)
			return err
		}

		// Cache the response if it's successful.
		if recorder.wroteJSON && recorder.status >= http.StatusOK && recorder.status < http.StatusMultipleChoices {
			s.setCached(key, cacheEntry{
				status:    recorder.status,
				body:      recorder.body,
				expiresAt: now.Add(ttl),
				timestamp: now,
			})
			flight.result.status = recorder.status
			flight.result.body = recorder.body
		}

		close(flight.done)
		return nil
	})
}

// getCached retrieves an entry from the cache, updating access time for LRU.
func (s *cacheStore) getCached(key string, now time.Time) (cacheEntry, bool) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return cacheEntry{}, false
	}

	if now.After(entry.expiresAt) {
		s.mu.Lock()
		// Re-check under write lock.
		if current, ok := s.entries[key]; ok && now.After(current.expiresAt) {
			delete(s.entries, key)
		}
		s.mu.Unlock()
		return cacheEntry{}, false
	}

	// Update access time for LRU tracking.
	s.lastAccessTimesMu.Lock()
	s.lastAccessTimes[key] = now
	s.lastAccessTimesMu.Unlock()

	return entry, true
}

// setCached stores an entry in the cache, enforcing maxSize.
func (s *cacheStore) setCached(key string, entry cacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If cache is at max size, evict before inserting.
	if len(s.entries) >= s.maxSize {
		toEvict := 1
		if evicted := s.evictEntries(toEvict, time.Now()); evicted > 0 {
			slog.Debug("cache set: evicted entry before insert", "strategy", s.evictionStrategy)
			s.evictions.Add(uint64(evicted))
		}
	}

	s.entries[key] = entry

	s.lastAccessTimesMu.Lock()
	s.lastAccessTimes[key] = entry.timestamp
	s.lastAccessTimesMu.Unlock()
}

// tryRegisterInflight atomically checks if a flight exists and registers if not.
// Returns (flight, isInflight) where if isInflight=true, another request is already processing.
// If isInflight=false, this request owns the flight and must call unregisterInflight when done.
func (s *cacheStore) tryRegisterInflight(fk flightKey) (*flightRequest, bool) {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	if existing, ok := s.inflight[fk]; ok {
		return existing, true // Another request is in flight
	}
	// Register ourselves
	flight := &flightRequest{done: make(chan struct{})}
	s.inflight[fk] = flight
	return flight, false // We own this flight
}

// unregisterInflight removes a request from the in-flight tracking.
func (s *cacheStore) unregisterInflight(fk flightKey) {
	s.flightMu.Lock()
	defer s.flightMu.Unlock()
	delete(s.inflight, fk)
}

// Stop gracefully stops the cache sweep goroutine.
// Size returns the current number of entries in the cache under lock.
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

// cacheInterceptorFactory returns a factory for the built-in "cache" interceptor.
//
// The cache is shared across all requests: responses are keyed only by HTTP method
// and URL, not by authenticated user or session. Register this factory under a
// per-user key scheme or disable it for endpoints that return user-scoped data.
//
// Configuration syntax: "duration[:max=N[:strategy]]"
// - duration: Required. Example: "5m", "1h", "30s"
// - max: Optional. Maximum cache entries (default: 1000)
// - strategy: Optional. Eviction strategy: "lru" (default) or "fifo"
//
// Examples:
// - "5m" → 5 minute TTL, 1000 entry max, LRU eviction
// - "5m:max=500" → 5 minute TTL, 500 entry max, LRU eviction
// - "5m:max=2000:fifo" → 5 minute TTL, 2000 entry max, FIFO eviction
func cacheInterceptorFactory(store *cacheStore) InterceptorFactory {
	return func(argument string) (Interceptor, error) {
		parts := strings.Split(argument, ":")
		if len(parts) == 0 || parts[0] == "" {
			return nil, fmt.Errorf("web: parse cache config %q: empty config", argument)
		}

		// Parse TTL from first part.
		ttl, err := time.ParseDuration(parts[0])
		if err != nil || ttl <= 0 {
			return nil, fmt.Errorf("web: parse cache duration %q: %w", parts[0], ErrInvalidDirective)
		}

		// Parse optional max size and strategy.
		maxSize := store.maxSize
		strategy := store.evictionStrategy

		for i := 1; i < len(parts); i++ {
			kv := strings.SplitN(parts[i], "=", 2)
			if len(kv) != 2 {
				return nil, fmt.Errorf("web: parse cache config: invalid key=value pair %q", parts[i])
			}

			key, value := kv[0], kv[1]
			switch key {
			case "max":
				n, err := parseIntValue(value)
				if err != nil || n <= 0 {
					return nil, fmt.Errorf("web: parse cache max %q: %w", value, ErrInvalidDirective)
				}
				maxSize = n
			case "strategy":
				if value != "lru" && value != "fifo" {
					return nil, fmt.Errorf("web: parse cache strategy %q: must be 'lru' or 'fifo'", value)
				}
				strategy = value
			default:
				return nil, fmt.Errorf("web: parse cache config: unknown key %q", key)
			}
		}

		slog.Debug("cache interceptor configured", "ttl", ttl, "max_size", maxSize, "strategy", strategy)
		return store.newInterceptor(ttl, maxSize, strategy), nil
	}
}

// parseIntValue parses an integer from a string.
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
	Context
	status    int
	body      []byte
	wroteJSON bool
}

func (r *responseRecorder) Status(code int) {
	r.status = code
	r.Context.Status(code)
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
	return r.Context.JSON(body)
}
