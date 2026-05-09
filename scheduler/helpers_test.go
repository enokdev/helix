package scheduler

// jobFuncForTest retrieves the raw job function by name, for use in unit tests.
func (w *adapterWrapper) jobFuncForTest(name string) func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	job, ok := w.byName[name]
	if !ok {
		return nil
	}
	return job.fn
}
