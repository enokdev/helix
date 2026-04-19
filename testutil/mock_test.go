package testutil

import (
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
	*r.started++
	return nil
}

func (r *realMockBeanRepository) OnStop() error {
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

func (m *mockMockBeanRepository) OnStop() error {
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
	if realStarts != 0 {
		t.Fatalf("realStarts = %d, want 0", realStarts)
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
