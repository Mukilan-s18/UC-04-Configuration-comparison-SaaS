package repository

import "errors"

var (
	ErrNotFound              = errors.New("record not found")
	ErrQuotaExceeded         = errors.New("daily quota exhausted")
	ErrIdempotencyConflict   = errors.New("idempotency key reused with different input")
	ErrRunInProgress         = errors.New("RUN_IN_PROGRESS")
	ErrKeyCollision          = ErrRunInProgress
	ErrSuspendedSubscription = errors.New("subscription is suspended")
	ErrUnauthorizedTenant    = errors.New("tenant mismatch or unauthorized")
)
