package data

import "errors"

var (
	// ErrRecordNotFound is returned when a repository cannot find the requested record.
	ErrRecordNotFound = errors.New("data: record not found")
	// ErrDuplicateKey is returned when a repository detects a unique key conflict.
	ErrDuplicateKey = errors.New("data: duplicate key")
	// ErrInvalidFilter is returned when a filter cannot be translated safely.
	ErrInvalidFilter = errors.New("data: invalid filter")
)
