package scheduler

import "errors"

// ErrInvalidCron is returned when a cron expression is malformed.
var ErrInvalidCron = errors.New("scheduler: invalid cron expression")

// ErrInvalidJob is returned when a job definition is incomplete or invalid.
var ErrInvalidJob = errors.New("scheduler: invalid job")

// ErrDuplicateJob is returned when a job name is already registered.
var ErrDuplicateJob = errors.New("scheduler: duplicate job")

// ErrJobNotFound is returned when a registered job cannot be found.
var ErrJobNotFound = errors.New("scheduler: job not found")
