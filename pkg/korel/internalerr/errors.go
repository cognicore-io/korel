package internalerr

import "errors"

// Sentinel errors for common cases
var (
	ErrNotFound         = errors.New("not found")
	ErrInvalidInput     = errors.New("invalid input")
	ErrDuplicate        = errors.New("duplicate entry")
	ErrStoreUnavailable = errors.New("store unavailable")
	ErrInvalidConfig    = errors.New("invalid configuration")
)
