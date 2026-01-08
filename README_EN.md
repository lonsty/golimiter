# golimiter

[中文](./README.md) | English

[![Go Reference](https://pkg.go.dev/badge/github.com/lonsty/golimiter.svg)](https://pkg.go.dev/github.com/lonsty/golimiter)
[![Go Report Card](https://goreportcard.com/badge/github.com/lonsty/golimiter)](https://goreportcard.com/report/github.com/lonsty/golimiter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://github.com/lonsty/golimiter/actions/workflows/go.yml/badge.svg)](https://github.com/lonsty/golimiter/actions/workflows/go.yml)

`golimiter` provides a semaphore-based Goroutine concurrency limiter with atomic operations, preventing resource exhaustion by limiting the number of concurrent goroutines.

## Features

- **GoroutineLimiter**: Low-level concurrency limiter providing basic resource acquisition and release capabilities
- **WorkerPool**: High-level worker pool encapsulating concurrency control and task waiting, ready to use out of the box
- Context cancellation support
- Thread-safe
- Zero dependencies (standard library only)

## Installation

```bash
go get github.com/lonsty/golimiter
```

## Quick Start

### Using WorkerPool (Recommended)

`WorkerPool` is a higher-level abstraction suitable for most scenarios:

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/lonsty/golimiter"
)

func main() {
    // Create a worker pool with max concurrency of 10
    pool := golimiter.NewWorkerPool(10)
    ctx := context.Background()
    
    // Submit tasks
    for i := 0; i < 100; i++ {
        taskID := i
        err := pool.Submit(ctx, func() {
            fmt.Printf("Task %d is running\n", taskID)
            time.Sleep(100 * time.Millisecond)
        })
        if err != nil {
            fmt.Printf("Failed to submit task %d: %v\n", taskID, err)
        }
    }
    
    // Wait for all tasks to complete
    pool.Wait()
    fmt.Println("All tasks completed")
}
```

### Using GoroutineLimiter (Low-level Control)

For more fine-grained control, use `GoroutineLimiter` directly:

```go
package main

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/lonsty/golimiter"
)

func main() {
    // Create a limiter with max concurrency of 10
    limiter := golimiter.NewGoroutineLimiter(10)
    ctx := context.Background()
    var wg sync.WaitGroup
    
    for i := 0; i < 100; i++ {
        // Acquire resource
        if err := limiter.Acquire(ctx, 1); err != nil {
            fmt.Printf("Failed to acquire: %v\n", err)
            continue
        }
        
        wg.Add(1)
        go func(taskID int) {
            defer wg.Done()
            defer limiter.Release(1)
            
            fmt.Printf("Task %d is running\n", taskID)
            time.Sleep(100 * time.Millisecond)
        }(i)
    }
    
    wg.Wait()
    fmt.Println("All tasks completed")
}
```

## Use Cases

### 1. Batch Task Processing

```go
func ProcessTasks(tasks []Task) error {
    pool := golimiter.NewWorkerPool(20)
    ctx := context.Background()
    
    for _, task := range tasks {
        t := task
        if err := pool.Submit(ctx, func() {
            t.Process()
        }); err != nil {
            return err
        }
    }
    
    pool.Wait()
    return nil
}
```

### 2. Tasks with Error Handling

```go
func ProcessTasksWithError(tasks []Task) []error {
    pool := golimiter.NewWorkerPool(20)
    ctx := context.Background()
    errChan := make(chan error, len(tasks))
    
    for _, task := range tasks {
        t := task
        if err := pool.SubmitWithError(ctx, func() error {
            return t.Process()
        }, errChan); err != nil {
            return []error{err}
        }
    }
    
    pool.Wait()
    close(errChan)
    
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    return errors
}
```

### 3. Context Cancellation Support

```go
func ProcessWithTimeout(tasks []Task, timeout time.Duration) error {
    pool := golimiter.NewWorkerPool(20)
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    for _, task := range tasks {
        t := task
        if err := pool.Submit(ctx, func() {
            t.Process()
        }); err != nil {
            return fmt.Errorf("submit failed: %w", err)
        }
    }
    
    pool.Wait()
    return nil
}
```

### 4. Batch Processing

```go
func ProcessInBatches(batches [][]Task) error {
    pool := golimiter.NewWorkerPool(20)
    ctx := context.Background()
    
    for _, batch := range batches {
        // Process current batch
        for _, task := range batch {
            t := task
            if err := pool.Submit(ctx, func() {
                t.Process()
            }); err != nil {
                return err
            }
        }
        
        // Wait for current batch to complete
        pool.Wait()
        fmt.Println("Batch completed")
    }
    
    return nil
}
```

### 5. Global Limiter

```go
// Initialize global limiter at application startup
func init() {
    golimiter.NewGlobalGoroutineLimiter(100)
}

// Use global limiter anywhere
func ProcessGlobally(task Task) error {
    limiter := golimiter.NewGlobalGoroutineLimiter(0) // Returns initialized instance
    ctx := context.Background()
    
    if err := limiter.Acquire(ctx, 1); err != nil {
        return err
    }
    defer limiter.Release(1)
    
    return task.Process()
}
```

## API Documentation

### WorkerPool

#### Creating a Worker Pool

```go
// NewWorkerPool creates a new worker pool
// maxWorkers: maximum number of concurrent worker goroutines, uses default value 100 if <= 0
func NewWorkerPool(maxWorkers uint32) *WorkerPool
```

#### Submitting Tasks

```go
// Submit submits a task to the worker pool for execution
// This method blocks until a resource is acquired or the context is cancelled
func (p *WorkerPool) Submit(ctx context.Context, fn func()) error

// SubmitWithError submits a task that may return an error
// Errors are returned through errChan, caller must read from errChan
func (p *WorkerPool) SubmitWithError(ctx context.Context, fn func() error, errChan chan<- error) error
```

#### Waiting and Querying

```go
// Wait waits for all submitted tasks to complete
func (p *WorkerPool) Wait()

// Current returns the number of currently executing tasks
func (p *WorkerPool) Current() uint32

// Max returns the maximum concurrency limit
func (p *WorkerPool) Max() uint32
```

### GoroutineLimiter

#### Creating a Limiter

```go
// NewGoroutineLimiter creates a new limiter
// max: maximum concurrency, uses default value 100 if <= 0
func NewGoroutineLimiter(max uint32) *GoroutineLimiter

// NewGlobalGoroutineLimiter creates or gets the global limiter (singleton)
func NewGlobalGoroutineLimiter(max uint32) *GoroutineLimiter
```

#### Resource Management

```go
// Acquire blocks to acquire n resources
// Internal counter increases by n after successful acquisition
// If context is cancelled, releases acquired resources and returns error
func (g *GoroutineLimiter) Acquire(ctx context.Context, n int) error

// Release releases n resources
// Returns error if release count exceeds currently acquired resources
func (g *GoroutineLimiter) Release(n int) error
```

## Design Principles

### Single Responsibility

- **GoroutineLimiter**: Focuses on concurrency control (semaphore)
- **WorkerPool**: Focuses on task management and waiting (worker pool)
- Functionality extension through composition, not inheritance

### Why Not Add Wait Method to GoroutineLimiter?

1. **Separation of Concerns**: `GoroutineLimiter` is a semaphore responsible only for resource limiting; `WaitGroup` is a synchronization primitive responsible for waiting for task completion
2. **Clear Semantics**: `Wait` waits for "task completion", but the limiter doesn't know what constitutes "a batch of tasks"
3. **Flexibility**: Different scenarios have different "waiting" requirements; composition provides more flexibility
4. **Go Philosophy**: Build complex functionality through composition of small, single-responsibility components

### Why No GlobalWorkerPool?

`WorkerPool` doesn't provide a global instance because it's a stateful task manager that internally maintains `sync.WaitGroup` to wait for a batch of tasks to complete. A global instance would cause state accumulation, and different scenarios need different concurrency configurations.

**If global concurrency control is needed**, share `GoroutineLimiter`, not `WorkerPool`:

```go
// ✅ Recommended: Share global limiter, create local worker pools
var globalLimiter = golimiter.NewGlobalGoroutineLimiter(100)

func ProcessUsers(users []*User) error {
    // Create local worker pool using global limiter
    pool := &golimiter.WorkerPool{
        limiter: globalLimiter, // Share global limiter
    }
    
    for _, user := range users {
        pool.Submit(ctx, func() { processUser(user) })
    }
    
    pool.Wait() // Only wait for this batch of user tasks
    return nil
}
```

## Performance Considerations

- Uses buffered channels for semaphore implementation, better performance than mutex locks
- Uses atomic operations for counter maintenance, avoiding lock contention
- Zero memory allocation (except for goroutine creation)

## Testing

Run all tests:

```bash
go test -v ./...
```

Run benchmarks:

```bash
go test -bench=. -benchmem
```

## Best Practices

1. **Prefer WorkerPool**: Use `WorkerPool` unless fine-grained control is needed
2. **Set Reasonable Concurrency**: Based on task type (CPU-intensive or IO-intensive) and system resources
3. **Handle Context Cancellation**: Always check return errors from `Submit` or `Acquire`
4. **Error Handling**: Remember to read from `errChan` when using `SubmitWithError` to avoid goroutine leaks
5. **Resource Release**: Use `defer` to ensure proper resource release

## Notes

- `WorkerPool` can be reused; calling `Wait` multiple times is safe
- `GoroutineLimiter`'s `Release` must be paired with `Acquire`
- Global limiter can only be initialized once; subsequent calls return the same instance
- Don't use `defer` in loops; it causes delayed resource release

## License

[MIT](LICENSE)

## Contributing

Issues and Pull Requests are welcome!
