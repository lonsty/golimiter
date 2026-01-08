package golimiter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestGoroutineLimiter_AcquireRelease(t *testing.T) {
	limit := 3
	limiter := NewGoroutineLimiter(uint32(limit))

	ctx := context.Background()

	// 一次获取 2 个资源
	if err := limiter.Acquire(ctx, 2); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// 再获取 1 个资源，应该成功
	if err := limiter.Acquire(ctx, 1); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// 现在已经获取了 3 个资源，达到上限
	// 再获取 1 个资源应该阻塞，使用带超时的 context 测试
	ctxTimeout, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	err := limiter.Acquire(ctxTimeout, 1)
	if err == nil {
		t.Fatalf("Expected timeout error, got nil")
	}

	// 释放 2 个资源
	limiter.Release(2)

	// 现在可以获取 2 个资源中的 1 个
	if err := limiter.Acquire(ctx, 1); err != nil {
		t.Fatalf("Acquire failed after release: %v", err)
	}

	// 释放剩余资源
	limiter.Release(2)

	// 释放超过已获取数量，应该返回错误
	if err := limiter.Release(1); err == nil {
		t.Fatalf("expected error when releasing more than acquired")
	}
}

func TestNewGlobalGoroutineLimiter(t *testing.T) {
	limiter1 := NewGlobalGoroutineLimiter(5)
	limiter2 := NewGlobalGoroutineLimiter(10)

	if limiter1 != limiter2 {
		t.Fatalf("Expected same global limiter instance")
	}

	if limiter1.max != 5 {
		t.Fatalf("Expected max=5, got %d", limiter1.max)
	}
}

func TestAcquireContextCancel(t *testing.T) {
	limiter := NewGoroutineLimiter(1)
	ctx := context.Background()

	// 占用唯一资源
	if err := limiter.Acquire(ctx, 1); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// 新建一个会立即取消的 context
	ctxCancel, cancel := context.WithCancel(ctx)
	cancel()

	err := limiter.Acquire(ctxCancel, 1)
	if err == nil {
		t.Fatalf("Expected error due to context cancel, got nil")
	}

	limiter.Release(1)
}

func TestReleaseMoreThanAcquire(t *testing.T) {
	limiter := NewGoroutineLimiter(2)
	ctx := context.Background()

	if err := limiter.Acquire(ctx, 1); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// 释放多于 Acquire 的资源，可能会导致阻塞死锁
	// 这里测试不会死锁，但实际使用中应避免
	done := make(chan struct{})
	go func() {
		limiter.Release(2)
		close(done)
	}()

	select {
	case <-done:
		// 释放完成
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("Release blocked due to over-release")
	}
}

func TestGoroutineLimiter_ConcurrentAcquireRelease(t *testing.T) {
	limiter := NewGoroutineLimiter(10)
	ctx := context.Background()

	var wg sync.WaitGroup
	acquireCount := 100
	concurrentLimit := 10

	// 记录最大并发数
	var maxConcurrent int32
	var currentConcurrent int32

	for i := 0; i < acquireCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := limiter.Acquire(ctx, 1); err != nil {
				t.Errorf("Acquire failed: %v", err)
				return
			}
			cur := atomic.AddInt32(&currentConcurrent, 1)
			for {
				max := atomic.LoadInt32(&maxConcurrent)
				if cur > max {
					atomic.CompareAndSwapInt32(&maxConcurrent, max, cur)
				} else {
					break
				}
			}

			// 模拟工作
			time.Sleep(10 * time.Millisecond)

			atomic.AddInt32(&currentConcurrent, -1)
			if err := limiter.Release(1); err != nil {
				t.Errorf("Release failed: %v", err)
			}
		}()
	}

	wg.Wait()

	if maxConcurrent > int32(concurrentLimit) {
		t.Errorf("max concurrent %d exceeded limit %d", maxConcurrent, concurrentLimit)
	}
}

func TestGoroutineLimiter_AcquireContextCancel(t *testing.T) {
	limiter := NewGoroutineLimiter(2)

	ctx, cancel := context.WithCancel(context.Background())

	// 先占满资源
	if err := limiter.Acquire(ctx, 2); err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	doneCh := make(chan error)
	go func() {
		// 这里会阻塞等待资源
		err := limiter.Acquire(ctx, 1)
		doneCh <- err
	}()

	// 取消上下文，解除阻塞
	cancel()

	err := <-doneCh
	if err == nil {
		t.Fatalf("expected error due to context cancel")
	}

	// 释放资源
	if err := limiter.Release(2); err != nil {
		t.Fatalf("Release failed: %v", err)
	}
}
