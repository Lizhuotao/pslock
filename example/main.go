package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lizhuotao/pslock"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	r := pslock.New(client)

	name := "test-lock"
	// Create distributed lock instance
	lock := r.NewMutex(name)

	// Create context
	ctx := context.Background()

	// Try to acquire lock
	err := lock.Lock(ctx)
	if err != nil {
		log.Fatalf("Failed to acquire lock: %v", err)
	}

	// Do some work while holding the lock
	fmt.Println("Lock acquired, doing some work...")
	time.Sleep(2 * time.Second)

	// Release the lock
	err = lock.Unlock(ctx)
	if err != nil {
		log.Fatalf("Failed to release lock: %v", err)
	}

	fmt.Println("Lock released successfully")
}
