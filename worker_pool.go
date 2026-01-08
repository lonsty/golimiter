package golimiter

import (
	"context"
	"sync"
)

// WorkerPool 工作池，封装了 GoroutineLimiter 和 sync.WaitGroup，
// 提供更高层次的并发任务管理能力。
// WorkerPool 负责限制并发数量并等待所有任务完成。
type WorkerPool struct {
	limiter *GoroutineLimiter // 并发限制器
	wg      sync.WaitGroup    // 等待所有任务完成
}

// NewWorkerPool 创建一个新的 WorkerPool，
// maxWorkers 表示最大并发工作协程数，若 maxWorkers <= 0，则使用默认值。
func NewWorkerPool(maxWorkers uint32) *WorkerPool {
	return &WorkerPool{
		limiter: NewGoroutineLimiter(maxWorkers),
	}
}

// Submit 提交一个任务到工作池执行。
// 该方法会阻塞直到获取到执行资源或上下文取消。
// 任务将在新的 goroutine 中异步执行。
func (p *WorkerPool) Submit(ctx context.Context, fn func()) error {
	if err := p.limiter.Acquire(ctx, 1); err != nil {
		return err
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.limiter.Release(1)
		fn()
	}()
	return nil
}

// SubmitWithError 提交一个可能返回错误的任务到工作池执行。
// 该方法会阻塞直到获取到执行资源或上下文取消。
// 任务将在新的 goroutine 中异步执行，错误通过 errChan 返回。
// 注意：调用者需要从 errChan 中读取错误，否则可能导致 goroutine 泄漏。
func (p *WorkerPool) SubmitWithError(ctx context.Context, fn func() error, errChan chan<- error) error {
	if err := p.limiter.Acquire(ctx, 1); err != nil {
		return err
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.limiter.Release(1)
		if err := fn(); err != nil && errChan != nil {
			errChan <- err
		}
	}()
	return nil
}

// Wait 等待所有已提交的任务完成。
// 该方法会阻塞直到所有通过 Submit 或 SubmitWithError 提交的任务都执行完毕。
func (p *WorkerPool) Wait() {
	p.wg.Wait()
}

// Current 返回当前正在执行的任务数量。
func (p *WorkerPool) Current() uint32 {
	return p.limiter.Current()
}

// Max 返回最大并发数。
func (p *WorkerPool) Max() uint32 {
	return p.limiter.Max()
}
