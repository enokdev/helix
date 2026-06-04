# Lacunes P1 Parallel Batch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close or advance three independent P1 robustness tasks from `lacunes.md`: OTLP TLS options, partial starter configuration failures, and cache interceptor limits.

**Architecture:** Keep each track scoped to its owning package. Observability gets backward-compatible tracing transport config. Starter rollback uses a small optional resolver unregister capability so starters can undo earlier registrations when a later registration fails. Cache work starts as proof-driven audit because the implementation already contains single-flight, max-size eviction, and proactive sweeping.

**Tech Stack:** Go 1.25.5 via `/Users/yacoubakone/.govm/go/bin/go`, OpenTelemetry OTLP HTTP exporter, Helix `core`, `starter`, `observability`, `web`, Markdown docs.

---

## File Structure

- Modify: `observability/tracing.go`
  - Add OTLP transport fields to `TracingConfig`.
  - Resolve loader keys for `insecure`, `headers`, `tls.ca-file`, `tls.cert-file`, `tls.key-file`, and `tls.server-name`.
  - Build OTLP HTTP exporter options without forcing insecure mode when TLS is configured.
- Modify: `observability/tracing_test.go`
  - Add table tests for config resolution and exporter construction with secure/insecure modes.
- Modify: `docs/reference/configuration-keys.md`
  - Document the new tracing keys and production OTLP example.
- Modify: `docs/fr/reference/configuration-keys.md`
  - French equivalent of tracing config docs.
- Modify: `docs/reference/starters.md`
  - Mention OTLP insecure/headers/TLS keys if this page already lists tracing keys.
- Modify: `docs/fr/reference/starters.md`
  - French equivalent.
- Modify: `core/resolver.go`
  - Add optional `UnregisterResolver` interface, without changing the required `Resolver` interface.
- Modify: `core/container.go`
  - Add `Unregister(component any) error` on `Container`.
- Modify: `core/reflect_resolver.go`
  - Implement `Unregister(component any) error`.
- Modify: `core/wire_resolver.go`
  - Implement `Unregister(component any) error`.
- Modify: `core/container_test.go`, `core/reflect_resolver_test.go`, `core/wire_resolver_test.go`
  - Add unregister behavior tests.
- Modify: `starter/web/starter.go`
  - Roll back server registration if lifecycle registration fails.
- Modify: `starter/scheduling/starter.go`
  - Roll back scheduler registration if registrar registration fails.
- Modify: `starter/data/starter.go`
  - Roll back DB component registrations if a later DB component or lifecycle registration fails.
- Modify: `starter/web/starter_test.go`, `starter/scheduling/starter_test.go`, `starter/data/starter_test.go`
  - Add targeted failure tests for starters that perform multi-step registrations.
- Modify: `web/cache_interceptor.go`
  - Make cache store shutdown idempotent.
- Modify: `web/cache_interceptor_test.go`
  - Add stop idempotency proof for the sweep goroutine.
- Modify: `docs/guide/web.md`, `docs/fr/guide/web.md`
  - Document cache directive options and production limits.
- Modify: `lacunes.md`
  - Mark only completed tasks with concrete `Preuve:` bullets after validation.

## Task 1: Add Core Unregister Capability

**Files:**
- Modify: `core/resolver.go`
- Modify: `core/container.go`
- Modify: `core/reflect_resolver.go`
- Modify: `core/wire_resolver.go`
- Modify: `core/container_test.go`
- Modify: `core/reflect_resolver_test.go`
- Modify: `core/wire_resolver_test.go`

- [ ] **Step 1: Add failing container unregister tests**

Append this test to `core/container_test.go`:

```go
func TestContainerUnregister(t *testing.T) {
	container := NewContainer(WithResolver(NewReflectResolver()))
	component := &testDependency{Name: "registered"}

	if err := container.Register(component); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	var resolved *testDependency
	if err := container.Resolve(&resolved); err != nil {
		t.Fatalf("Resolve() before unregister error = %v", err)
	}

	if err := container.Unregister(component); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	resolved = nil
	if err := container.Resolve(&resolved); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() after unregister error = %v, want ErrNotFound", err)
	}
}

func TestContainerUnregisterWithoutResolverSupport(t *testing.T) {
	container := NewContainer(WithResolver(&registerOnlyResolver{}))

	err := container.Unregister(&testDependency{})
	if !errors.Is(err, ErrUnresolvable) {
		t.Fatalf("Unregister() error = %v, want ErrUnresolvable", err)
	}
}

type registerOnlyResolver struct{}

func (r *registerOnlyResolver) Register(any) error     { return nil }
func (r *registerOnlyResolver) Resolve(any) error      { return ErrNotFound }
func (r *registerOnlyResolver) Graph() DependencyGraph { return DependencyGraph{} }
```

Ensure `core/container_test.go` imports `errors`. If it already has an import block, add `"errors"` to that block.

- [ ] **Step 2: Run the failing container test**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./core -run 'TestContainerUnregister' -count=1
```

Expected: FAIL because `Container.Unregister` is not defined.

- [ ] **Step 3: Add optional unregister interface and container method**

In `core/resolver.go`, after `Resolver`, add:

```go
// UnregisterResolver is an optional resolver capability used to roll back
// framework-owned registrations when a multi-step configuration fails.
type UnregisterResolver interface {
	Unregister(component any) error
}
```

In `core/container.go`, after `Register`, add:

```go
// Unregister removes a component from the container's resolver registry when
// the active resolver supports rollback.
func (c *Container) Unregister(component any) error {
	c.resolverMu.Lock()
	defer c.resolverMu.Unlock()

	if component == nil {
		return fmt.Errorf("core: unregister: %w", ErrUnresolvable)
	}
	if c.resolver == nil {
		return fmt.Errorf("core: unregister %T: %w", component, ErrUnresolvable)
	}
	unregisterer, ok := c.resolver.(UnregisterResolver)
	if !ok {
		return fmt.Errorf("core: unregister %T: resolver does not support unregister: %w", component, ErrUnresolvable)
	}
	return unregisterer.Unregister(component)
}
```

- [ ] **Step 4: Implement reflect resolver unregister**

Add this method to `core/reflect_resolver.go` after `Register`:

```go
// Unregister removes a component registration keyed by its concrete type.
func (r *ReflectResolver) Unregister(component any) error {
	componentValue := reflect.ValueOf(component)
	if !isRegistrableComponent(componentValue) {
		return fmt.Errorf("core: unregister %T: %w", component, ErrUnresolvable)
	}
	componentType := componentValue.Type()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.registrations[componentType]; !exists {
		return fmt.Errorf("core: unregister %T: %w", component, ErrNotFound)
	}

	delete(r.registrations, componentType)
	delete(r.singletons, componentType)
	r.invalidateSingletonsDependingOn(componentType)
	r.registrationOrder = removeRegistrationType(r.registrationOrder, componentType)
	delete(r.graph.Edges, componentType.String())
	r.graph.Nodes = removeGraphNode(r.graph.Nodes, componentType.String())
	for node, edges := range r.graph.Edges {
		r.graph.Edges[node] = removeGraphNode(edges, componentType.String())
	}

	return nil
}

func removeRegistrationType(types []reflect.Type, target reflect.Type) []reflect.Type {
	for i, typ := range types {
		if typ == target {
			return append(types[:i], types[i+1:]...)
		}
	}
	return types
}

func removeGraphNode(nodes []string, target string) []string {
	for i, node := range nodes {
		if node == target {
			return append(nodes[:i], nodes[i+1:]...)
		}
	}
	return nodes
}
```

- [ ] **Step 5: Implement wire resolver unregister**

Add this method to `core/wire_resolver.go` after `Register`:

```go
// Unregister removes a pre-wired component instance by concrete type.
func (r *WireResolver) Unregister(component any) error {
	componentType := reflect.TypeOf(component)
	componentValue := reflect.ValueOf(component)
	if !isRegistrableComponent(componentValue) {
		return fmt.Errorf("core: unregister %T: %w", component, ErrUnresolvable)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[componentType]; !exists {
		return fmt.Errorf("core: unregister %T: %w", component, ErrNotFound)
	}
	delete(r.instances, componentType)
	r.registrationOrder = removeWireRegistrationType(r.registrationOrder, componentType)
	return nil
}

func removeWireRegistrationType(types []reflect.Type, target reflect.Type) []reflect.Type {
	for i, typ := range types {
		if typ == target {
			return append(types[:i], types[i+1:]...)
		}
	}
	return types
}
```

- [ ] **Step 6: Add resolver-specific tests**

Append to `core/reflect_resolver_test.go`:

```go
func TestReflectResolverUnregisterRemovesGraphNode(t *testing.T) {
	resolver := NewReflectResolver()
	component := &reflectResolverDependency{Value: "registered"}

	if err := resolver.Register(component); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := resolver.Unregister(component); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	graph := resolver.Graph()
	for _, node := range graph.Nodes {
		if node == "*core.reflectResolverDependency" {
			t.Fatalf("Graph node %q still present after unregister", node)
		}
	}
}
```

Append to `core/wire_resolver_test.go`:

```go
func TestWireResolverUnregister(t *testing.T) {
	resolver := NewWireResolver()
	component := &wireResolverService{}

	if err := resolver.Register(component); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := resolver.Unregister(component); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	var resolved *wireResolverService
	if err := resolver.Resolve(&resolved); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Resolve() error = %v, want ErrNotFound", err)
	}
}
```

Ensure `core/wire_resolver_test.go` imports `errors`.

- [ ] **Step 7: Run core tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./core -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit core unregister capability**

Run:

```bash
git add core/resolver.go core/container.go core/reflect_resolver.go core/wire_resolver.go core/container_test.go core/reflect_resolver_test.go core/wire_resolver_test.go
git commit -m "feat(core): support registration rollback"
```

Expected: commit succeeds.

## Task 2: Roll Back Partial Starter Configuration

**Files:**
- Modify: `starter/web/starter.go`
- Modify: `starter/scheduling/starter.go`
- Modify: `starter/data/starter.go`
- Modify: `starter/web/starter_test.go`
- Modify: `starter/scheduling/starter_test.go`
- Modify: `starter/data/starter_test.go`

- [ ] **Step 1: Add failing starter rollback tests**

In each starter test package, add a local resolver that fails on a chosen registration count:

```go
type failOnRegisterResolver struct {
	inner  *core.ReflectResolver
	count  int
	failAt int
	err    error
}

func newFailOnRegisterResolver(failAt int, err error) *failOnRegisterResolver {
	return &failOnRegisterResolver{inner: core.NewReflectResolver(), failAt: failAt, err: err}
}

func (r *failOnRegisterResolver) Register(component any) error {
	r.count++
	if r.count == r.failAt {
		return r.err
	}
	return r.inner.Register(component)
}

func (r *failOnRegisterResolver) Unregister(component any) error {
	return r.inner.Unregister(component)
}

func (r *failOnRegisterResolver) Resolve(target any) error {
	return r.inner.Resolve(target)
}

func (r *failOnRegisterResolver) Graph() core.DependencyGraph {
	return r.inner.Graph()
}
```

Add this test to `starter/web/starter_test.go`:

```go
func TestStarterConfigureRollsBackServerWhenLifecycleRegisterFails(t *testing.T) {
	sentinel := errors.New("register lifecycle failed")
	resolver := newFailOnRegisterResolver(2, sentinel)
	container := core.NewContainer(core.WithResolver(resolver))

	err := New(nil).Configure(container)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Configure() error = %v, want sentinel", err)
	}

	var server helixweb.HTTPServer
	if resolveErr := container.Resolve(&server); !errors.Is(resolveErr, core.ErrNotFound) {
		t.Fatalf("Resolve HTTPServer after rollback error = %v, want ErrNotFound", resolveErr)
	}
}
```

Add this test to `starter/scheduling/starter_test.go`:

```go
func TestConfigureRollsBackSchedulerWhenRegistrarRegisterFails(t *testing.T) {
	sentinel := errors.New("register registrar failed")
	resolver := newFailOnRegisterResolver(2, sentinel)
	container := core.NewContainer(core.WithResolver(resolver))

	err := New(nil).Configure(container)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Configure() error = %v, want sentinel", err)
	}

	var sched scheduler.Scheduler
	if resolveErr := container.Resolve(&sched); !errors.Is(resolveErr, core.ErrNotFound) {
		t.Fatalf("Resolve Scheduler after rollback error = %v, want ErrNotFound", resolveErr)
	}
}
```

Add this test to `starter/data/starter_test.go`:

```go
func TestConfigureRollsBackDBComponentsWhenLifecycleRegisterFails(t *testing.T) {
	sentinel := errors.New("register lifecycle failed")
	resolver := newFailOnRegisterResolver(3, sentinel)
	container := core.NewContainer(core.WithResolver(resolver))
	cfg := fakeConfig{values: map[string]any{
		databaseURLKey: "file::memory:?cache=shared",
	}}

	err := New(cfg).Configure(container)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Configure() error = %v, want sentinel", err)
	}

	var db *datagorm.DB
	if resolveErr := container.Resolve(&db); !errors.Is(resolveErr, core.ErrNotFound) {
		t.Fatalf("Resolve DB after rollback error = %v, want ErrNotFound", resolveErr)
	}
}
```

Adjust imports in each file for `errors`, `core`, and package aliases already used by that test file.

- [ ] **Step 2: Run failing starter tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./starter/web ./starter/scheduling ./starter/data -run 'Rollback|RollsBack' -count=1
```

Expected: FAIL because starters do not yet unregister earlier registrations.

- [ ] **Step 3: Add rollback helper locally to starters**

In each starter file that needs rollback, add this helper near the bottom:

```go
func rollbackRegistration(container *core.Container, component any) {
	if container == nil || component == nil {
		return
	}
	_ = container.Unregister(component)
}
```

If multiple starter packages use the same helper name, keep it package-local in each package. Do not create a shared internal package for one small helper.

- [ ] **Step 4: Update web starter rollback**

In `starter/web/starter.go`, replace the lifecycle registration block:

```go
if err := container.Register(lifecycle.server); err != nil {
	return fmt.Errorf("web starter: register server: %w", err)
}
if err := container.Register(lifecycle); err != nil {
	return fmt.Errorf("web starter: register lifecycle: %w", err)
}
```

with:

```go
if err := container.Register(lifecycle.server); err != nil {
	return fmt.Errorf("web starter: register server: %w", err)
}
if err := container.Register(lifecycle); err != nil {
	rollbackRegistration(container, lifecycle.server)
	return fmt.Errorf("web starter: register lifecycle: %w", err)
}
```

- [ ] **Step 5: Update scheduling starter rollback**

In `starter/scheduling/starter.go`, replace:

```go
sched := scheduler.NewScheduler()
if err := container.Register(sched); err != nil {
	return fmt.Errorf("scheduling starter: register scheduler: %w", err)
}
if err := container.Register(newScheduledJobRegistrar(container, sched)); err != nil {
	return fmt.Errorf("scheduling starter: register scheduled job registrar: %w", err)
}
```

with:

```go
sched := scheduler.NewScheduler()
if err := container.Register(sched); err != nil {
	return fmt.Errorf("scheduling starter: register scheduler: %w", err)
}
registrar := newScheduledJobRegistrar(container, sched)
if err := container.Register(registrar); err != nil {
	rollbackRegistration(container, sched)
	return fmt.Errorf("scheduling starter: register scheduled job registrar: %w", err)
}
```

- [ ] **Step 6: Update data starter rollback**

In `starter/data/starter.go`, inside `Configure`, track registered DB components:

```go
registeredComponents := make([]any, 0)
```

Place it immediately after `lc := &databaseLifecycle{}`.

Inside the `for _, comp := range db.Components()` loop, after successful registration append:

```go
registeredComponents = append(registeredComponents, comp)
```

Replace the lifecycle registration failure block:

```go
if err := container.Register(lc); err != nil {
	if lc.db != nil {
		_ = lc.db.Close()
	}
	return fmt.Errorf("data starter: register lifecycle: %w", err)
}
```

with:

```go
if err := container.Register(lc); err != nil {
	for i := len(registeredComponents) - 1; i >= 0; i-- {
		rollbackRegistration(container, registeredComponents[i])
	}
	if lc.db != nil {
		_ = lc.db.Close()
	}
	return fmt.Errorf("data starter: register lifecycle: %w", err)
}
```

Also in the DB component registration failure branch, before closing the DB, add:

```go
for i := len(registeredComponents) - 1; i >= 0; i-- {
	rollbackRegistration(container, registeredComponents[i])
}
```

- [ ] **Step 7: Run starter tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./starter/... -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit starter rollback**

Run:

```bash
git add starter/web/starter.go starter/scheduling/starter.go starter/data/starter.go starter/web/starter_test.go starter/scheduling/starter_test.go starter/data/starter_test.go
git commit -m "fix(starter): roll back partial registrations"
```

Expected: commit succeeds.

## Task 3: Add OTLP TLS and Header Configuration

**Files:**
- Modify: `observability/tracing.go`
- Modify: `observability/tracing_test.go`
- Modify: `docs/reference/configuration-keys.md`
- Modify: `docs/fr/reference/configuration-keys.md`
- Modify: `docs/reference/starters.md`
- Modify: `docs/fr/reference/starters.md`

- [ ] **Step 1: Add failing config resolution tests**

Append to `observability/tracing_test.go`:

```go
func TestResolveTracingConfig_OTLPTransportOptions(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":         true,
		"helix.starters.observability.tracing.exporter":        "otlp",
		"helix.starters.observability.tracing.endpoint":        "https://otel.example.com:4318",
		"helix.starters.observability.tracing.insecure":        false,
		"helix.starters.observability.tracing.headers":         map[string]any{"Authorization": "Bearer token", "x-tenant": "helix"},
		"helix.starters.observability.tracing.tls.server-name": "otel.example.com",
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}

	if !cfg.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if cfg.Exporter != "otlp" {
		t.Fatalf("Exporter = %q, want otlp", cfg.Exporter)
	}
	if cfg.Insecure {
		t.Fatal("Insecure = true, want false")
	}
	if cfg.Headers["Authorization"] != "Bearer token" || cfg.Headers["x-tenant"] != "helix" {
		t.Fatalf("Headers = %#v, want Authorization and x-tenant", cfg.Headers)
	}
	if cfg.TLS.ServerName != "otel.example.com" {
		t.Fatalf("TLS.ServerName = %q, want otel.example.com", cfg.TLS.ServerName)
	}
}

func TestResolveTracingConfig_DefaultOTLPInsecureRemainsCompatible(t *testing.T) {
	loader := mapLoader{
		"helix.starters.observability.tracing.enabled":  true,
		"helix.starters.observability.tracing.exporter": "otlp",
	}

	cfg, err := resolveTracingConfig(loader, &tracingOptions{})
	if err != nil {
		t.Fatalf("resolveTracingConfig() error = %v", err)
	}

	if !cfg.Insecure {
		t.Fatal("Insecure = false, want true for backward compatibility")
	}
}
```

- [ ] **Step 2: Run failing observability tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./observability -run 'TestResolveTracingConfig_OTLPTransportOptions|TestResolveTracingConfig_DefaultOTLPInsecureRemainsCompatible' -count=1
```

Expected: FAIL because `TracingConfig.Insecure`, `TracingConfig.Headers`, and `TracingConfig.TLS` do not exist.

- [ ] **Step 3: Extend tracing config types**

In `observability/tracing.go`, add imports:

```go
"crypto/tls"
"crypto/x509"
"net/url"
```

Change `TracingConfig` to:

```go
type TracingConfig struct {
	Enabled     bool
	Exporter    string // "stdout" | "otlp" | "jaeger"
	Endpoint    string // OTLP HTTP endpoint, default "localhost:4318"
	ServiceName string // default "helix"
	Insecure    bool
	Headers     map[string]string
	TLS         TracingTLSConfig
}

// TracingTLSConfig holds optional TLS client settings for OTLP HTTP exporters.
type TracingTLSConfig struct {
	CAFile     string
	CertFile   string
	KeyFile    string
	ServerName string
}
```

Set the default in `resolveTracingConfig`:

```go
Insecure:    true,
Headers:     make(map[string]string),
```

- [ ] **Step 4: Resolve loader keys**

In `resolveTracingConfig`, after service-name resolution, add:

```go
if v, ok := loader.Lookup("helix.starters.observability.tracing.insecure"); ok {
	if enabled, parsed := boolValue(v); parsed {
		cfg.Insecure = enabled
	}
}
if v, ok := loader.Lookup("helix.starters.observability.tracing.headers"); ok {
	cfg.Headers = stringMapValue(v)
}
if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.ca-file"); ok {
	if s, ok := v.(string); ok {
		cfg.TLS.CAFile = strings.TrimSpace(s)
	}
}
if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.cert-file"); ok {
	if s, ok := v.(string); ok {
		cfg.TLS.CertFile = strings.TrimSpace(s)
	}
}
if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.key-file"); ok {
	if s, ok := v.(string); ok {
		cfg.TLS.KeyFile = strings.TrimSpace(s)
	}
}
if v, ok := loader.Lookup("helix.starters.observability.tracing.tls.server-name"); ok {
	if s, ok := v.(string); ok {
		cfg.TLS.ServerName = strings.TrimSpace(s)
	}
}
```

In the `o.cfgSet` block, add:

```go
cfg.Insecure = o.cfg.Insecure
if o.cfg.Headers != nil {
	cfg.Headers = cloneStringMap(o.cfg.Headers)
}
if o.cfg.TLS != (TracingTLSConfig{}) {
	cfg.TLS = o.cfg.TLS
}
```

Add helper functions near `validateExporter`:

```go
func boolValue(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		}
	}
	return false, false
}

func stringMapValue(value any) map[string]string {
	out := make(map[string]string)
	switch m := value.(type) {
	case map[string]string:
		for k, v := range m {
			key := strings.TrimSpace(k)
			if key != "" {
				out[key] = strings.TrimSpace(v)
			}
		}
	case map[string]any:
		for k, v := range m {
			key := strings.TrimSpace(k)
			if key == "" {
				continue
			}
			if s, ok := v.(string); ok {
				out[key] = strings.TrimSpace(s)
			}
		}
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 5: Build TLS client config**

Add this helper to `observability/tracing.go` before `buildExporter`:

```go
func buildTLSConfig(cfg TracingTLSConfig) (*tls.Config, error) {
	if cfg == (TracingTLSConfig{}) {
		return nil, nil
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12} //nolint:gosec
	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse ca file %q: %w", cfg.CAFile, ErrInvalidTracing)
		}
		tlsConfig.RootCAs = roots
	}

	if cfg.CertFile != "" || cfg.KeyFile != "" {
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return nil, fmt.Errorf("client certificate and key must be configured together: %w", ErrInvalidTracing)
		}
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
```

- [ ] **Step 6: Apply OTLP exporter options**

Replace the OTLP/Jaeger branch in `buildExporter`:

```go
case "otlp", "jaeger":
	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Endpoint),
		otlptracehttp.WithInsecure(),
	)
```

with:

```go
case "otlp", "jaeger":
	options := []otlptracehttp.Option{}
	if strings.HasPrefix(cfg.Endpoint, "http://") || strings.HasPrefix(cfg.Endpoint, "https://") {
		if _, err := url.ParseRequestURI(cfg.Endpoint); err != nil {
			return nil, fmt.Errorf("invalid endpoint url %q: %w", cfg.Endpoint, ErrInvalidTracing)
		}
		options = append(options, otlptracehttp.WithEndpointURL(cfg.Endpoint))
	} else {
		options = append(options, otlptracehttp.WithEndpoint(cfg.Endpoint))
	}
	if len(cfg.Headers) > 0 {
		options = append(options, otlptracehttp.WithHeaders(cfg.Headers))
	}
	if cfg.Insecure {
		options = append(options, otlptracehttp.WithInsecure())
	} else if tlsConfig, err := buildTLSConfig(cfg.TLS); err != nil {
		return nil, err
	} else if tlsConfig != nil {
		options = append(options, otlptracehttp.WithTLSClientConfig(tlsConfig))
	}
	return otlptracehttp.New(ctx, options...)
```

- [ ] **Step 7: Add TLS helper tests**

Append to `observability/tracing_test.go`:

```go
func TestBuildTLSConfigRejectsIncompleteClientCertificate(t *testing.T) {
	_, err := buildTLSConfig(TracingTLSConfig{CertFile: "client.crt"})
	if err == nil {
		t.Fatal("buildTLSConfig() error = nil, want error")
	}
	if !errors.Is(err, ErrInvalidTracing) {
		t.Fatalf("buildTLSConfig() error = %v, want ErrInvalidTracing", err)
	}
}

func TestBuildExporterAcceptsSecureOTLPEndpointWithHeaders(t *testing.T) {
	exp, err := buildExporter(context.Background(), TracingConfig{
		Enabled:  true,
		Exporter: "otlp",
		Endpoint: "https://otel.example.com:4318",
		Insecure: false,
		Headers: map[string]string{"Authorization": "Bearer token"},
	}, nil)
	if err != nil {
		t.Fatalf("buildExporter() error = %v", err)
	}
	if exp == nil {
		t.Fatal("buildExporter() exporter = nil")
	}
	if err := exp.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}
```

- [ ] **Step 8: Run observability tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./observability -count=1
```

Expected: PASS.

- [ ] **Step 9: Update tracing docs**

In `docs/reference/configuration-keys.md`, add rows under Observability / Tracing:

```markdown
| `observability.tracing.insecure` | bool | `true` | Use plaintext/insecure OTLP transport for backward compatibility |
| `observability.tracing.headers` | map[string]string | — | Static OTLP exporter headers, for example auth or tenant headers |
| `observability.tracing.tls.ca-file` | string | `""` | PEM CA bundle for OTLP TLS |
| `observability.tracing.tls.cert-file` | string | `""` | Client certificate for mTLS |
| `observability.tracing.tls.key-file` | string | `""` | Client certificate key for mTLS |
| `observability.tracing.tls.server-name` | string | `""` | TLS server name override |
```

Replace the tracing YAML example with:

```yaml
observability:
  tracing:
    enabled: true
    service-name: "my-api"
    exporter: otlp
    endpoint: "https://otel-collector:4318"
    insecure: false
    headers:
      Authorization: "Bearer ${OTEL_TOKEN}"
    tls:
      ca-file: "/etc/ssl/otel-ca.pem"
      server-name: "otel-collector"
```

Apply the equivalent French rows and YAML example to `docs/fr/reference/configuration-keys.md`.

In `docs/reference/starters.md` and `docs/fr/reference/starters.md`, update the tracing key table with the same new key names, using concise English/French descriptions.

- [ ] **Step 10: Commit OTLP TLS configuration**

Run:

```bash
git add observability/tracing.go observability/tracing_test.go docs/reference/configuration-keys.md docs/fr/reference/configuration-keys.md docs/reference/starters.md docs/fr/reference/starters.md
git commit -m "feat(observability): configure otlp transport security"
```

Expected: commit succeeds.

## Task 4: Audit and Close Cache Interceptor Limits

**Files:**
- Modify: `web/cache_interceptor.go`
- Modify: `web/cache_interceptor_test.go`
- Modify: `docs/guide/web.md`
- Modify: `docs/fr/guide/web.md`
- Modify: `lacunes.md`

- [ ] **Step 1: Run current cache tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./web -run 'TestCacheInterceptor|TestCacheStore' -count=1
```

Expected: PASS. If this fails, stop and fix the failing behavior before marking anything complete.

- [ ] **Step 2: Add failing stop idempotency proof**

Append to `web/cache_interceptor_test.go`:

```go
func TestCacheStoreStopIsIdempotent(t *testing.T) {
	store := newCacheStore()

	if err := store.Stop(); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	if err := store.Stop(); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}
```

- [ ] **Step 3: Run the new cache proof test**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./web -run 'TestCacheStoreStopIsIdempotent|TestCacheInterceptorSingleFlightPatternColdCache|TestCacheInterceptorMaxSize|TestCacheInterceptorProactiveSweep' -count=1
```

Expected: FAIL because current `Stop` waits on `sweepDone` again on the second call.

- [ ] **Step 4: Make cache store stop idempotent**

In `web/cache_interceptor.go`, add this field to `cacheStore` near the sweep fields:

```go
stopOnce sync.Once
```

Replace `Stop`:

```go
func (s *cacheStore) Stop() error {
	s.sweepTicker.Stop()
	s.sweepCancel()
	<-s.sweepDone
	return nil
}
```

with:

```go
func (s *cacheStore) Stop() error {
	s.stopOnce.Do(func() {
		s.sweepTicker.Stop()
		s.sweepCancel()
		<-s.sweepDone
	})
	return nil
}
```

- [ ] **Step 5: Rerun cache proof tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./web -run 'TestCacheStoreStopIsIdempotent|TestCacheInterceptorSingleFlightPatternColdCache|TestCacheInterceptorMaxSize|TestCacheInterceptorProactiveSweep' -count=1
```

Expected: PASS.

- [ ] **Step 6: Document cache directive options**

In `docs/guide/web.md`, ensure the Cache interceptor section documents:

```markdown
The built-in cache interceptor accepts:

- `cache:<duration>` for TTL, for example `cache:5m`.
- `cache:<duration>:max=<entries>` to cap stored responses.
- `cache:<duration>:lru` or `cache:<duration>:fifo` to choose eviction order.

Only successful GET JSON responses are cached. Concurrent cold requests for the same URL are coalesced so one handler computes the response and waiters replay that result. Expired entries are removed lazily on read and proactively by a background sweep. Responses larger than the framework cache body limit are not stored.
```

Add the French equivalent to `docs/fr/guide/web.md`:

```markdown
L'interceptor de cache integre accepte :

- `cache:<duration>` pour le TTL, par exemple `cache:5m`.
- `cache:<duration>:max=<entries>` pour limiter le nombre de reponses stockees.
- `cache:<duration>:lru` ou `cache:<duration>:fifo` pour choisir l'ordre d'eviction.

Seules les reponses JSON GET reussies sont mises en cache. Les requetes concurrentes a froid vers la meme URL sont coalescees : un seul handler calcule la reponse et les autres requetes rejouent ce resultat. Les entrees expirees sont supprimees a la lecture et par un sweep periodique. Les reponses plus grandes que la limite de corps du cache ne sont pas stockees.
```

- [ ] **Step 7: Mark cache lacune complete**

In `lacunes.md`, change:

```markdown
- [ ] Corriger les limites connues du cache interceptor
```

to:

```markdown
- [x] Corriger les limites connues du cache interceptor
```

Add this proof bullet under the validation bullet:

```markdown
  - Preuve: `web/cache_interceptor.go` implemente coalescing cold-key, limite `max`, strategies LRU/FIFO et sweep periodique; `web/cache_interceptor_test.go` couvre concurrence, TTL, eviction taille, stop et reponses non cacheables; `docs/guide/web.md` documente le contrat public.
```

- [ ] **Step 8: Commit cache proof and docs**

Run:

```bash
git add web/cache_interceptor.go web/cache_interceptor_test.go docs/guide/web.md docs/fr/guide/web.md lacunes.md
git commit -m "docs(web): close cache interceptor limits"
```

Expected: commit succeeds.

## Task 5: Mark Completed P1 Items and Run Full Verification

**Files:**
- Modify: `lacunes.md`

- [ ] **Step 1: Run targeted package tests**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./core ./starter/... ./observability ./web -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full test suite**

Run:

```bash
/Users/yacoubakone/.govm/go/bin/go test ./...
```

Expected: PASS.

- [ ] **Step 3: Mark completed P1 items**

If Task 2 and Task 3 passed and docs were committed, update `lacunes.md`:

```markdown
- [x] Gerer les echecs partiels de configuration des starters
```

Add:

```markdown
  - Preuve: les starters web, data et scheduling annulent leurs registrations precedentes quand une registration suivante echoue; les tests `starter/...` couvrent les echecs forces et prouvent qu'aucun composant orphelin resolvable ne reste actif.
```

Change:

```markdown
- [x] Ajouter une option TLS pour les exporters OTLP
```

Add:

```markdown
  - Preuve: `observability.TracingConfig` expose `insecure`, `headers` et options TLS; `observability/tracing_test.go` couvre resolution de config et construction exporter; les docs reference listent les cles production OTLP.
```

Do not mark any task that did not pass its tests.

- [ ] **Step 4: Commit lacunes completion status**

Run:

```bash
git add lacunes.md
git commit -m "docs: mark completed p1 robustness tasks"
```

Expected: commit succeeds if `lacunes.md` changed. If no additional item was completed, skip this commit and note why in the final report.

- [ ] **Step 5: Inspect final status**

Run:

```bash
git status --short
```

Expected: clean worktree.
