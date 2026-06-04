package web

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
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
	locals      map[string]any
}

func (c *cacheTestContext) Path() string           { return "/" }
func (c *cacheTestContext) Param(_ string) string  { return "" }
func (c *cacheTestContext) Query(_ string) string  { return "" }
func (c *cacheTestContext) Header(_ string) string { return "" }
func (c *cacheTestContext) IP() string             { return "" }
func (c *cacheTestContext) Body() []byte           { return nil }
func (c *cacheTestContext) OriginalURL() string    { return c.originalURL }
func (c *cacheTestContext) Status(code int) {
	c.statusCode = code
	c.mockContext.Status(code)
}

func (c *cacheTestContext) Locals(key string, value ...any) any {
	if c.locals == nil {
		c.locals = make(map[string]any)
	}
	if len(value) > 0 {
		c.locals[key] = value[0]
		return value[0]
	}
	return c.locals[key]
}
func (c *cacheTestContext) SetHeader(string, string)    {}
func (c *cacheTestContext) AppendHeader(string, string) {}
func (c *cacheTestContext) Send(_ []byte) error         { return nil }
func (c *cacheTestContext) Context() context.Context    { return context.Background() }

// TestCacheInterceptorSingleFlightPatternColdCache tests AC 1: Multiple concurrent requests on cold cache.
func TestCacheInterceptorSingleFlightPatternColdCache(t *testing.T) {
	store := newCacheStore()
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	var startOnce sync.Once
	handler := func(ctx Context) error {
		startOnce.Do(func() { close(handlerStarted) })
		handlerCalls.Add(1)
		<-handlerDone
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "test"})
	}

	results := make(chan struct {
		err    error
		status int
	}, 50)
	ready := make(chan struct{}, 10)
	start := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			ready <- struct{}{}
			<-start
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

	for i := 0; i < 10; i++ {
		<-ready
	}
	close(start)

	// Wait for the first handler call, then give the other goroutines
	// time to pile into the singleflight group before releasing.
	<-handlerStarted
	for {
		store.flightMu.RLock()
		waitersReady := len(store.inflight) == 1
		store.flightMu.RUnlock()
		if waitersReady {
			break
		}
		runtime.Gosched()
	}
	close(handlerDone)

	for i := 0; i < 10; i++ {
		result := <-results
		assert.NoError(t, result.err)
		assert.Equal(t, http.StatusOK, result.status)
	}

	assert.Equal(t, int32(1), handlerCalls.Load())
}

func TestCacheInterceptorEvaluatesGuardsForInflightWaiters(t *testing.T) {
	store := newCacheStore()
	t.Cleanup(func() { _ = store.Stop() })

	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	handler := func(ctx Context) error {
		close(handlerStarted)
		<-handlerDone
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "authorized"})
	}

	first := &cacheTestContext{
		mockContext: &mockContext{method: http.MethodGet},
		originalURL: "/api/guarded",
	}
	first.Locals(routeGuardEvaluatorLocal, func(Context) error { return nil })

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- interceptor.Intercept(first, handler)
	}()

	<-handlerStarted
	second := &cacheTestContext{
		mockContext: &mockContext{method: http.MethodGet},
		originalURL: "/api/guarded",
	}
	second.Locals(routeGuardEvaluatorLocal, func(Context) error {
		return Unauthorized("authentication required")
	})

	secondResult := make(chan error, 1)
	go func() {
		secondResult <- interceptor.Intercept(second, handler)
	}()

	time.Sleep(50 * time.Millisecond)
	close(handlerDone)
	if err := <-firstResult; err != nil {
		t.Fatalf("first request error = %v", err)
	}
	err := <-secondResult
	structured, ok := err.(interface{ StatusCode() int })
	if !ok {
		t.Fatal("second request error = nil, want guard rejection")
	}
	if structured.StatusCode() != http.StatusUnauthorized {
		t.Fatalf("second request status error = %d, want %d", structured.StatusCode(), http.StatusUnauthorized)
	}
	if second.statusCode == http.StatusOK {
		t.Fatalf("second request status = %d, in-flight response was written despite guard rejection", second.statusCode)
	}
}

// TestCacheInterceptorHitAndMiss tests basic cache hit/miss metrics.
func TestCacheInterceptorHitAndMiss(t *testing.T) {
	store := newCacheStore()
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })

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
			t.Cleanup(func() { _ = store.Stop() })
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
	t.Cleanup(func() { _ = store.Stop() })

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
	t.Cleanup(func() { _ = store.Stop() })
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
		t.Cleanup(func() { _ = store.Stop() })
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
	t.Cleanup(func() { _ = store.Stop() })
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

func TestCacheStoreStopIsIdempotent(t *testing.T) {
	store := newCacheStore()
	if err := store.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := store.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func TestCacheStoreStopConcurrentIsIdempotent(t *testing.T) {
	store := newCacheStore()

	const goroutines = 20
	errs := make(chan error, goroutines)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			errs <- store.Stop()
		}()
	}

	go func() {
		wg.Wait()
		close(errs)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent Stop() calls timed out")
	}

	for err := range errs {
		if err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
	}

	select {
	case <-store.sweepDone:
	default:
		t.Fatal("sweep goroutine did not exit")
	}
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

// TestCacheInterceptorStatusAfterJSON verifies that calling Status(non-2xx) AFTER
// JSON() prevents caching, even though JSON() has already been called.
// This documents the intentional ordering contract of responseRecorder.
func TestCacheInterceptorStatusAfterJSON(t *testing.T) {
	store := newCacheStore()
	t.Cleanup(func() { _ = store.Stop() })
	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	// Handler calls JSON() first then Status(404) — should NOT be cached.
	handler := func(ctx Context) error {
		if err := ctx.JSON(map[string]string{"msg": "not found"}); err != nil {
			return err
		}
		ctx.Status(http.StatusNotFound)
		return nil
	}

	m := &mockContext{method: "GET"}
	w := &cacheTestContext{mockContext: m, originalURL: "/missing"}
	_ = interceptor.Intercept(w, handler)

	assert.Equal(t, 0, store.Size(), "response with Status(404) after JSON() must not be cached")
}

func TestCacheInterceptorInflightWaitersReplayNonCacheableJSON(t *testing.T) {
	store := newCacheStore()
	t.Cleanup(func() { _ = store.Stop() })
	interceptor := store.newInterceptor(5*time.Minute, 100, "lru")

	handlerCalls := atomic.Int32{}
	handlerStarted := make(chan struct{})
	handlerDone := make(chan struct{})
	handler := func(ctx Context) error {
		if handlerCalls.Add(1) == 1 {
			close(handlerStarted)
		}
		<-handlerDone
		if err := ctx.JSON(map[string]string{"msg": "not found"}); err != nil {
			return err
		}
		ctx.Status(http.StatusNotFound)
		return nil
	}

	first := &cacheTestContext{
		mockContext: &mockContext{method: http.MethodGet},
		originalURL: "/missing-concurrent",
	}
	firstResult := make(chan error, 1)
	go func() {
		firstResult <- interceptor.Intercept(first, handler)
	}()
	<-handlerStarted

	second := &cacheTestContext{
		mockContext: &mockContext{method: http.MethodGet},
		originalURL: "/missing-concurrent",
	}
	secondResult := make(chan error, 1)
	go func() {
		secondResult <- interceptor.Intercept(second, handler)
	}()

	time.Sleep(50 * time.Millisecond)
	close(handlerDone)

	assert.NoError(t, <-firstResult)
	assert.NoError(t, <-secondResult)
	assert.Equal(t, int32(1), handlerCalls.Load())
	assert.Equal(t, http.StatusNotFound, first.statusCode)
	assert.Equal(t, http.StatusNotFound, second.statusCode)
	assert.Equal(t, 0, store.Size())
}
