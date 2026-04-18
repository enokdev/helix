package data

// Transaction wraps an adapter-specific transaction without exposing the ORM.
// Implementations must not return nil from Unwrap.
type Transaction[TX any] interface {
	Unwrap() TX
}
