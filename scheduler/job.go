package scheduler

// CronExpression is a standard 5-field cron expression or a robfig @-shortcut.
// Examples: "0 0 * * *" (daily midnight), "@every 1h", "@hourly".
type CronExpression = string

// Job is a named task to be executed on a cron schedule.
type Job struct {
	// Name uniquely identifies the job for logging and error reporting.
	Name string
	// Expr is the cron schedule expression.
	Expr CronExpression
	// Fn is the function to invoke on schedule.
	Fn func()
}
