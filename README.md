# pslock: Efficient Distributed Locking with Redis
   
   `pslock` is a Go library that implements a robust and efficient distributed lock mechanism using Redis. It leverages `SETNX` for atomic lock acquisition and enhances it with Redis PubSub for low-latency notifications, backed by a polling mechanism for ultimate resilience.
   
   This hybrid approach ensures safe concurrent operations by:
   
   1. **Fast Path Acquisition**: Attempting lock acquisition via the atomic `SETNX` command.
   2. **Efficient Waiting**: If the lock is held, `pslock` subscribes to a Redis PubSub channel for instant notification upon lock release.
   3. **Resilient Fallback**: Simultaneously, it employs a polling mechanism to re-attempt acquisition, safeguarding against lost PubSub messages (e.g., due to network issues or Redis failovers).
   4. **Graceful Release**: Locks are released cleanly, notifying any waiting instances via PubSub, and internal waiting mechanisms (PubSub subscriptions, pollers) are terminated.
   
   **Key Advantage**: Compared to pure polling-based locks, `pslock` significantly reduces latency and CPU overhead by instantly detecting lock releases through PubSub. It maintains the reliability of polling by using it as a fallback, effectively addressing Redis PubSub's at-most-once delivery guarantee.

   ## Installation
   
   ```bash
   go get github.com/Lizhuotao/pslock
   ```
    ## Usage (Conceptual Example)
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


   ## Features
   
   - **Low Latency**: Near-instant lock acquisition notification via PubSub.
   - **High Resilience**: Polling fallback prevents deadlocks from lost PubSub messages.
   - **Reduced CPU Overhead**: Minimizes busy-waiting typical of pure polling solutions.
   - **Atomic Operations**: Utilizes Redis `SETNX` for safe lock acquisition.
   - **Deadlock Prevention**: Designed to be robust against common distributed system failures.
   - **Configurable Timeouts**: Supports lock expiry and acquisition timeouts.
   
   ## How It Works
   
   The locking mechanism follows these steps:
   
   1. **Initial Acquisition Attempt**:
   
      - The client executes `SET key value NX PX milliseconds` (SET if Not Exists, with an expiry time).
      - If successful, the lock is acquired, and the client can proceed with its critical section. The `value` is typically a unique identifier for the lock holder.
   
   2. **Blocking Flow (If Lock is Already Held)**:
   
      - If the `SETNX` command fails (meaning the lock is already held by another process), the client enters a waiting state:

   
        - **Subscribe to PubSub**: The client subscribes to a specific Redis PubSub channel (e.g., `pslock_channel:<lock_key>`). This channel is used to broadcast lock release notifications.
   
        - **Start Polling Goroutine**: A separate goroutine is initiated to periodically re-attempt lock acquisition using `SETNX`. This acts as a safety net.
   
        - Wait for Notification or Polling Success
   
          : The client waits for one of the following:
   
          - A message on the PubSub channel indicating the lock *might* have been released. Upon receiving a message, it immediately attempts `SETNX` again.
          - The polling goroutine successfully acquires the lock via `SETNX`.
          - A configurable acquisition timeout is reached, or the provided context is cancelled.
   
      - If the lock is successfully acquired (either via PubSub trigger or polling), the PubSub subscription is cancelled, and the polling goroutine is terminated. The client can then proceed.
   
   3. **Unlock Process**:
   
      - To release the lock, the client holding the lock:
   
        - Executes a `DEL key` command on Redis to remove the lock key.
        - Publishes a message to the corresponding PubSub channel (e.g., `pslock_channel:<lock_key>`) to notify any waiting clients that the lock has been released.
   
      - Client-side (for the instance that was waiting and now acquired the lock):
        - Once the lock is acquired (or the attempt is aborted), it unsubscribes from the PubSub channel.
        - It terminates its internal polling goroutine.
   
   ## Why This Hybrid Approach?
   
   1. **Why use `SETNX PX`?**
      - **Atomicity**: `SETNX` is an atomic operation, ensuring that only one client can acquire the lock at any given time.
      - **Lock Expiry (`PX` option)**: Setting an expiry time on the lock is crucial. It prevents indefinite deadlocks if a lock-holding client crashes before releasing the lock.
   2. **Why use PubSub?**
      - **Efficiency & Low Latency**: PubSub allows clients to be notified almost instantly when a lock is released, rather than repeatedly polling Redis. This significantly reduces the delay in acquiring a released lock and minimizes unnecessary network traffic and CPU load on both the client and Redis server.
   3. **Why use Polling alongside PubSub?**
      - **Resilience against Message Loss**: Redis PubSub provides an "at-most-once" delivery guarantee. There's no acknowledgment mechanism, and messages are not persisted. If network fluctuations occur, or if Redis undergoes a master-slave failover while messages are in transit, these notifications can be lost.
      - **Deadlock Prevention**: Without a fallback, lost PubSub messages could lead to clients waiting indefinitely for a notification that will never arrive, resulting in a deadlock. Polling ensures that even if a PubSub message is missed, the client will eventually re-attempt and acquire the lock.



   
   ## Configuration Options (Example)
   
   `pslock` could be configured with options like:
   
   - **Lock Expiry**: Default duration for which a lock is held before auto-expiring (used with `SETNX PX`).
   - **Polling Interval**: Frequency at which the polling mechanism attempts to acquire the lock.
   - **Acquisition Timeout**: Maximum time a client will wait to acquire a lock.
   - **Retry Attempts**: (Optional) Number of retries for certain Redis operations.
   
   ## Limitations & Considerations
   
   - **Redis as a Single Point of Failure**: This implementation relies on a single Redis instance (or a primary in a cluster/sentinel setup). If Redis goes down, the locking mechanism will fail. For high availability, consider Redis Sentinel or Redis Cluster, though this adds complexity to the client's Redis connection management.
   - **Fairness**: This lock is not strictly fair (FIFO). When a lock is released, any waiting client (either notified by PubSub or succeeding in a poll) might acquire it. True fairness often requires more complex server-side scripting (e.g., using Lua with Redis lists).
   - **Redlock Algorithm**: For scenarios requiring higher guarantees against data inconsistency when dealing with multiple independent Redis masters (not a standard master-slave replication), consider the Redlock algorithm and its implications. `pslock` as described is for a single Redis master or a properly managed primary-replica setup.
   
   ## Contributing
   
   Contributions are welcome! Please feel free to submit issues or pull requests.
   
   ## License
   
   MIT License. See `LICENSE` for details.