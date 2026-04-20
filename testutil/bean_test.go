package testutil

import "testing"

type testGreeter interface {
	Greet() string
}

type testGreeterImpl struct{}

func (g *testGreeterImpl) Greet() string {
	return "hello"
}

func TestGetBeanResolvesInterfaces(t *testing.T) {
	t.Parallel()

	app := NewApp(t, WithComponents(&testGreeterImpl{}))

	greeter := GetBean[testGreeter](app)
	if greeter.Greet() != "hello" {
		t.Fatalf("greeter.Greet() = %q, want hello", greeter.Greet())
	}
}
