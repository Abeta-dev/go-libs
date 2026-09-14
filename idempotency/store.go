// SPDX-License-Identifier: MIT

// Package idempotency provides two-phase atomic idempotency locking and storage.
package idempotency

import (
	"context"
	"errors"
)

// ErrAlreadyInProgress is returned if a client requests an operation that is currently executing.
var ErrAlreadyInProgress = errors.New("idempotent operation already in progress")

// Response encapsulates the HTTP response to be returned to the client.
type Response struct {
	StatusCode int
	Headers    map[string][]string
	Body       []byte
}

// Status represents the current state of an idempotent operation.
type Status string

// Idempotent operation lifecycle states.
const (
	StatusInProgress Status = "IN_PROGRESS"
	StatusCompleted  Status = "COMPLETED"
)

// Record represents a stored idempotency record.
type Record struct {
	Status   Status
	Response *Response
}

// Store defines the interface for an idempotency storage backend.
type Store interface {
	// Lock attempts to start a new idempotent operation.
	// If the key doesn't exist, it is created with StatusInProgress, and (false, nil, nil) is returned.
	// If the key exists, it returns (true, &Record, nil).
	Lock(ctx context.Context, key string) (exists bool, record *Record, err error)

	// Save marks an operation as StatusCompleted and stores the response.
	Save(ctx context.Context, key string, response Response) error

	// Unlock releases an in-progress lock without saving a response, e.g. on operation failure.
	// If the record exists and is StatusInProgress, it is removed so the operation can be retried.
	// If the record is already StatusCompleted, Unlock is a no-op and returns nil.
	Unlock(ctx context.Context, key string) error

	// Delete removes an idempotency record completely regardless of status.
	Delete(ctx context.Context, key string) error
}
