package scheduler

import "errors"

var (
	// ErrInvalidCron is returned when a cron expression is malformed.
	ErrInvalidCron = errors.New("scheduler: invalid cron expression")

	// ErrJobNotFound is returned when a requested job does not exist.
	ErrJobNotFound = errors.New("scheduler: job not found")
)
