package testutil

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/enokdev/helix/core"
)

type mockBeanRepository interface {
	FindName() string
}

type realMockBeanRepository struct {
	started *int
}

func (r *realMockBeanRepository) FindName() string {
	return "real"
}

func (r *realMockBeanRepository) OnStart() error {
	if r.started != nil {
		*r.started++
	}
	return nil
}

func (r *realMockBeanRepository) OnStop(_ context.Context) error {
	return nil
}

type mockMockBeanRepository struct {
	started *int
	name    string
}

func (m *mockMockBeanRepository) FindName() string {
	return m.name
}

func (m *mockMockBeanRepository) OnStart() error {
	if m.started != nil {
		*m.started++
	}
	return nil
}

func (m *mockMockBeanRepository) OnStop(_ context.Context) error {
	return nil
}

type nonRegistrableMockBeanRepository struct{}

func (m nonRegistrableMockBeanRepository) FindName() string {
	return "value"
}

type mockBeanNotifier interface {
	Notify() string
}

type realMockBeanNotifier struct{}

func (n *realMockBeanNotifier) Notify() string {
	return "real-notifier"
}

type mockMockBeanNotifier struct {
	message string
}

func (n *mockMockBeanNotifier) Notify() string {
	return n.message
}

type untouchedMockBeanDependency struct {
	value string
}

type mockBeanService struct {
	Repository mockBeanRepository           `inject:"true"`
	Notifier   mockBeanNotifier             `inject:"true"`
	Dependency *untouchedMockBeanDependency `inject:"true"`
}

type mockBeanMailer interface {
	Send() string
}

type mockBeanHealthIndicator interface {
	Health() string
}

type realMultiInterfaceMailer struct {
	started *int
}

func (m *realMultiInterfaceMailer) Send() string {
	return "real-mailer"
}

func (m *realMultiInterfaceMailer) Health() string {
	return "healthy"
}

func (m *realMultiInterfaceMailer) OnStart() error {
	if m.started != nil {
		*m.started++
	}
	return nil
}

func (m *realMultiInterfaceMailer) OnStop(_ context.Context) error {
	return nil
}

type mockMultiInterfaceMailer struct{}

func (m *mockMultiInterfaceMailer) Send() string {
	return "mock-mailer"
}

func (m *mockMultiInterfaceMailer) Health() string {
	return "mock-health"
}

type mockBeanMailService struct {
	Mailer mockBeanMailer          `inject:"true"`
	Health mockBeanHealthIndicator `inject:"true"`
}

type alternateMultiInterfaceMailer struct{}

func (m *alternateMultiInterfaceMailer) Send() string {
	return "alternate-mailer"
}

func (m *alternateMultiInterfaceMailer) Health() string {
	return "alternate-health"
}

func TestMockBeanReplacesInterfaceImplementation(t *testing.T) {
	t.Parallel()

	realStarts := 0
	mockStarts := 0
	mockRepo := &mockMockBeanRepository{started: &mockStarts, name: "mock"}

	app := NewApp(t,
		WithComponents(
			&realMockBeanRepository{started: &realStarts},
			&mockBeanService{},
			&untouchedMockBeanDependency{value: "kept"},
			&mockMockBeanNotifier{message: "notifier"},
		),
		MockBean[mockBeanRepository](mockRepo),
	)

	service := GetBean[*mockBeanService](app)
	if service.Repository.FindName() != "mock" {
		t.Fatalf("service.Repository.FindName() = %q, want mock", service.Repository.FindName())
	}
	if service.Dependency.value != "kept" {
		t.Fatalf("service.Dependency.value = %q, want kept", service.Dependency.value)
	}
	if realStarts != 1 {
		t.Fatalf("realStarts = %d, want 1 because real component remains registered for lifecycle", realStarts)
	}
	if mockStarts != 1 {
		t.Fatalf("mockStarts = %d, want 1", mockStarts)
	}
}

func TestMockBeanSupportsMultipleMocks(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithComponents(
			&realMockBeanRepository{},
			&realMockBeanNotifier{},
			&mockBeanService{},
			&untouchedMockBeanDependency{value: "kept"},
		),
		MockBean[mockBeanRepository](&mockMockBeanRepository{name: "repo-mock"}),
		MockBean[mockBeanNotifier](&mockMockBeanNotifier{message: "notify-mock"}),
	)

	service := GetBean[*mockBeanService](app)
	if service.Repository.FindName() != "repo-mock" {
		t.Fatalf("service.Repository.FindName() = %q, want repo-mock", service.Repository.FindName())
	}
	if service.Notifier.Notify() != "notify-mock" {
		t.Fatalf("service.Notifier.Notify() = %q, want notify-mock", service.Notifier.Notify())
	}
	if service.Dependency.value != "kept" {
		t.Fatalf("service.Dependency.value = %q, want kept", service.Dependency.value)
	}
}

func TestMockBeanCanProvideOnlyImplementationForInterface(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithComponents(&mockBeanService{}, &untouchedMockBeanDependency{}),
		MockBean[mockBeanRepository](&mockMockBeanRepository{name: "only-mock"}),
		MockBean[mockBeanNotifier](&mockMockBeanNotifier{message: "only-notifier"}),
	)

	service := GetBean[*mockBeanService](app)
	if service.Repository.FindName() != "only-mock" {
		t.Fatalf("service.Repository.FindName() = %q, want only-mock", service.Repository.FindName())
	}
	if service.Notifier.Notify() != "only-notifier" {
		t.Fatalf("service.Notifier.Notify() = %q, want only-notifier", service.Notifier.Notify())
	}
}

func TestMockBeanPreservesNonTargetInterfacesOnRealComponent(t *testing.T) {
	t.Parallel()

	realStarts := 0
	app := NewApp(t,
		WithComponents(
			&realMultiInterfaceMailer{started: &realStarts},
			&mockBeanMailService{},
		),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	service := GetBean[*mockBeanMailService](app)
	if service.Mailer.Send() != "mock-mailer" {
		t.Fatalf("service.Mailer.Send() = %q, want mock-mailer", service.Mailer.Send())
	}
	if service.Health.Health() != "healthy" {
		t.Fatalf("service.Health.Health() = %q, want healthy", service.Health.Health())
	}
	if realStarts != 1 {
		t.Fatalf("realStarts = %d, want 1 because real component remains registered for health", realStarts)
	}
}

func TestMockBeanPreservesLifecycleOnRealComponent(t *testing.T) {
	t.Parallel()

	realStarts := 0
	app := NewApp(t,
		WithComponents(&realMultiInterfaceMailer{started: &realStarts}),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	mailer := GetBean[mockBeanMailer](app)
	if mailer.Send() != "mock-mailer" {
		t.Fatalf("mailer.Send() = %q, want mock-mailer", mailer.Send())
	}
	if realStarts != 1 {
		t.Fatalf("realStarts = %d, want 1 because lifecycle remains registered", realStarts)
	}
}

func TestMockBeanMultiInterfaceMockDoesNotCreateAmbiguity(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithComponents(
			&realMultiInterfaceMailer{},
			&mockBeanMailService{},
		),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	service := GetBean[*mockBeanMailService](app)
	if service.Mailer.Send() != "mock-mailer" {
		t.Fatalf("service.Mailer.Send() = %q, want mock-mailer", service.Mailer.Send())
	}
	if service.Health.Health() != "healthy" {
		t.Fatalf("service.Health.Health() = %q, want healthy", service.Health.Health())
	}
}

func TestMockBeanPreservesNonTargetInterfaceForDirectResolution(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithComponents(&realMultiInterfaceMailer{}),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	mailer := GetBean[mockBeanMailer](app)
	if mailer.Send() != "mock-mailer" {
		t.Fatalf("mailer.Send() = %q, want mock-mailer", mailer.Send())
	}

	health := GetBean[mockBeanHealthIndicator](app)
	if health.Health() != "healthy" {
		t.Fatalf("health.Health() = %q, want healthy", health.Health())
	}
}

func TestMockBeanPreservesNonTargetInterfaceAmbiguity(t *testing.T) {
	t.Parallel()

	app := NewApp(t,
		WithComponents(
			&realMultiInterfaceMailer{},
			&alternateMultiInterfaceMailer{},
		),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	var health mockBeanHealthIndicator
	err := app.Container().Resolve(&health)
	if err == nil {
		t.Fatal("Resolve(mockBeanHealthIndicator) error = nil, want ambiguity")
	}
	if !errors.Is(err, core.ErrUnresolvable) {
		t.Fatalf("Resolve(mockBeanHealthIndicator) error = %v, want core.ErrUnresolvable", err)
	}
	if !strings.Contains(err.Error(), "multiple registrations") {
		t.Fatalf("Resolve(mockBeanHealthIndicator) error = %q, want multiple registrations", err)
	}
}

func TestMockBeanRecognizesComponentRegistration(t *testing.T) {
	t.Parallel()

	registration := core.NewComponentRegistration(&realMultiInterfaceMailer{})
	registration.Scope = core.ScopePrototype

	app := NewApp(t,
		WithComponents(
			registration,
			&mockBeanMailService{},
		),
		MockBean[mockBeanMailer](&mockMultiInterfaceMailer{}),
	)

	service := GetBean[*mockBeanMailService](app)
	if service.Mailer.Send() != "mock-mailer" {
		t.Fatalf("service.Mailer.Send() = %q, want mock-mailer", service.Mailer.Send())
	}
	if service.Health.Health() != "healthy" {
		t.Fatalf("service.Health.Health() = %q, want healthy", service.Health.Health())
	}

	first := GetBean[mockBeanHealthIndicator](app)
	second := GetBean[mockBeanHealthIndicator](app)
	if first == second {
		t.Fatal("prototype ComponentRegistration returned the same health instance twice")
	}
}

func TestValidateMockBeansRejectsInvalidDefinitions(t *testing.T) {
	t.Parallel()

	targetRepository := reflect.TypeOf((*mockBeanRepository)(nil)).Elem()
	targetAny := reflect.TypeOf((*any)(nil)).Elem()

	tests := []struct {
		name    string
		mocks   []mockBean
		wantErr string
	}{
		{
			name: "duplicate target",
			mocks: []mockBean{
				{target: targetRepository, impl: &mockMockBeanRepository{}},
				{target: targetRepository, impl: &mockMockBeanRepository{}},
			},
			wantErr: "duplicate",
		},
		{
			name:    "nil mock",
			mocks:   []mockBean{{target: targetRepository, impl: nil}},
			wantErr: "nil",
		},
		{
			name:    "non registrable mock",
			mocks:   []mockBean{{target: targetRepository, impl: nonRegistrableMockBeanRepository{}}},
			wantErr: "non-nil pointer to struct",
		},
		{
			name:    "implementation not assignable",
			mocks:   []mockBean{{target: targetRepository, impl: &mockMockBeanNotifier{}}},
			wantErr: "not assignable",
		},
		{
			name:    "empty interface target",
			mocks:   []mockBean{{target: targetAny, impl: &mockMockBeanRepository{}}},
			wantErr: "empty interface",
		},
		{
			name:    "non-interface target",
			mocks:   []mockBean{{target: reflect.TypeOf((*mockMockBeanRepository)(nil)), impl: &mockMockBeanRepository{}}},
			wantErr: "must be an interface",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateMockBeans(tt.mocks)
			if err == nil {
				t.Fatal("validateMockBeans() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateMockBeans() error = %q, want substring %q", err, tt.wantErr)
			}
			if !errors.Is(err, core.ErrUnresolvable) {
				t.Fatalf("validateMockBeans() error = %q, want wrapped core.ErrUnresolvable", err)
			}
		})
	}
}
