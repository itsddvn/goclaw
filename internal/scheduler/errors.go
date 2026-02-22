package scheduler

import "errors"

var (
	// ErrQueueFull is returned when a message is rejected because the session queue is full (drop=new policy).
	ErrQueueFull = errors.New("session queue is full")

	// ErrQueueDropped is returned when a queued message is evicted to make room (drop=old policy).
	ErrQueueDropped = errors.New("message dropped from queue")
)
