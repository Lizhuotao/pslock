package pslock

import (
	"context"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	minRetryDelayMilliSec = 50
	maxRetryDelayMilliSec = 250
)

// Redsync provides a simple method for creating distributed mutexes using multiple Redis connection pools.
type PSLock struct {
	client *redis.Client
}

// New creates and returns a new Redsync instance from given Redis connection pools.
func New(c *redis.Client) *PSLock {
	cmd := c.Ping(context.Background())
	if cmd.Err() != nil {
		panic(cmd.Err())
	}
	return &PSLock{
		client: c,
	}
}

// NewMutex returns a new distributed mutex with given name.
func (r PSLock) NewMutex(key string, options ...Option) *Mutex {

	m := &Mutex{
		client:  r.client,
		key:     key,
		name:    key,
		expiry:  8 * time.Second,
		patient: 8 * time.Second,
		tries:   32,
		delayFunc: func(tries int) time.Duration {
			return time.Duration(rand.Intn(maxRetryDelayMilliSec-minRetryDelayMilliSec)+minRetryDelayMilliSec) * time.Millisecond
		},
	}
	for _, o := range options {
		o.Apply(m)
	}
	return m
}

// An Option configures a mutex.
type Option interface {
	Apply(*Mutex)
}

// OptionFunc is a function that configures a mutex.
type OptionFunc func(*Mutex)

// Apply calls f(mutex)
func (f OptionFunc) Apply(mutex *Mutex) {
	f(mutex)
}

// WithExpiry can be used to set the expiry of a mutex to the given value.
// The default is 8s.
func WithName(name string) Option {
	return OptionFunc(func(m *Mutex) {
		m.name = name
	})
}

// WithExpiry can be used to set the expiry of a mutex to the given value.
// The default is 8s.
func WithExpiry(expiry time.Duration) Option {
	return OptionFunc(func(m *Mutex) {
		m.expiry = expiry
	})
}

// WithTries can be used to set the number of times lock acquire is attempted.
// The default value is 32.
func WithTries(tries int) Option {
	return OptionFunc(func(m *Mutex) {
		m.tries = tries
	})
}

// WithRetryDelay can be used to set the amount of time to wait between retries.
// The default value is rand(50ms, 250ms).
func WithRetryDelay(delay time.Duration) Option {
	return OptionFunc(func(m *Mutex) {
		m.delayFunc = func(tries int) time.Duration {
			return delay
		}
	})
}

// WithRetryDelayFunc can be used to override default delay behavior.
func WithRetryDelayFunc(delayFunc DelayFunc) Option {
	return OptionFunc(func(m *Mutex) {
		m.delayFunc = delayFunc
	})
}
