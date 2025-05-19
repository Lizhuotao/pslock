# pslock

This project implements a distributed lock mechanism using Redis PubSub and polling. It ensures safe concurrent operations by:

1. Attempting lock acquisition via `SETNX`.
2. Falling back to a hybrid approach (PubSub + polling) if blocked, preventing deadlocks from message loss.
3. Gracefully releasing locks via unsubscribe and polling termination.

‌**Key Advantage**‌: Compared to pure polling-based locks, it instantly detects lock releases through PubSub notifications, reducing latency and CPU overhead while maintaining reliability.

The design addresses Redis PubSub's at-most-once limitation, making it resilient to network issues or failovers.

‌**Example Usage**‌:
```go
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
```

## Lock Acquisition Process

1. Execute the Redis `SETNX` command.
2. If the distributed lock is successfully acquired, proceed. If acquisition fails, enter the blocking flow.

## Blocking Flow

1. Use Redis's `SETNX` command to acquire the lock. If successful, proceed with operations.
2. If acquisition fails, start a goroutine to poll for lock acquisition.
3. Simultaneously, subscribe to a Redis channel to wait for lock release notifications. If a notification is received, attempt to acquire the lock again.
4. If the lock is successfully acquired, terminate the polling operation and proceed with data requests and cache updates.

## Unlock Process

1. Unsubscribe from the channel.
2. Terminate the polling.

## Why?

1. ‌**Why use polling alongside PubSub?**‌
   ‌**Answer:**‌ Redis PubSub follows an at-most-once delivery model without ACK mechanisms or message persistence. If network fluctuations occur or Redis undergoes a master-slave failover, messages might be lost, potentially leading to deadlocks. Polling adds redundancy to prevent this scenario.