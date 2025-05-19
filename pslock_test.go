package pslock

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/lizhuotao/pslock/looplock"
	"github.com/redis/go-redis/v9"
)

func mockRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}

func TestNewMutex_DefaultOptions(t *testing.T) {
	r := New(mockRedisClient())
	mutex := r.NewMutex("test-mutex")
	if mutex.name != "test-mutex" {
		t.Errorf("expected mutex name 'test-mutex', got %s", mutex.name)
	}
	if mutex.expiry != 8*time.Second {
		t.Errorf("expected default expiry 8s, got %v", mutex.expiry)
	}
	if mutex.tries != 32 {
		t.Errorf("expected default tries 32, got %d", mutex.tries)
	}
	if mutex.delayFunc == nil {
		t.Error("expected default delayFunc to be set")
	}
}

func TestNewMutex_WithOptions(t *testing.T) {
	r := New(mockRedisClient())
	expiry := 3 * time.Second
	tries := 5
	delay := 100 * time.Millisecond
	mutex := r.NewMutex("test-mutex-opt",
		WithExpiry(expiry),
		WithTries(tries),
		WithRetryDelay(delay),
	)
	if mutex.expiry != expiry {
		t.Errorf("expected expiry %v, got %v", expiry, mutex.expiry)
	}
	if mutex.tries != tries {
		t.Errorf("expected tries %d, got %d", tries, mutex.tries)
	}
	if mutex.delayFunc(0) != delay {
		t.Errorf("expected delayFunc to return %v, got %v", delay, mutex.delayFunc(0))
	}
}

func randTimeout() time.Duration {
	return time.Duration(rand.Intn(maxRetryDelayMilliSec-minRetryDelayMilliSec)+minRetryDelayMilliSec) * time.Millisecond
}

func TestConcurrentLockAcquireAndRelease(t *testing.T) {
	var successCount int
	var waitTime time.Duration
	loopCount := 10

	start := time.Now()
	for i := range loopCount {
		fmt.Printf("start loop: %d\n", i)
		waitTime += psworker(&successCount)
		// waitTime += loopworker(&successCount)
		// waitTime += redsyncworker(&successCount)
	}

	if successCount == 0 {
		t.Error("expected at least one goroutine to acquire the lock")
	}
	if successCount != loopCount*3 {
		t.Errorf("expected successCount = %d", loopCount*3)
	}
	useTime := time.Since(start)
	fmt.Printf("useTime: %v, waitTime: %v, scheduleTime: %v", useTime, waitTime, useTime-waitTime)
}

func psworker(suc *int) time.Duration {
	client := mockRedisClient()
	defer client.Close()
	r := New(client)
	name := "my-red-lock"

	done := make(chan struct{})
	var successCount int
	var waitTime time.Duration

	threadCount := 3

	start := make(chan struct{})

	for i := 0; i < threadCount; i++ {
		go func(id int) {
			<-start
			mutex := r.NewMutex(name, WithExpiry(8*time.Second), WithName(fmt.Sprintf("%s-%d", name, id)))

			if err := mutex.Lock(context.Background()); err == nil {
				// fmt.Printf("id: %d got lock\n", id)

				*suc++
				timeout := randTimeout()
				<-time.After(timeout)
				waitTime += timeout

				// time.Sleep(150 * time.Millisecond)
				mutex.Unlock(context.Background())
				successCount++
				done <- struct{}{}
			}
		}(i)
	}
	close(start)

	for i := 0; i < threadCount; i++ {
		<-done
	}
	return waitTime
}

func loopworker(suc *int) time.Duration {
	client := mockRedisClient()
	defer client.Close()
	r := looplock.New(client)
	name := "my-red-lock"

	done := make(chan struct{})
	var successCount int
	var waitTime time.Duration

	threadCount := 3

	start := make(chan struct{})

	for i := 0; i < threadCount; i++ {
		go func(id int) {
			<-start
			opts := []looplock.Option{
				looplock.WithExpiry(8 * time.Second),
				looplock.WithName(fmt.Sprintf("%s-%d", name, id)),
				looplock.WithRetryDelay(30 * time.Millisecond),
			}
			mutex := r.NewMutex(name, opts...)

			if err := mutex.Lock(context.Background()); err == nil {
				// fmt.Printf("id: %d got lock\n", id)

				*suc++
				timeout := randTimeout()
				<-time.After(timeout)
				waitTime += timeout

				// time.Sleep(150 * time.Millisecond)
				mutex.Unlock(context.Background())
				successCount++
				done <- struct{}{}
			}
		}(i)
	}
	close(start)

	for i := 0; i < threadCount; i++ {
		<-done
	}
	return waitTime
}
