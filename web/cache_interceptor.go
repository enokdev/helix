package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type cacheEntry struct {
	status    int
	body      []byte // JSON-encoded response body; stored as bytes to prevent mutation by the caller.
	expiresAt time.Time
}

type cacheStore struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newCacheStore() *cacheStore {
	return &cacheStore{entries: make(map[string]cacheEntry)}
}

func (s *cacheStore) newInterceptor(ttl time.Duration) Interceptor {
	return InterceptorFunc(func(ctx Context, next HandlerFunc) error {
		if ctx.Method() != http.MethodGet {
			return next(ctx)
		}

		key := ctx.Method() + " " + ctx.OriginalURL()
		now := time.Now()
		if entry, ok := s.get(key, now); ok {
			ctx.Status(entry.status)
			var body any
			if err := json.Unmarshal(entry.body, &body); err != nil {
				return fmt.Errorf("web: cache interceptor unmarshal: %w", err)
			}
			return ctx.JSON(body)
		}

		recorder := &responseRecorder{Context: ctx}
		if err := next(recorder); err != nil {
			return err
		}
		if recorder.wroteJSON && recorder.status >= http.StatusOK && recorder.status < http.StatusMultipleChoices {
			s.set(key, cacheEntry{
				status:    recorder.status,
				body:      recorder.body,
				expiresAt: now.Add(ttl),
			})
		}
		return nil
	})
}

func (s *cacheStore) get(key string, now time.Time) (cacheEntry, bool) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return cacheEntry{}, false
	}
	if now.After(entry.expiresAt) {
		s.mu.Lock()
		// Re-read under write lock: a concurrent goroutine may have refreshed this entry.
		if current, ok := s.entries[key]; ok && now.After(current.expiresAt) {
			delete(s.entries, key)
		}
		s.mu.Unlock()
		return cacheEntry{}, false
	}
	return entry, true
}

func (s *cacheStore) set(key string, entry cacheEntry) {
	s.mu.Lock()
	s.entries[key] = entry
	s.mu.Unlock()
}

// cacheInterceptorFactory returns a factory for the built-in "cache" interceptor.
// The cache is shared across all requests: responses are keyed only by HTTP method
// and URL, not by authenticated user or session. Register this factory under a
// per-user key scheme or disable it for endpoints that return user-scoped data.
func cacheInterceptorFactory(store *cacheStore) InterceptorFactory {
	return func(argument string) (Interceptor, error) {
		ttl, err := time.ParseDuration(argument)
		if err != nil || ttl <= 0 {
			return nil, fmt.Errorf("web: parse cache duration %q: %w", argument, ErrInvalidDirective)
		}
		return store.newInterceptor(ttl), nil
	}
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
