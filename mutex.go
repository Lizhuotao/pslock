package pslock

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	lockPrefix = "distributed_lock:"
)

// A DelayFunc is used to decide the amount of time to wait between retries.
type DelayFunc func(tries int) time.Duration

// Mutex represents a distributed lock implementation
type Mutex struct {
	client *redis.Client
	// The maximum waiting time if the lock is not obtained
	patient time.Duration
	name    string
	key     string
	expiry  time.Duration

	tries     int
	delayFunc DelayFunc
}

// Name returns mutex name (i.e. the Redis key).
func (m *Mutex) Name() string {
	return m.name
}

// Lock attempts to acquire a distributed lock
func (dl *Mutex) Lock(ctx context.Context) error {
	lockKey := lockPrefix + dl.key

	// Try to acquire the lock using SETNX
	success, err := dl.client.SetNX(ctx, lockKey, "1", dl.expiry).Result()
	// fmt.Println("got lock:", dl.name, lockKey, success)

	if err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}

	if success {
		return nil
	}

	// If lock acquisition failed, enter blocking flow
	return dl.blockingLock(ctx)
}

func (dl *Mutex) getKey() string {
	return lockPrefix + dl.key
}

// Unlock releases the distributed lock
func (dl *Mutex) Unlock(ctx context.Context) error {
	lockKey := dl.getKey()

	// Delete the lock key
	_, err := dl.client.Del(ctx, lockKey).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// fmt.Printf("id: %s release key\n", dl.name)
	// Publish unlock message to notify waiting goroutines
	err = dl.client.Publish(ctx, lockKey, "unlock").Err()
	if err != nil {
		return fmt.Errorf("failed to publish unlock message: %w", err)
	}
	// fmt.Printf("id: %s pub mes\n", dl.name)

	return nil
}

// blockingLock implements the blocking flow for lock acquisition
func (dl *Mutex) blockingLock(ctx context.Context) error {
	lockKey := dl.getKey()

	// Subscribe to Redis channel for unlock notifications
	sub := dl.client.Subscribe(ctx, lockKey)
	defer sub.Close()

	if _, err := sub.Receive(ctx); err != nil {
		fmt.Printf("sub error: %v\n", err)
		return nil
	}

	msgCh := sub.Channel()

	// Create a context with timeout for the entire blocking operation
	blockCtx, cancel := context.WithTimeout(ctx, dl.patient)
	defer cancel()

	// Start polling attempts
	pollDone := make(chan struct{})
	msgDone := make(chan struct{})

	go func() {
		for i := range dl.tries {
			if i == dl.tries-1 {
				close(pollDone)
				return
			}

			select {
			case <-blockCtx.Done():
				return
			case <-msgDone:
				close(pollDone)
				return
			case <-time.After(dl.delayFunc(i)):
				// fmt.Printf("id: %s, try %d\n", dl.name, i)
				success, err := dl.client.SetNX(blockCtx, lockKey, "1", dl.expiry).Result()
				if err == nil && success {
					close(pollDone)
					cancel()

					return
				}
			}
		}
	}()

	// Wait for either polling success or unlock notification
	select {
	case <-pollDone:
		// Polling succeeded, cancel subscription
		return nil
	case <-msgCh:
		// fmt.Printf("id: %s, got mes\n", dl.name)
		close(msgDone)
		return dl.Lock(blockCtx)
		// return nil
	case <-blockCtx.Done():
		return fmt.Errorf("lock acquisition timeout")
	}
}
