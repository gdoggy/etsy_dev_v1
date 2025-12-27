package database

import (
	"context"
	"log"
	"sync"
	"time"
)

// PartitionTask 分区维护任务
type PartitionTask struct {
	manager      *PartitionManager
	futureMonths int
	interval     time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	running      bool
	mu           sync.Mutex
}

// NewPartitionTask 创建分区维护任务
func NewPartitionTask(manager *PartitionManager, opts ...PartitionTaskOption) *PartitionTask {
	t := &PartitionTask{
		manager:      manager,
		futureMonths: 3,
		interval:     24 * time.Hour,
		stopCh:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// PartitionTaskOption 任务选项
type PartitionTaskOption func(*PartitionTask)

// WithFutureMonths 设置未来分区月数
func WithFutureMonths(months int) PartitionTaskOption {
	return func(t *PartitionTask) {
		t.futureMonths = months
	}
}

// WithInterval 设置执行间隔
func WithInterval(d time.Duration) PartitionTaskOption {
	return func(t *PartitionTask) {
		t.interval = d
	}
}

// Start 启动任务
func (t *PartitionTask) Start() {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	t.mu.Unlock()

	t.wg.Add(1)
	go t.run()

	log.Printf("[PartitionTask] 已启动，间隔: %v, 未来分区: %d 月", t.interval, t.futureMonths)
}

// Stop 停止任务
func (t *PartitionTask) Stop() {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return
	}
	t.running = false
	t.mu.Unlock()

	close(t.stopCh)
	t.wg.Wait()
	log.Println("[PartitionTask] 已停止")
}

func (t *PartitionTask) run() {
	defer t.wg.Done()

	// 启动时立即执行
	t.execute()

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			t.execute()
		case <-t.stopCh:
			return
		}
	}
}

func (t *PartitionTask) execute() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("[PartitionTask] 开始执行...")
	start := time.Now()

	// 1. 健康检查
	if err := t.manager.HealthCheck(ctx); err != nil {
		log.Printf("[PartitionTask] 健康检查: %v", err)
	}

	// 2. 创建未来分区
	if err := t.manager.EnsureFuturePartitions(ctx, t.futureMonths); err != nil {
		log.Printf("[PartitionTask] 创建分区失败: %v", err)
	}

	// 3. 清理过期分区
	dropped, err := t.manager.CleanupExpiredPartitions(ctx)
	if err != nil {
		log.Printf("[PartitionTask] 清理过期分区失败: %v", err)
	} else if dropped > 0 {
		log.Printf("[PartitionTask] 已删除 %d 个过期分区", dropped)
	}

	// 4. 打印统计
	t.printStats(ctx)

	log.Printf("[PartitionTask] 执行完成，耗时: %v", time.Since(start))
}

func (t *PartitionTask) printStats(ctx context.Context) {
	stats, err := t.manager.GetAllStats(ctx)
	if err != nil {
		return
	}
	for _, s := range stats {
		log.Printf("[PartitionTask] %s: %d 分区, %.2f MB",
			s.TableName, s.PartitionCount, float64(s.TotalSizeBytes)/1024/1024)
	}
}

// RunOnce 手动执行一次
func (t *PartitionTask) RunOnce() {
	t.execute()
}
