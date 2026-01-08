# golimiter

中文 | [English](./README_EN.md)

[![Go Reference](https://pkg.go.dev/badge/github.com/lonsty/golimiter.svg)](https://pkg.go.dev/github.com/lonsty/golimiter)
[![Go Report Card](https://goreportcard.com/badge/github.com/lonsty/golimiter)](https://goreportcard.com/report/github.com/lonsty/golimiter)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go](https://github.com/lonsty/golimiter/actions/workflows/go.yml/badge.svg)](https://github.com/lonsty/golimiter/actions/workflows/go.yml)

`golimiter` 提供基于信号量和原子操作的 Goroutine 并发数限制器，通过限制同时运行的 Goroutine 数量，防止资源过度消耗。

## 功能特性

- **GoroutineLimiter**：底层并发限制器，提供资源获取和释放的基本能力
- **WorkerPool**：高层工作池，封装了并发限制和任务等待功能，开箱即用
- 支持上下文取消
- 线程安全
- 零依赖（仅使用标准库）

## 安装

```bash
go get github.com/lonsty/golimiter
```

## 快速开始

### 使用 WorkerPool（推荐）

`WorkerPool` 是更高层次的抽象，适合大多数场景：

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "github.com/lonsty/golimiter"
)

func main() {
    // 创建一个最大并发数为 10 的工作池
    pool := golimiter.NewWorkerPool(10)
    ctx := context.Background()
    
    // 提交任务
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
    
    // 等待所有任务完成
    pool.Wait()
    fmt.Println("All tasks completed")
}
```

### 使用 GoroutineLimiter（底层控制）

如果需要更精细的控制，可以直接使用 `GoroutineLimiter`：

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
    // 创建一个最大并发数为 10 的限制器
    limiter := golimiter.NewGoroutineLimiter(10)
    ctx := context.Background()
    var wg sync.WaitGroup
    
    for i := 0; i < 100; i++ {
        // 获取资源
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

## 使用场景

### 1. 批量处理任务

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

### 2. 带错误处理的任务

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

### 3. 支持上下文取消

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

### 4. 分批处理

```go
func ProcessInBatches(batches [][]Task) error {
    pool := golimiter.NewWorkerPool(20)
    ctx := context.Background()
    
    for _, batch := range batches {
        // 处理当前批次
        for _, task := range batch {
            t := task
            if err := pool.Submit(ctx, func() {
                t.Process()
            }); err != nil {
                return err
            }
        }
        
        // 等待当前批次完成
        pool.Wait()
        fmt.Println("Batch completed")
    }
    
    return nil
}
```

### 5. 全局限制器

```go
// 在应用启动时初始化全局限制器
func init() {
    golimiter.NewGlobalGoroutineLimiter(100)
}

// 在任何地方使用全局限制器
func ProcessGlobally(task Task) error {
    limiter := golimiter.NewGlobalGoroutineLimiter(0) // 返回已初始化的实例
    ctx := context.Background()
    
    if err := limiter.Acquire(ctx, 1); err != nil {
        return err
    }
    defer limiter.Release(1)
    
    return task.Process()
}
```

## API 文档

### WorkerPool

#### 创建工作池

```go
// NewWorkerPool 创建一个新的工作池
// maxWorkers: 最大并发工作协程数，若 <= 0 则使用默认值 100
func NewWorkerPool(maxWorkers uint32) *WorkerPool
```

#### 提交任务

```go
// Submit 提交一个任务到工作池执行
// 该方法会阻塞直到获取到执行资源或上下文取消
func (p *WorkerPool) Submit(ctx context.Context, fn func()) error

// SubmitWithError 提交一个可能返回错误的任务
// 错误通过 errChan 返回，调用者需要从 errChan 中读取错误
func (p *WorkerPool) SubmitWithError(ctx context.Context, fn func() error, errChan chan<- error) error
```

#### 等待和查询

```go
// Wait 等待所有已提交的任务完成
func (p *WorkerPool) Wait()

// Current 返回当前正在执行的任务数量
func (p *WorkerPool) Current() uint32

// Max 返回最大并发数
func (p *WorkerPool) Max() uint32
```

### GoroutineLimiter

#### 创建限制器

```go
// NewGoroutineLimiter 创建一个新的限制器
// max: 最大并发数，若 <= 0 则使用默认值 100
func NewGoroutineLimiter(max uint32) *GoroutineLimiter

// NewGlobalGoroutineLimiter 创建或获取全局限制器（单例）
func NewGlobalGoroutineLimiter(max uint32) *GoroutineLimiter
```

#### 资源管理

```go
// Acquire 阻塞获取 n 个资源
// 成功获取后内部计数器增加 n
// 如果上下文取消，会释放已获取的资源并返回错误
func (g *GoroutineLimiter) Acquire(ctx context.Context, n int) error

// Release 释放 n 个资源
// 如果释放数量超过当前已占用资源数，返回错误
func (g *GoroutineLimiter) Release(n int) error
```

## 设计原则

### 单一职责

- **GoroutineLimiter**：专注于并发数量控制（信号量）
- **WorkerPool**：专注于任务管理和等待（工作池）
- 通过组合而非继承实现功能扩展

### 为什么不在 GoroutineLimiter 中加入 Wait 方法？

1. **职责分离**：`GoroutineLimiter` 是信号量，只负责资源限制；`WaitGroup` 是同步原语，负责等待任务完成
2. **语义清晰**：`Wait` 等待的是"任务完成"，而 limiter 不知道什么是"一批任务"
3. **灵活性**：不同场景对"等待"的需求不同，组合使用更灵活
4. **符合 Go 哲学**：通过组合小的、职责单一的组件构建复杂功能

### 为什么没有 GlobalWorkerPool？

`WorkerPool` 不提供全局实例，因为它是有状态的任务管理器，内部维护 `sync.WaitGroup` 用于等待一批任务完成。全局实例会导致状态累积，且不同场景需要不同的并发配置。

如果需要全局并发控制，应该共享 `GoroutineLimiter`，而不是 `WorkerPool`：

```go
// ✅ 推荐：共享全局限制器，局部创建工作池
var globalLimiter = golimiter.NewGlobalGoroutineLimiter(100)

func ProcessUsers(users []*User) error {
    // 使用全局限制器创建局部工作池
    pool := &golimiter.WorkerPool{
        limiter: globalLimiter, // 共享全局限制器
    }
    
    for _, user := range users {
        pool.Submit(ctx, func() { processUser(user) })
    }
    
    pool.Wait() // 只等待这批用户的任务
    return nil
}
```

## 性能考虑

- 使用带缓冲通道实现信号量，性能优于互斥锁
- 使用原子操作维护计数器，避免锁竞争
- 零内存分配（除了创建 goroutine）

## 测试

运行所有测试：

```bash
go test -v ./...
```

运行基准测试：

```bash
go test -bench=. -benchmem
```

## 最佳实践

1. **优先使用 WorkerPool**：除非需要精细控制，否则使用 `WorkerPool` 更简单
2. **合理设置并发数**：根据任务类型（CPU 密集型或 IO 密集型）和系统资源设置
3. **处理上下文取消**：始终检查 `Submit` 或 `Acquire` 的返回错误
4. **错误处理**：使用 `SubmitWithError` 时记得读取 `errChan`，避免 goroutine 泄漏
5. **资源释放**：使用 `defer` 确保资源正确释放

## 注意事项

- `WorkerPool` 可以重复使用，多次调用 `Wait` 是安全的
- `GoroutineLimiter` 的 `Release` 必须与 `Acquire` 配对使用
- 全局限制器只能初始化一次，后续调用返回同一实例
- 不要在循环中使用 `defer`，会导致资源延迟释放

## 许可证

[MIT](LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！
