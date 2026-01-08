// Package golimiter 提供基于信号量和原子操作的 Goroutine 并发数限制器。
// 通过限制同时运行的 Goroutine 数量，防止资源过度消耗。
package golimiter

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	// defaultConcurrencyLimit 默认全局 goroutine 并发限制
	defaultConcurrencyLimit = 100
)

var (
	// globalLimiter 全局 goroutine 并发限制器实例
	globalLimiter *GoroutineLimiter
	// once 确保全局限制器只初始化一次
	once sync.Once
)

// GoroutineLimiter 限制并发的 goroutine 数量，
// 通过带缓冲通道实现信号量，结合原子操作维护当前占用资源数。
type GoroutineLimiter struct {
	sem     chan struct{} // 信号量通道，容量为最大并发数
	max     uint32        // 最大并发数
	current uint32        // 当前已占用资源数，使用原子操作维护
}

// NewGlobalGoroutineLimiter 初始化全局 GoroutineLimiter 实例，
// 只允许初始化一次，后续调用返回第一次创建的实例。
// 如果传入的 max <= 0，则使用默认值 defaultConcurrencyLimit。
func NewGlobalGoroutineLimiter(max uint32) *GoroutineLimiter {
	once.Do(func() {
		globalConcurrencyLimit := max
		if max <= 0 {
			globalConcurrencyLimit = defaultConcurrencyLimit
		}
		globalLimiter = NewGoroutineLimiter(globalConcurrencyLimit)
	})
	return globalLimiter
}

// NewGoroutineLimiter 创建一个新的 GoroutineLimiter，
// max 表示最大并发数，若 max <= 0，则使用默认值 defaultConcurrencyLimit。
func NewGoroutineLimiter(max uint32) *GoroutineLimiter {
	if max <= 0 {
		max = defaultConcurrencyLimit
	}
	return &GoroutineLimiter{
		sem: make(chan struct{}, max),
		max: max,
	}
}

// Acquire 阻塞获取 n 个资源，直到成功或上下文取消。
// 成功获取后，内部计数器增加 n。
// 如果上下文取消，会释放已获取的资源并返回错误。
func (g *GoroutineLimiter) Acquire(ctx context.Context, n int) error {
	for i := 0; i < n; i++ {
		select {
		case g.sem <- struct{}{}:
			atomic.AddUint32(&g.current, 1)
		case <-ctx.Done():
			if i > 0 {
				if err := g.Release(i); err != nil {
					return fmt.Errorf("release failed during acquire rollback: %w", err)
				}
			}
			return ctx.Err()
		}
	}
	return nil
}

// Release 释放 n 个资源，内部计数器减少 n，
// 并从信号量通道中取出 n 个元素。
// 如果释放数量超过当前已占用资源数，返回错误。
func (g *GoroutineLimiter) Release(n int) error {
	for {
		current := atomic.LoadUint32(&g.current)
		if uint32(n) > current {
			return fmt.Errorf("release %d resources but only %d acquired", n, current)
		}
		if atomic.CompareAndSwapUint32(&g.current, current, current-uint32(n)) {
			break
		}
	}
	for i := 0; i < n; i++ {
		<-g.sem
	}
	return nil
}

// Current 返回当前已占用的资源数量。
func (g *GoroutineLimiter) Current() uint32 {
	return atomic.LoadUint32(&g.current)
}

// Max 返回最大并发数。
func (g *GoroutineLimiter) Max() uint32 {
	return g.max
}
