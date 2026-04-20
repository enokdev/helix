package web

import "time"

// RouteObservation carries the observed data for a single handled HTTP request.
type RouteObservation struct {
	Method     string
	Route      string
	StatusCode int
	Duration   time.Duration
}

// RouteObserver receives observations about handled HTTP requests.
type RouteObserver interface {
	Observe(RouteObservation)
}
