package core

import "testing"

func BenchmarkReflectResolverRegisterAndResolve(b *testing.B) {
	for i := 0; i < b.N; i++ {
		resolver := NewReflectResolver()
		if err := resolver.Register(&benchmarkDependency{}); err != nil {
			b.Fatalf("Register(dependency) error = %v", err)
		}
		if err := resolver.Register(&benchmarkService{}); err != nil {
			b.Fatalf("Register(service) error = %v", err)
		}

		var service *benchmarkService
		if err := resolver.Resolve(&service); err != nil {
			b.Fatalf("Resolve(service) error = %v", err)
		}
	}
}

func BenchmarkReflectResolverResolveSingleton(b *testing.B) {
	resolver := NewReflectResolver()
	if err := resolver.Register(&benchmarkDependency{}); err != nil {
		b.Fatalf("Register(dependency) error = %v", err)
	}
	if err := resolver.Register(&benchmarkService{}); err != nil {
		b.Fatalf("Register(service) error = %v", err)
	}
	var service *benchmarkService
	if err := resolver.Resolve(&service); err != nil {
		b.Fatalf("warm Resolve(service) error = %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service = nil
		if err := resolver.Resolve(&service); err != nil {
			b.Fatalf("Resolve(service) error = %v", err)
		}
	}
}

type benchmarkDependency struct {
	Name string
}

type benchmarkService struct {
	Dependency *benchmarkDependency `inject:"true"`
}
