package web

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func BenchmarkBindingJSON(b *testing.B) {
	type request struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18"`
	}

	plan, err := newBindingPlan(reflect.TypeOf(request{}))
	if err != nil {
		b.Fatalf("newBindingPlan() error = %v", err)
	}
	ctx := &testContext{body: []byte(`{"name":"Ada","email":"ada@example.test","age":36}`)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := plan.bind(ctx); err != nil {
			b.Fatalf("bind() error = %v", err)
		}
	}
}

func BenchmarkCacheInterceptorHit(b *testing.B) {
	store := newCacheStore()
	b.Cleanup(func() { _ = store.Stop() })
	interceptor := store.newInterceptor(time.Minute, 128, "lru")
	handler := func(ctx Context) error {
		ctx.Status(http.StatusOK)
		return ctx.JSON(map[string]string{"data": "value"})
	}
	ctx := &cacheTestContext{
		mockContext: &mockContext{method: http.MethodGet},
		originalURL: "/bench/cache",
	}
	if err := interceptor.Intercept(ctx, handler); err != nil {
		b.Fatalf("warm Intercept() error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := &cacheTestContext{
			mockContext: &mockContext{method: http.MethodGet},
			originalURL: "/bench/cache",
		}
		if err := interceptor.Intercept(ctx, handler); err != nil {
			b.Fatalf("Intercept() error = %v", err)
		}
	}
}
