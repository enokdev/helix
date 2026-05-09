package data

import "errors"

var (
	// ErrRecordNotFound is returned when a repository cannot find the requested record.
	ErrRecordNotFound = errors.New("data: record not found")
	// ErrDuplicateKey is returned when a repository detects a unique key conflict.
	ErrDuplicateKey = errors.New("data: duplicate key")
	// ErrInvalidPagination is returned when a pagination request cannot be executed safely.
	// This covers both out-of-range inputs (page < 1 or size < 1) and arithmetic
	// overflow cases where (page-1)*size would exceed the platform's max int.
	ErrInvalidPagination = errors.New("data: invalid pagination")
	// ErrInvalidFilter is returned when a filter cannot be translated safely.
	ErrInvalidFilter = errors.New("data: invalid filter")
	// ErrInvalidCondition is returned when a filter condition cannot be translated safely.
	ErrInvalidCondition = errors.New("data: invalid condition")
)
