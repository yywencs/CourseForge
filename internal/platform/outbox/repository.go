package outbox

import (
	"context"
	"time"
)

type Repository interface {
	ClaimPending(
		ctx context.Context,
		limit int,
		now time.Time,
		lease time.Duration,
	) ([]*Event, error)
	MarkPublished(
		ctx context.Context,
		eventID uint64,
		publishedAt time.Time,
	) error
	MarkFailed(
		ctx context.Context,
		eventID uint64,
		nextRetryAt time.Time,
		lastError string,
	) error
}

type Writer interface {
	Append(context.Context, *NewEvent) error
	AppendBatch(context.Context, []*NewEvent) error
}

type BacklogReader interface {
	ReadBacklog(context.Context) (*Backlog, error)
}
