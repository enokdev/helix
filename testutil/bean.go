package testutil

// GetBean resolves a component from the test app container and fails the test
// immediately if resolution is impossible.
func GetBean[T any](app *App) T {
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.t.Helper()

	var target T
	if err := app.container.Resolve(&target); err != nil {
		app.t.Fatalf("testutil: get bean %T: %v", target, err)
	}
	return target
}
