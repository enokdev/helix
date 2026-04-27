package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestCacheInterceptorSingleFlightPatternColdCache tests AC 1: Multiple concurrent requests on cold cache.
func TestCacheInterceptorSingleFlightPatternColdCache(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handler := func(ctx Context) error {
		handlerCalls.Add(1)
		time.Sleep(10 * time.Millisecond) // Simulate slow handler.
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "value"})
	}

	// Launch N concurrent requests on the same cold cache key.
	const numConcurrent = 50
	wg := sync.WaitGroup{}
	wg.Add(numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			defer wg.Done()
			mockCtx := &mockContext{method: "GET"}
			// Use a wrapper to provide OriginalURL and Status methods.
			wrappedCtx := &cacheTestContext{
				mockContext: mockCtx,
				originalURL: "/api/data",
			}
			// All requests use the SAME interceptor instance.
			err := interceptor.Intercept(wrappedCtx, handler)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, wrappedCtx.statusCode)
		}()
	}

	wg.Wait()

	// AC 1: Verify only one handler call was made.
	calls := handlerCalls.Load()
	assert.Equal(t, int32(1), calls, "Expected handler to be called once, but was called %d times", calls)

	// Verify cache has one entry.
	assert.Equal(t, 1, store.Size())
}

// cacheTestContext wraps mockContext and adds required methods for cache interceptor tests.
type cacheTestContext struct {
	*mockContext
	originalURL string
	statusCode  int
}

func (c *cacheTestContext) Path() string {
	return "/"
}

func (c *cacheTestContext) Param(key string) string {
	return ""
}

func (c *cacheTestContext) Query(key string) string {
	return ""
}

func (c *cacheTestContext) Header(key string) string {
	return ""
}

func (c *cacheTestContext) IP() string {
	return ""
}

func (c *cacheTestContext) Body() []byte {
	return nil
}

func (c *cacheTestContext) OriginalURL() string {
	return c.originalURL
}

func (c *cacheTestContext) Status(code int) {
	c.statusCode = code
	c.mockContext.Status(code)
}

func (c *cacheTestContext) GetHeader(key string) string {
	return ""
}

func (c *cacheTestContext) FormValue(key string) string {
	return ""
}

func (c *cacheTestContext) Locals(key string, value ...any) any {
	return nil
}

func (c *cacheTestContext) SetLocals(key string, value interface{}) {
	// no-op
}

func (c *cacheTestContext) Send(body []byte) error {
	return nil
}

// TestCacheInterceptorSingleFlightWaitGroup tests that requests wait correctly for in-flight result.
func TestCacheInterceptorSingleFlightWaitGroup(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handlerDone := make(chan struct{})
	handler := func(ctx Context) error {
		handlerCalls.Add(1)
		<-handlerDone // Block handler until signaled.
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	results := make(chan struct {
		err    error
		status int
	}, 50)

	// Launch requests in goroutines; they'll block on handler.
	for i := 0; i < 10; i++ {
		go func() {
			mockCtx := &mockContext{method: "GET"}
			wrappedCtx := &cacheTestContext{
				mockContext: mockCtx,
				originalURL: "/api/test",
			}
			err := interceptor.Intercept(wrappedCtx, handler)
			results <- struct {
				err    error
				status int
			}{err, wrappedCtx.statusCode}
		}()
	}

	// Allow handler to complete.
	time.Sleep(50 * time.Millisecond)
	close(handlerDone)

	// Collect results.
	for i := 0; i < 10; i++ {
		result := <-results
		assert.NoError(t, result.err)
		assert.Equal(t, http.StatusOK, result.status)
	}

	// Only 1 handler call.
	assert.Equal(t, int32(1), handlerCalls.Load())
}

// TestCacheInterceptorHitAndMiss tests basic cache hit/miss metrics.
func TestCacheInterceptorHitAndMiss(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handler := func(ctx Context) error {
		handlerCalls.Add(1)
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "cached"})
	}

	// First request: miss.
	mockCtx := &mockContext{method: "GET"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/test",
	}
	err := interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), store.misses.Load())
	assert.Equal(t, uint64(0), store.hits.Load())

	// Second request: hit.
	mockCtx = &mockContext{method: "GET"}
	wrappedCtx = &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/test",
	}
	err = interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), store.misses.Load())
	assert.Equal(t, uint64(1), store.hits.Load())
	assert.Equal(t, int32(1), handlerCalls.Load())
}

// TestCacheInterceptorMaxSize tests AC 2: Cache size limit enforcement.
func TestCacheInterceptorMaxSize(t *testing.T) {
	maxSize := 10
	store := newCacheStore()
	store.maxSize = maxSize
	store.evictionStrategy = "lru"
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, maxSize, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Fill cache beyond maxSize.
	for i := 0; i < 15; i++ {
		url := fmt.Sprintf("/api/item%d", i)
		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: url,
		}
		err := interceptor.Intercept(wrappedCtx, handler)
		assert.NoError(t, err)
	}

	// AC 2: Cache size should not exceed max.
	assert.LessOrEqual(t, len(store.entries), maxSize, "Cache size should not exceed max")
	assert.Greater(t, len(store.entries), 0, "Cache should not be empty")
}

// TestCacheInterceptorLRUEviction tests LRU eviction strategy.
func TestCacheInterceptorLRUEviction(t *testing.T) {
	store := newCacheStore()
	store.maxSize = 3
	store.evictionStrategy = "lru"
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Populate cache with 3 entries.
	for i := 1; i <= 3; i++ {
		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: fmt.Sprintf("/item%d", i),
		}
		_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(wrappedCtx, handler)
	}

	assert.Equal(t, 3, len(store.entries))

	// Access /item1 to update its access time.
	mockCtx := &mockContext{method: "GET"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/item1",
	}
	_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(wrappedCtx, handler)

	// Add a 4th entry; LRU should evict /item2 (least recently used).
	mockCtx = &mockContext{method: "GET"}
	wrappedCtx = &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/item4",
	}
	_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(wrappedCtx, handler)

	// Cache should still be at max size.
	assert.Equal(t, 3, len(store.entries))

	// /item1 should still be in cache (recently accessed).
	_, hasItem1 := store.entries["GET /item1"]
	assert.True(t, hasItem1, "/item1 should not be evicted (recently accessed)")

	// /item4 should be in cache (newly added).
	_, hasItem4 := store.entries["GET /item4"]
	assert.True(t, hasItem4, "/item4 should be in cache")
}

// TestCacheInterceptorFIFOEviction tests FIFO eviction strategy.
func TestCacheInterceptorFIFOEviction(t *testing.T) {
	store := newCacheStore()
	store.maxSize = 3
	store.evictionStrategy = "fifo"
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Populate cache with 3 entries.
	for i := 1; i <= 3; i++ {
		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: fmt.Sprintf("/item%d", i),
		}
		_ = store.newInterceptor(5*time.Minute, 3, "fifo").Intercept(wrappedCtx, handler)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps.
	}

	assert.Equal(t, 3, len(store.entries))

	// Add a 4th entry; FIFO should evict /item1 (oldest by insertion time).
	mockCtx := &mockContext{method: "GET"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/item4",
	}
	_ = store.newInterceptor(5*time.Minute, 3, "fifo").Intercept(wrappedCtx, handler)

	assert.Equal(t, 3, len(store.entries))

	// /item1 should be evicted.
	_, hasItem1 := store.entries["GET /item1"]
	assert.False(t, hasItem1, "/item1 should be evicted (oldest insertion)")

	// /item4 should be in cache.
	_, hasItem4 := store.entries["GET /item4"]
	assert.True(t, hasItem4, "/item4 should be in cache")
}

// TestCacheInterceptorProactiveSweep tests AC 2: Proactive expiration sweep.
func TestCacheInterceptorProactiveSweep(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Add entries with short TTL.
	ttl := 100 * time.Millisecond
	for i := 0; i < 5; i++ {
		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: fmt.Sprintf("/item%d", i),
		}
		_ = store.newInterceptor(ttl, 100, "lru").Intercept(wrappedCtx, handler)
	}

	assert.Equal(t, 5, store.Size(), "Should have 5 entries initially")

	// Wait for entries to expire and sweep to remove them.
	// DefaultSweepInterval is 30s, so we need to manually trigger sweep for the test.
	time.Sleep(150 * time.Millisecond) // Wait for entries to expire
	store.sweep()                        // Manually trigger sweep

	// Sweep should have removed expired entries.
	assert.Less(t, store.Size(), 5, "Sweep should have removed expired entries")
}

// TestCacheInterceptorConfigParsing tests AC 3: Config parsing for cache duration, max, strategy.
func TestCacheInterceptorConfigParsing(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedErr bool
	}{
		{"Basic duration", "5m", false},
		{"Duration with max", "5m:max=500", false},
		{"Duration with max and LRU", "5m:max=500:strategy=lru", false},
		{"Duration with max and FIFO", "5m:max=200:strategy=fifo", false},
		{"Invalid duration", "invalid", true},
		{"Invalid max", "5m:max=abc", true},
		{"Invalid strategy", "5m:max=500:strategy=invalid", true},
		{"Zero TTL", "0s", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newCacheStore()
			defer store.Stop()

			factory := cacheInterceptorFactory(store)
			interceptor, err := factory(tt.config)

			if tt.expectedErr {
				assert.Error(t, err, "Expected error for config: %s", tt.config)
				return
			}

			assert.NoError(t, err, "Unexpected error for config: %s", tt.config)
			assert.NotNil(t, interceptor)
		})
	}
}

// TestCacheInterceptorExpirationLazyDeletion tests that expired entries are cleaned on access.
func TestCacheInterceptorExpirationLazyDeletion(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	ttl := 50 * time.Millisecond
	interceptor := store.newInterceptor(ttl, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Add entry.
	mockCtx := &mockContext{method: "GET"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/test",
	}
	_ = interceptor.Intercept(wrappedCtx, handler)
	assert.Equal(t, 1, len(store.entries))

	// Wait for expiration.
	time.Sleep(100 * time.Millisecond)

	// Access the expired entry; it should be lazily deleted.
	mockCtx = &mockContext{method: "GET"}
	wrappedCtx = &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/test",
	}
	handlerCallCount := atomic.Int32{}
	err := interceptor.Intercept(wrappedCtx, func(ctx Context) error {
		handlerCallCount.Add(1)
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "new"})
	})

	assert.NoError(t, err)
	// Handler should have been called (cache miss due to expiration).
	assert.Equal(t, int32(1), handlerCallCount.Load())
}

// TestCacheInterceptorNonGetMethodsNotCached tests that only GET methods are cached.
func TestCacheInterceptorNonGetMethodsNotCached(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handler := func(ctx Context) error {
		handlerCalls.Add(1)
		ctx.Status(http.StatusCreated)
		return ctx.JSON(map[string]string{"id": "123"})
	}

	// POST request should not be cached.
	mockCtx := &mockContext{method: "POST"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/api/items",
	}
	err := interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), handlerCalls.Load())

	// Repeat POST; should not be cached.
	mockCtx = &mockContext{method: "POST"}
	wrappedCtx = &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/api/items",
	}
	err = interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), handlerCalls.Load(), "POST should not be cached")

	// Cache should be empty.
	assert.Equal(t, 0, len(store.entries), "Cache should be empty for non-GET methods")
}

// TestCacheInterceptorErrorResponsesNotCached tests that error responses (5xx, 4xx) are not cached.
func TestCacheInterceptorErrorResponsesNotCached(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handler := func(ctx Context) error {
		handlerCalls.Add(1)
		ctx.Status(http.StatusInternalServerError)
		return ctx.JSON(map[string]string{"error": "server error"})
	}

	// First request.
	mockCtx := &mockContext{method: "GET"}
	wrappedCtx := &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/api/error",
	}
	err := interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, int32(1), handlerCalls.Load())

	// Second request; should not hit cache.
	mockCtx = &mockContext{method: "GET"}
	wrappedCtx = &cacheTestContext{
		mockContext: mockCtx,
		originalURL: "/api/error",
	}
	err = interceptor.Intercept(wrappedCtx, handler)
	assert.NoError(t, err)
	assert.Equal(t, int32(2), handlerCalls.Load(), "Error responses should not be cached")

	// Cache should be empty.
	assert.Equal(t, 0, len(store.entries), "Error responses should not be cached")
}

// TestCacheInterceptorSuccessRangeOnlyCached tests that only 2xx responses are cached.
func TestCacheInterceptorSuccessRangeOnlyCached(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	// Test that 2xx responses are cached.
	testCases := []struct {
		statusCode  int
		shouldCache bool
	}{
		{http.StatusOK, true},
		{http.StatusCreated, true},
		{http.StatusAccepted, true},
		{http.StatusNoContent, true},
		{http.StatusMultipleChoices, false},
		{http.StatusBadRequest, false},
		{http.StatusNotFound, false},
		{http.StatusInternalServerError, false},
	}

	for _, tc := range testCases {
		store.entries = make(map[string]cacheEntry) // Clear cache for each test.

		interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

		handler := func(ctx Context) error {
			ctx.Status(tc.statusCode)
			if tc.statusCode < 300 {
				return ctx.JSON(map[string]string{"data": "test"})
			}
			return ctx.JSON(map[string]string{"error": "error"})
		}

		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: "/test",
		}
		_ = interceptor.Intercept(wrappedCtx, handler)

		shouldCache := tc.statusCode >= 200 && tc.statusCode < 300
		isCached := len(store.entries) > 0

		assert.Equal(t, shouldCache, isCached, "Status %d: shouldCache=%v, isCached=%v", tc.statusCode, shouldCache, isCached)
	}
}

// TestCacheInterceptorConcurrentAccessNoRace tests that concurrent cache access is race-free.
// Run with -race flag: go test -race ./web -run TestCacheInterceptorConcurrentAccessNoRace
func TestCacheInterceptorConcurrentAccessNoRace(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Concurrent reads and writes.
	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				url := fmt.Sprintf("/item%d", n%5)
				mockCtx := &mockContext{method: "GET"}
				wrappedCtx := &cacheTestContext{
					mockContext: mockCtx,
					originalURL: url,
				}
				_ = interceptor.Intercept(wrappedCtx, handler)
			}
		}(i)
	}
	wg.Wait()

	// Should complete without race detector errors.
	assert.Greater(t, len(store.entries), 0)
}

// TestCacheStoreStop tests graceful shutdown of cache sweep goroutine.
func TestCacheStoreStop(t *testing.T) {
	store := newCacheStore()

	// Verify sweep goroutine is running.
	assert.NotNil(t, store.sweepTicker)

	// Stop should not panic and should complete quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := store.Stop()
	assert.NoError(t, err)

	// Verify ticker is stopped.
	select {
	case <-store.sweepDone:
		// Good: sweep routine exited.
	case <-ctx.Done():
		t.Fatal("Sweep routine did not exit")
	}
}

// TestCacheInterceptorResponseRecorderCapture tests that responseRecorder properly captures responses.
func TestCacheInterceptorResponseRecorderCapture(t *testing.T) {
	mockCtx := &mockContext{method: "GET", jsonErr: nil}
	recorder := &responseRecorder{
		Context: mockCtx,
		status:  0,
	}

	// Set status.
	recorder.Status(http.StatusOK)
	assert.Equal(t, http.StatusOK, recorder.status)

	// Write JSON.
	data := map[string]string{"key": "value"}
	err := recorder.JSON(data)
	assert.NoError(t, err)
	assert.True(t, recorder.wroteJSON)

	// Verify body is properly marshaled.
	assert.NotEmpty(t, recorder.body)
	var parsed map[string]string
	err = json.Unmarshal(recorder.body, &parsed)
	assert.NoError(t, err)
	assert.Equal(t, "value", parsed["key"])
}

// TestCacheInterceptorCacheSizeThreshold tests that cache respects 1.1x max size margin.
func TestCacheInterceptorCacheSizeThreshold(t *testing.T) {
	store := newCacheStore()
	store.maxSize = 100
	store.evictionStrategy = "lru"
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "value"})
	}

	// Add many entries beyond max.
	for i := 0; i < 200; i++ {
		mockCtx := &mockContext{method: "GET"}
		wrappedCtx := &cacheTestContext{
			mockContext: mockCtx,
			originalURL: fmt.Sprintf("/item%d", i),
		}
		_ = store.newInterceptor(5*time.Minute, 100, "lru").Intercept(wrappedCtx, handler)
	}

	// Cache size should not exceed max * 1.1 (110).
	upperBound := int(float64(store.maxSize) * 1.1)
	assert.LessOrEqual(t, len(store.entries), upperBound, "Cache size exceeded threshold")
	assert.Greater(t, len(store.entries), 0, "Cache should not be empty")
}
