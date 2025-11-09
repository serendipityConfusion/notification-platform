package distribute_lock

import (
	"context"
	"time"
)

type Client interface {
	NewLock(ctx context.Context, key string, opts *LockerOption) DistributeMuter
}

type DistributeMuter interface {
	Lock() error
	Unlock() error
}

type LockerOption struct {
	// default 5s
	Expiration time.Duration
	// default 0
	RetryCount int
	// default 100ms
	RetryDelay time.Duration
}
