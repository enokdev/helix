package scheduler

import "errors"

// ErrInvalidCron is returned when a cron expression is malformed.
var ErrInvalidCron = errors.New("scheduler: invalid cron expression")
