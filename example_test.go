package golimiter_test

import (
	"context"
	"fmt"
	"time"

	"github.com/lonsty/golimiter"
)

// ExampleWorkerPool_basic 演示 WorkerPool 的基本使用
func ExampleWorkerPool_basic() {
	// 创建一个最大并发数为 3 的工作池
	pool := golimiter.NewWorkerPool(3)
	ctx := context.Background()

	// 提交 5 个任务
	for i := 1; i <= 5; i++ {
		taskID := i
		err := pool.Submit(ctx, func() {
			fmt.Printf("Task %d started\n", taskID)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Task %d completed\n", taskID)
		})
		if err != nil {
			fmt.Printf("Failed to submit task %d: %v\n", taskID, err)
		}
	}

	// 等待所有任务完成
	pool.Wait()
	fmt.Println("All tasks finished")
}

// ExampleWorkerPool_withError 演示带错误处理的任务提交
func ExampleWorkerPool_withError() {
	pool := golimiter.NewWorkerPool(5)
	ctx := context.Background()

	// 创建错误通道
	errChan := make(chan error, 10)

	// 提交任务
	for i := 1; i <= 10; i++ {
		taskID := i
		err := pool.SubmitWithError(ctx, func() error {
			if taskID%3 == 0 {
				return fmt.Errorf("task %d failed", taskID)
			}
			fmt.Printf("Task %d succeeded\n", taskID)
			return nil
		}, errChan)
		if err != nil {
			fmt.Printf("Failed to submit task %d: %v\n", taskID, err)
		}
	}

	// 等待所有任务完成
	pool.Wait()
	close(errChan)

	// 收集错误
	for err := range errChan {
		fmt.Printf("Error: %v\n", err)
	}
}

// ExampleWorkerPool_withTimeout 演示带超时的任务处理
func ExampleWorkerPool_withTimeout() {
	pool := golimiter.NewWorkerPool(5)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// 提交任务
	for i := 1; i <= 10; i++ {
		taskID := i
		err := pool.Submit(ctx, func() {
			fmt.Printf("Task %d running\n", taskID)
			time.Sleep(50 * time.Millisecond)
		})
		if err != nil {
			fmt.Printf("Task %d submission failed: %v\n", taskID, err)
			break
		}
	}

	pool.Wait()
	fmt.Println("Done")
}

// ExampleGoroutineLimiter_basic 演示 GoroutineLimiter 的基本使用
func ExampleGoroutineLimiter_basic() {
	// 创建一个最大并发数为 3 的限制器
	limiter := golimiter.NewGoroutineLimiter(3)
	ctx := context.Background()

	// 手动管理 goroutine
	done := make(chan bool)

	for i := 1; i <= 5; i++ {
		// 获取资源
		if err := limiter.Acquire(ctx, 1); err != nil {
			fmt.Printf("Failed to acquire: %v\n", err)
			continue
		}

		go func(taskID int) {
			defer limiter.Release(1)
			fmt.Printf("Task %d is running\n", taskID)
			time.Sleep(100 * time.Millisecond)

			if taskID == 5 {
				close(done)
			}
		}(i)
	}

	<-done
	fmt.Println("All tasks completed")
}

// ExampleNewGlobalGoroutineLimiter 演示全局限制器的使用
func ExampleNewGlobalGoroutineLimiter() {
	// 注意：全局限制器只会初始化一次
	// 在实际应用中，应该在 init() 或 main() 函数开始时初始化
	limiter := golimiter.NewGlobalGoroutineLimiter(10)
	fmt.Printf("Global limiter initialized\n")

	// 再次调用返回同一实例，参数会被忽略
	sameLimiter := golimiter.NewGlobalGoroutineLimiter(20)

	// 验证是同一个实例
	if limiter == sameLimiter {
		fmt.Printf("Same instance confirmed\n")
	}

	// Output:
	// Global limiter initialized
	// Same instance confirmed
}
