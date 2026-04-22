package scheduler

// ScheduledJobProvider exposes cron jobs declared by a Helix component.
//
// Components can use this runtime interface with methods documented by
// //helix:scheduled comments today; future code generation can derive the
// implementation from those comments automatically.
type ScheduledJobProvider interface {
	ScheduledJobs() []Job
}
