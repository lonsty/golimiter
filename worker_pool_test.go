package golimiter

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWorkerPool_Basic 测试 WorkerPool 基本功能
func TestWorkerPool_Basic(t *testing.T) {
	pool := NewWorkerPool(5)

	var counter int32
	ctx := context.Background()

	// 提交 10 个任务
	for i := 0; i < 10; i++ {
		err := pool.Submit(ctx, func() {
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	// 等待所有任务完成
	pool.Wait()

	// 验证所有任务都执行了
	if counter != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}

	// 验证所有资源都已释放
	if pool.Current() != 0 {
		t.Errorf("Expected current to be 0, got %d", pool.Current())
	}
}

// TestWorkerPool_ConcurrencyLimit 测试并发数限制
func TestWorkerPool_ConcurrencyLimit(t *testing.T) {
	maxWorkers := uint32(3)
	pool := NewWorkerPool(maxWorkers)

	var maxConcurrent uint32
	var currentConcurrent uint32
	var mu sync.Mutex

	ctx := context.Background()

	// 提交 10 个任务，每个任务执行 50ms
	for i := 0; i < 10; i++ {
		err := pool.Submit(ctx, func() {
			current := atomic.AddUint32(&currentConcurrent, 1)

			mu.Lock()
			if current > maxConcurrent {
				maxConcurrent = current
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)
			atomic.AddUint32(&currentConcurrent, ^uint32(0)) // -1
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	pool.Wait()

	// 验证最大并发数不超过限制
	if maxConcurrent > maxWorkers {
		t.Errorf("Expected max concurrent to be <= %d, got %d", maxWorkers, maxConcurrent)
	}

	// 验证确实达到了最大并发数（说明限制器在工作）
	if maxConcurrent < maxWorkers {
		t.Errorf("Expected max concurrent to reach %d, got %d", maxWorkers, maxConcurrent)
	}
}

// TestWorkerPool_ContextCancellation 测试上下文取消
func TestWorkerPool_ContextCancellation(t *testing.T) {
	pool := NewWorkerPool(2)

	ctx, cancel := context.WithCancel(context.Background())

	// 提交 2 个长时间运行的任务，占满工作池
	for i := 0; i < 2; i++ {
		err := pool.Submit(ctx, func() {
			time.Sleep(200 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	// 等待一下确保任务开始执行
	time.Sleep(10 * time.Millisecond)

	// 取消上下文
	cancel()

	// 尝试提交新任务，应该立即失败
	err := pool.Submit(ctx, func() {
		t.Error("This task should not run")
	})

	if err == nil {
		t.Error("Expected error when submitting with cancelled context")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	// 等待已提交的任务完成
	pool.Wait()
}

// TestWorkerPool_SubmitWithError 测试带错误返回的任务提交
func TestWorkerPool_SubmitWithError(t *testing.T) {
	pool := NewWorkerPool(5)
	ctx := context.Background()

	errChan := make(chan error, 10)
	expectedErr := errors.New("task error")

	var successCount int32
	var errorCount int32

	// 提交 10 个任务，其中一半会返回错误
	for i := 0; i < 10; i++ {
		taskID := i
		err := pool.SubmitWithError(ctx, func() error {
			time.Sleep(10 * time.Millisecond)
			if taskID%2 == 0 {
				atomic.AddInt32(&successCount, 1)
				return nil
			}
			atomic.AddInt32(&errorCount, 1)
			return expectedErr
		}, errChan)

		if err != nil {
			t.Fatalf("SubmitWithError failed: %v", err)
		}
	}

	// 等待所有任务完成
	pool.Wait()
	close(errChan)

	// 验证成功和失败的任务数
	if successCount != 5 {
		t.Errorf("Expected 5 successful tasks, got %d", successCount)
	}
	if errorCount != 5 {
		t.Errorf("Expected 5 failed tasks, got %d", errorCount)
	}

	// 验证错误通道中的错误数量
	var receivedErrors int
	for err := range errChan {
		if !errors.Is(err, expectedErr) {
			t.Errorf("Expected error to be %v, got %v", expectedErr, err)
		}
		receivedErrors++
	}

	if receivedErrors != 5 {
		t.Errorf("Expected 5 errors in channel, got %d", receivedErrors)
	}
}

// TestWorkerPool_MultipleWaits 测试多次调用 Wait
func TestWorkerPool_MultipleWaits(t *testing.T) {
	pool := NewWorkerPool(3)
	ctx := context.Background()

	var counter int32

	// 第一批任务
	for i := 0; i < 5; i++ {
		err := pool.Submit(ctx, func() {
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	pool.Wait()

	if counter != 5 {
		t.Errorf("Expected counter to be 5 after first batch, got %d", counter)
	}

	// 第二批任务
	for i := 0; i < 5; i++ {
		err := pool.Submit(ctx, func() {
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	pool.Wait()

	if counter != 10 {
		t.Errorf("Expected counter to be 10 after second batch, got %d", counter)
	}
}

// TestWorkerPool_ZeroWorkers 测试使用默认值
func TestWorkerPool_ZeroWorkers(t *testing.T) {
	pool := NewWorkerPool(0)

	// 应该使用默认值
	if pool.Max() != defaultConcurrencyLimit {
		t.Errorf("Expected max to be %d, got %d", defaultConcurrencyLimit, pool.Max())
	}

	ctx := context.Background()
	var counter int32

	// 提交一些任务验证工作正常
	for i := 0; i < 10; i++ {
		err := pool.Submit(ctx, func() {
			atomic.AddInt32(&counter, 1)
		})
		if err != nil {
			t.Fatalf("Submit failed: %v", err)
		}
	}

	pool.Wait()

	if counter != 10 {
		t.Errorf("Expected counter to be 10, got %d", counter)
	}
}

// TestWorkerPool_CurrentAndMax 测试 Current 和 Max 方法
func TestWorkerPool_CurrentAndMax(t *testing.T) {
	maxWorkers := uint32(5)
	pool := NewWorkerPool(maxWorkers)

	if pool.Max() != maxWorkers {
		t.Errorf("Expected max to be %d, got %d", maxWorkers, pool.Max())
	}

	if pool.Current() != 0 {
		t.Errorf("Expected current to be 0, got %d", pool.Current())
	}

	ctx := context.Background()
	started := make(chan struct{})

	// 提交一个长时间运行的任务
	err := pool.Submit(ctx, func() {
		close(started)
		time.Sleep(100 * time.Millisecond)
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// 等待任务开始
	<-started

	// 验证当前有 1 个任务在执行
	if pool.Current() != 1 {
		t.Errorf("Expected current to be 1, got %d", pool.Current())
	}

	pool.Wait()

	// 验证任务完成后 current 为 0
	if pool.Current() != 0 {
		t.Errorf("Expected current to be 0 after wait, got %d", pool.Current())
	}
}

// BenchmarkWorkerPool_Submit 基准测试 Submit 性能
func BenchmarkWorkerPool_Submit(b *testing.B) {
	pool := NewWorkerPool(100)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pool.Submit(ctx, func() {
			// 模拟一些工作
			time.Sleep(time.Microsecond)
		})
		if err != nil {
			b.Fatalf("Submit failed: %v", err)
		}
	}
	pool.Wait()
}

// BenchmarkWorkerPool_SubmitWithError 基准测试 SubmitWithError 性能
func BenchmarkWorkerPool_SubmitWithError(b *testing.B) {
	pool := NewWorkerPool(100)
	ctx := context.Background()
	errChan := make(chan error, b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := pool.SubmitWithError(ctx, func() error {
			// 模拟一些工作
			time.Sleep(time.Microsecond)
			return nil
		}, errChan)
		if err != nil {
			b.Fatalf("SubmitWithError failed: %v", err)
		}
	}
	pool.Wait()
	close(errChan)
}
