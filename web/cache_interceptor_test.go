package web

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// cacheTestContext wraps mockContext and adds required methods for cache interceptor tests.
type cacheTestContext struct {
	*mockContext
	originalURL string
	statusCode  int
}

func (c *cacheTestContext) Path() string        { return "/" }
func (c *cacheTestContext) Param(key string) string { return "" }
func (c *cacheTestContext) Query(key string) string { return "" }
func (c *cacheTestContext) Header(key string) string { return "" }
func (c *cacheTestContext) IP() string          { return "" }
func (c *cacheTestContext) Body() []byte        { return nil }
func (c *cacheTestContext) OriginalURL() string { return c.originalURL }
func (c *cacheTestContext) Status(code int) {
	c.statusCode = code
	c.mockContext.Status(code)
}
func (c *cacheTestContext) Locals(key string, value ...any) any { return nil }
func (c *cacheTestContext) Send(body []byte) error             { return nil }
func (c *cacheTestContext) Context() context.Context           { return context.Background() }

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

	const numConcurrent = 50
	wg := sync.WaitGroup{}
	wg.Add(numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		go func() {
			defer wg.Done()
			mockCtx := &mockContext{method: "GET"}
			wrappedCtx := &cacheTestContext{
				mockContext: mockCtx,
				originalURL: "/api/data",
			}
			err := interceptor.Intercept(wrappedCtx, handler)
			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, wrappedCtx.statusCode)
		}()
	}

	wg.Wait()
	assert.Equal(t, int32(1), handlerCalls.Load())
	assert.Equal(t, 1, store.Size())
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
		<-handlerDone
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	results := make(chan struct {
		err    error
		status int
	}, 50)

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

	time.Sleep(50 * time.Millisecond)
	close(handlerDone)

	for i := 0; i < 10; i++ {
		result := <-results
		assert.NoError(t, result.err)
		assert.Equal(t, http.StatusOK, result.status)
	}

	assert.Equal(t, int32(1), handlerCalls.Load())
}

// TestCacheInterceptorHitAndMiss tests basic cache hit/miss metrics.
func TestCacheInterceptorHitAndMiss(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "cached"})
	}

	// Miss
	m1 := &mockContext{method: "GET"}
	w1 := &cacheTestContext{mockContext: m1, originalURL: "/test"}
	_ = interceptor.Intercept(w1, handler)
	assert.Equal(t, uint64(1), store.misses.Load())
	assert.Equal(t, uint64(0), store.hits.Load())

	// Hit
	m2 := &mockContext{method: "GET"}
	w2 := &cacheTestContext{mockContext: m2, originalURL: "/test"}
	_ = interceptor.Intercept(w2, handler)
	assert.Equal(t, uint64(1), store.misses.Load())
	assert.Equal(t, uint64(1), store.hits.Load())
}

// TestCacheInterceptorMaxSize tests AC 2: Cache size limit enforcement.
func TestCacheInterceptorMaxSize(t *testing.T) {
	maxSize := 10
	store := newCacheStore()
	defer store.Stop()

	interceptor := store.newInterceptor(5*time.Minute, maxSize, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	for i := 0; i < 15; i++ {
		m := &mockContext{method: "GET"}
		w := &cacheTestContext{mockContext: m, originalURL: fmt.Sprintf("/api/item%d", i)}
		_ = interceptor.Intercept(w, handler)
	}

	assert.LessOrEqual(t, store.Size(), maxSize)
}

// TestCacheInterceptorLRUEviction tests LRU eviction strategy.
func TestCacheInterceptorLRUEviction(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	// Fill 3
	for i := 1; i <= 3; i++ {
		m := &mockContext{method: "GET"}
		w := &cacheTestContext{mockContext: m, originalURL: fmt.Sprintf("/item%d", i)}
		_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(w, handler)
	}

	// Access /item1
	m1 := &mockContext{method: "GET"}
	w1 := &cacheTestContext{mockContext: m1, originalURL: "/item1"}
	_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(w1, handler)

	// Add 4th -> evict /item2 (LRU)
	m4 := &mockContext{method: "GET"}
	w4 := &cacheTestContext{mockContext: m4, originalURL: "/item4"}
	_ = store.newInterceptor(5*time.Minute, 3, "lru").Intercept(w4, handler)

	assert.Equal(t, 3, store.Size())
	_, hasItem1 := store.entries["GET /item1"]
	assert.True(t, hasItem1)
	_, hasItem2 := store.entries["GET /item2"]
	assert.False(t, hasItem2)
}

// TestCacheInterceptorFIFOEviction tests FIFO eviction strategy.
func TestCacheInterceptorFIFOEviction(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	for i := 1; i <= 3; i++ {
		m := &mockContext{method: "GET"}
		w := &cacheTestContext{mockContext: m, originalURL: fmt.Sprintf("/item%d", i)}
		_ = store.newInterceptor(5*time.Minute, 3, "fifo").Intercept(w, handler)
		time.Sleep(10 * time.Millisecond)
	}

	m4 := &mockContext{method: "GET"}
	w4 := &cacheTestContext{mockContext: m4, originalURL: "/item4"}
	_ = store.newInterceptor(5*time.Minute, 3, "fifo").Intercept(w4, handler)

	assert.Equal(t, 3, store.Size())
	_, hasItem1 := store.entries["GET /item1"]
	assert.False(t, hasItem1)
}

// TestCacheInterceptorProactiveSweep tests AC 2: Proactive expiration sweep.
func TestCacheInterceptorProactiveSweep(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	ttl := 100 * time.Millisecond
	for i := 0; i < 5; i++ {
		m := &mockContext{method: "GET"}
		w := &cacheTestContext{mockContext: m, originalURL: fmt.Sprintf("/item%d", i)}
		_ = store.newInterceptor(ttl, 100, "lru").Intercept(w, handler)
	}

	assert.Equal(t, 5, store.Size())
	time.Sleep(150 * time.Millisecond)
	store.sweep()
	assert.Less(t, store.Size(), 5)
}

// TestCacheInterceptorConfigParsing tests AC 3: Config parsing.
func TestCacheInterceptorConfigParsing(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectedErr bool
	}{
		{"Basic duration", "5m", false},
		{"Duration with max", "5m:max=500", false},
		{"Duration with max and LRU", "5m:max=500:lru", false},
		{"Duration with max and FIFO", "5m:max=200:fifo", false},
		{"Invalid duration", "invalid", true},
		{"Invalid max", "5m:max=abc", true},
		{"Invalid strategy", "5m:max=500:invalid", true},
		{"Zero TTL", "0s", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newCacheStore()
			defer store.Stop()
			factory := cacheInterceptorFactory(store)
			_, err := factory(tt.config)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCacheInterceptorExpirationLazyDeletion tests lazy deletion.
func TestCacheInterceptorExpirationLazyDeletion(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()

	ttl := 50 * time.Millisecond
	interceptor := store.newInterceptor(ttl, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	m1 := &mockContext{method: "GET"}
	w1 := &cacheTestContext{mockContext: m1, originalURL: "/test"}
	_ = interceptor.Intercept(w1, handler)
	assert.Equal(t, 1, store.Size())

	time.Sleep(100 * time.Millisecond)

	calls := atomic.Int32{}
	m2 := &mockContext{method: "GET"}
	w2 := &cacheTestContext{mockContext: m2, originalURL: "/test"}
	_ = interceptor.Intercept(w2, func(ctx Context) error {
		calls.Add(1)
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "new"})
	})

	assert.Equal(t, int32(1), calls.Load())
}

// TestCacheInterceptorNonGetMethodsNotCached tests non-GET methods.
func TestCacheInterceptorNonGetMethodsNotCached(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()
	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusCreated)
		return ctx.JSON(map[string]string{"id": "123"})
	}

	m1 := &mockContext{method: "POST"}
	w1 := &cacheTestContext{mockContext: m1, originalURL: "/api"}
	_ = interceptor.Intercept(w1, handler)

	assert.Equal(t, 0, store.Size())
}

// TestCacheInterceptorSuccessRangeOnlyCached tests caching logic for status codes.
func TestCacheInterceptorSuccessRangeOnlyCached(t *testing.T) {
	testCases := []struct {
		statusCode  int
		shouldCache bool
	}{
		{http.StatusOK, true},
		{http.StatusBadRequest, false},
		{http.StatusInternalServerError, false},
	}

	for _, tc := range testCases {
		store := newCacheStore()
		defer store.Stop()
		interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

		handler := func(ctx Context) error {
			ctx.Status(tc.statusCode)
			return ctx.JSON(map[string]string{"d": "v"})
		}

		m := &mockContext{method: "GET"}
		w := &cacheTestContext{mockContext: m, originalURL: "/t"}
		_ = interceptor.Intercept(w, handler)

		assert.Equal(t, tc.shouldCache, store.Size() > 0)
	}
}

// TestCacheInterceptorConcurrentAccessNoRace tests thread-safety.
func TestCacheInterceptorConcurrentAccessNoRace(t *testing.T) {
	store := newCacheStore()
	defer store.Stop()
	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"d": "v"})
	}

	wg := sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				m := &mockContext{method: "GET"}
				w := &cacheTestContext{mockContext: m, originalURL: fmt.Sprintf("/item%d", n%5)}
				_ = interceptor.Intercept(w, handler)
			}
		}(i)
	}
	wg.Wait()
	assert.Greater(t, store.Size(), 0)
}

// TestCacheStoreStop tests graceful shutdown.
func TestCacheStoreStop(t *testing.T) {
	store := newCacheStore()
	assert.NotNil(t, store.sweepTicker)
	err := store.Stop()
	assert.NoError(t, err)
}

// TestCacheInterceptorResponseRecorderCapture tests responseRecorder.
func TestCacheInterceptorResponseRecorderCapture(t *testing.T) {
	mockCtx := &mockContext{method: "GET"}
	recorder := &responseRecorder{BaseContext: mockCtx}
	recorder.Status(http.StatusOK)
	_ = recorder.JSON(map[string]string{"k": "v"})
	assert.Equal(t, http.StatusOK, recorder.status)
	assert.NotEmpty(t, recorder.body)
}
