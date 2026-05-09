package web

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	fiberinternal "github.com/enokdev/helix/web/internal"
)

type failingOnceAdapter struct {
	fail bool
}

func (a *failingOnceAdapter) Start(string) error {
	return nil
}

func (a *failingOnceAdapter) Stop(context.Context) error {
	return nil
}

func (a *failingOnceAdapter) RegisterRoute(string, string, fiberinternal.HandlerFunc) error {
	if a.fail {
		a.fail = false
		return errors.New("adapter failed")
	}
	return nil
}

func (a *failingOnceAdapter) ServeHTTP(*http.Request) (*http.Response, error) {
	return nil, nil
}

func newInternalTestServer(adapter fiberinternal.Adapter) *server {
	return &server{
		adapter:              adapter,
		errorHandlers:        make(map[string]errorHandlerInvoker),
		guards:               make(map[string]Guard),
		guardFactories:       make(map[string]GuardFactory),
		interceptors:         make(map[string]Interceptor),
		interceptorFactories: make(map[string]InterceptorFactory),
		registeredRoutes:     make(map[string]struct{}),
		cache:                newCacheStore(),
	}
}

func TestRegisterRouteRollsBackDuplicateStateWhenAdapterFails(t *testing.T) {
	t.Parallel()

	srv := newInternalTestServer(&failingOnceAdapter{fail: true})
	t.Cleanup(func() { _ = srv.cache.Stop() })
	handler := func(Context) error { return nil }

	if err := srv.RegisterRoute(http.MethodGet, "/retry", handler); err == nil {
		t.Fatal("first RegisterRoute() error = nil, want adapter failure")
	}
	if err := srv.RegisterRoute(http.MethodGet, "/retry", handler); err != nil {
		t.Fatalf("second RegisterRoute() error = %v, want nil", err)
	}
}

func TestResolveGuardFactoryCanRegisterWithoutDeadlock(t *testing.T) {
	t.Parallel()

	srv := newInternalTestServer(&failingOnceAdapter{})
	t.Cleanup(func() { _ = srv.cache.Stop() })
	if err := srv.registerGuardFactory("dynamic", func(string) (Guard, error) {
		if err := RegisterGuard(srv, "registered-from-factory", GuardFunc(func(Context) error { return nil })); err != nil {
			return nil, err
		}
		return GuardFunc(func(Context) error { return nil }), nil
	}); err != nil {
		t.Fatalf("registerGuardFactory() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := srv.resolveGuard(namedDirective{name: "dynamic", argument: "value", raw: "dynamic:value"})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("resolveGuard() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolveGuard() deadlocked")
	}
}

func TestResolveInterceptorFactoryCanRegisterWithoutDeadlock(t *testing.T) {
	t.Parallel()

	srv := newInternalTestServer(&failingOnceAdapter{})
	t.Cleanup(func() { _ = srv.cache.Stop() })
	if err := srv.registerInterceptorFactory("dynamic", func(string) (Interceptor, error) {
		if err := RegisterInterceptor(srv, "registered-from-factory", InterceptorFunc(func(ctx Context, next HandlerFunc) error {
			return next(ctx)
		})); err != nil {
			return nil, err
		}
		return InterceptorFunc(func(ctx Context, next HandlerFunc) error { return next(ctx) }), nil
	}); err != nil {
		t.Fatalf("registerInterceptorFactory() error = %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := srv.resolveInterceptor(namedDirective{name: "dynamic", argument: "value", raw: "dynamic:value"})
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("resolveInterceptor() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("resolveInterceptor() deadlocked")
	}
}
