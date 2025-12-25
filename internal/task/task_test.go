package task

import (
	"context"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ==================== Task 测试模型 ====================

type TestTaskProxy struct {
	ID           int64 `gorm:"primaryKey"`
	Host         string
	Port         int
	Status       int
	LastCheckAt  *time.Time
	ResponseTime int
	FailCount    int
}

func (TestTaskProxy) TableName() string { return "proxies" }

type TestTaskShop struct {
	ID            int64 `gorm:"primaryKey"`
	ShopID        int64
	ShopName      string
	Status        int
	TokenExpireAt *time.Time
	LastSyncAt    *time.Time
	SyncStatus    int
}

func (TestTaskShop) TableName() string { return "shops" }

type TestTaskDraftProduct struct {
	ID         int64 `gorm:"primaryKey"`
	TaskID     int64
	ShopID     int64
	Title      string
	Status     string
	SyncStatus int
	SyncError  string
	ListingID  int64
}

func (TestTaskDraftProduct) TableName() string { return "draft_products" }

type TestTaskDraftTask struct {
	ID        int64 `gorm:"primaryKey"`
	CreatedAt time.Time
	Status    string
	AIStatus  string
}

func (TestTaskDraftTask) TableName() string { return "draft_tasks" }

// ==================== 辅助函数 ====================

func setupTaskTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	err = db.AutoMigrate(
		&TestTaskProxy{},
		&TestTaskShop{},
		&TestTaskDraftProduct{},
		&TestTaskDraftTask{},
	)
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	return db
}

// ==================== ProxyMonitorTask 测试 ====================

func TestProxyMonitorTask_CheckProxies(t *testing.T) {
	db := setupTaskTestDB(t)

	// 创建测试代理
	now := time.Now()
	oldCheck := now.Add(-10 * time.Minute)
	proxies := []TestTaskProxy{
		{ID: 1, Host: "1.1.1.1", Port: 8080, Status: 1, LastCheckAt: &oldCheck},
		{ID: 2, Host: "2.2.2.2", Port: 8080, Status: 1, LastCheckAt: &oldCheck},
		{ID: 3, Host: "3.3.3.3", Port: 8080, Status: 2, LastCheckAt: &oldCheck}, // 已不可用
	}
	for _, p := range proxies {
		db.Create(&p)
	}

	// 模拟检测：查找需要检测的代理（上次检测超过5分钟）
	checkInterval := 5 * time.Minute
	var needCheck []TestTaskProxy
	db.Where("status = ? AND (last_check_at IS NULL OR last_check_at < ?)", 1, now.Add(-checkInterval)).Find(&needCheck)

	if len(needCheck) != 2 {
		t.Errorf("需要检测的代理数量 = %d, want 2", len(needCheck))
	}

	// 模拟检测结果更新
	for _, p := range needCheck {
		db.Model(&p).Updates(map[string]interface{}{
			"last_check_at": now,
			"response_time": 100,
			"fail_count":    0,
		})
	}

	// 验证更新
	var updated TestTaskProxy
	db.First(&updated, 1)
	if updated.ResponseTime != 100 {
		t.Errorf("ResponseTime = %d, want 100", updated.ResponseTime)
	}
}

func TestProxyMonitorTask_MarkExpired(t *testing.T) {
	db := setupTaskTestDB(t)

	// 创建连续失败的代理
	proxies := []TestTaskProxy{
		{ID: 1, Host: "1.1.1.1", Port: 8080, Status: 1, FailCount: 3},
		{ID: 2, Host: "2.2.2.2", Port: 8080, Status: 1, FailCount: 5}, // 超过阈值
		{ID: 3, Host: "3.3.3.3", Port: 8080, Status: 1, FailCount: 10},
	}
	for _, p := range proxies {
		db.Create(&p)
	}

	// 标记失败次数过多的代理为不可用
	maxFailCount := 4
	result := db.Model(&TestTaskProxy{}).Where("fail_count >= ? AND status = ?", maxFailCount, 1).Update("status", 2)

	if result.RowsAffected != 2 {
		t.Errorf("标记不可用的代理数量 = %d, want 2", result.RowsAffected)
	}
}

// ==================== TokenRefreshTask 测试 ====================

func TestTokenRefreshTask_FindExpiring(t *testing.T) {
	db := setupTaskTestDB(t)

	now := time.Now()
	// 创建测试店铺
	shops := []TestTaskShop{
		{ID: 1, ShopID: 1001, Status: 1, TokenExpireAt: timePtr(now.Add(30 * time.Minute))}, // 即将过期
		{ID: 2, ShopID: 1002, Status: 1, TokenExpireAt: timePtr(now.Add(2 * time.Hour))},    // 未过期
		{ID: 3, ShopID: 1003, Status: 1, TokenExpireAt: timePtr(now.Add(-1 * time.Hour))},   // 已过期
		{ID: 4, ShopID: 1004, Status: 0, TokenExpireAt: timePtr(now.Add(30 * time.Minute))}, // 已停用
	}
	for _, s := range shops {
		db.Create(&s)
	}

	// 查找需要刷新的店铺（1小时内过期且状态正常）
	refreshThreshold := now.Add(1 * time.Hour)
	var expiring []TestTaskShop
	db.Where("status = ? AND token_expire_at < ?", 1, refreshThreshold).Find(&expiring)

	if len(expiring) != 2 {
		t.Errorf("需要刷新的店铺数量 = %d, want 2", len(expiring))
	}
}

func TestTokenRefreshTask_RefreshToken(t *testing.T) {
	db := setupTaskTestDB(t)

	now := time.Now()
	shop := &TestTaskShop{
		ID:            1,
		ShopID:        1001,
		Status:        1,
		TokenExpireAt: timePtr(now.Add(30 * time.Minute)),
	}
	db.Create(shop)

	// 模拟刷新成功
	newExpireAt := now.Add(24 * time.Hour)
	db.Model(shop).Update("token_expire_at", newExpireAt)

	var updated TestTaskShop
	db.First(&updated, 1)

	if updated.TokenExpireAt.Before(now.Add(23 * time.Hour)) {
		t.Error("Token 过期时间未正确更新")
	}
}

// ==================== DraftCleanupTask 测试 ====================

func TestDraftCleanupTask_CleanExpired(t *testing.T) {
	db := setupTaskTestDB(t)

	now := time.Now()
	// 创建测试草稿任务
	tasks := []TestTaskDraftTask{
		{ID: 1, CreatedAt: now.Add(-48 * time.Hour), Status: "draft", AIStatus: "done"},     // 过期
		{ID: 2, CreatedAt: now.Add(-12 * time.Hour), Status: "draft", AIStatus: "done"},     // 未过期
		{ID: 3, CreatedAt: now.Add(-48 * time.Hour), Status: "confirmed", AIStatus: "done"}, // 已确认，不清理
	}
	for _, task := range tasks {
		db.Create(&task)
	}

	// 清理超过24小时未确认的草稿
	expireHours := 24
	expireTime := now.Add(-time.Duration(expireHours) * time.Hour)

	result := db.Model(&TestTaskDraftTask{}).
		Where("created_at < ? AND status = ?", expireTime, "draft").
		Update("status", "expired")

	if result.RowsAffected != 1 {
		t.Errorf("清理的草稿数量 = %d, want 1", result.RowsAffected)
	}
}

// ==================== DraftSubmitTask 测试 ====================

func TestDraftSubmitTask_FindPending(t *testing.T) {
	db := setupTaskTestDB(t)

	// 创建测试数据
	products := []TestTaskDraftProduct{
		{ID: 1, TaskID: 1, ShopID: 1, Status: "confirmed", SyncStatus: 1}, // 待提交
		{ID: 2, TaskID: 1, ShopID: 2, Status: "confirmed", SyncStatus: 2}, // 已提交
		{ID: 3, TaskID: 2, ShopID: 1, Status: "draft", SyncStatus: 0},     // 未确认
		{ID: 4, TaskID: 2, ShopID: 1, Status: "confirmed", SyncStatus: 1}, // 待提交
	}
	for _, p := range products {
		db.Create(&p)
	}

	// 查找待提交的商品
	var pending []TestTaskDraftProduct
	db.Where("status = ? AND sync_status = ?", "confirmed", 1).Find(&pending)

	if len(pending) != 2 {
		t.Errorf("待提交商品数量 = %d, want 2", len(pending))
	}
}

func TestDraftSubmitTask_SubmitSuccess(t *testing.T) {
	db := setupTaskTestDB(t)

	product := &TestTaskDraftProduct{
		ID:         1,
		TaskID:     1,
		ShopID:     1,
		Title:      "Test Product",
		Status:     "confirmed",
		SyncStatus: 1,
	}
	db.Create(product)

	// 模拟提交成功
	listingID := int64(123456789)
	db.Model(product).Updates(map[string]interface{}{
		"sync_status": 2,
		"listing_id":  listingID,
		"sync_error":  "",
	})

	var updated TestTaskDraftProduct
	db.First(&updated, 1)

	if updated.SyncStatus != 2 {
		t.Errorf("SyncStatus = %d, want 2", updated.SyncStatus)
	}
	if updated.ListingID != listingID {
		t.Errorf("ListingID = %d, want %d", updated.ListingID, listingID)
	}
}

func TestDraftSubmitTask_SubmitFailure(t *testing.T) {
	db := setupTaskTestDB(t)

	product := &TestTaskDraftProduct{
		ID:         1,
		TaskID:     1,
		ShopID:     1,
		Title:      "Test Product",
		Status:     "confirmed",
		SyncStatus: 1,
	}
	db.Create(product)

	// 模拟提交失败
	errorMsg := "API rate limit exceeded"
	db.Model(product).Updates(map[string]interface{}{
		"sync_status": 3, // 失败
		"sync_error":  errorMsg,
	})

	var updated TestTaskDraftProduct
	db.First(&updated, 1)

	if updated.SyncStatus != 3 {
		t.Errorf("SyncStatus = %d, want 3", updated.SyncStatus)
	}
	if updated.SyncError != errorMsg {
		t.Errorf("SyncError = %s, want %s", updated.SyncError, errorMsg)
	}
}

// ==================== ShopSyncTask 测试 ====================

func TestShopSyncTask_FindNeedSync(t *testing.T) {
	db := setupTaskTestDB(t)

	now := time.Now()
	shops := []TestTaskShop{
		{ID: 1, ShopID: 1001, Status: 1, LastSyncAt: timePtr(now.Add(-2 * time.Hour)), SyncStatus: 0},
		{ID: 2, ShopID: 1002, Status: 1, LastSyncAt: timePtr(now.Add(-30 * time.Minute)), SyncStatus: 0},
		{ID: 3, ShopID: 1003, Status: 0, LastSyncAt: timePtr(now.Add(-2 * time.Hour)), SyncStatus: 0}, // 已停用
	}
	for _, s := range shops {
		db.Create(&s)
	}

	// 查找需要同步的店铺（上次同步超过1小时）
	syncInterval := 1 * time.Hour
	var needSync []TestTaskShop
	db.Where("status = ? AND (last_sync_at IS NULL OR last_sync_at < ?)", 1, now.Add(-syncInterval)).Find(&needSync)

	if len(needSync) != 1 {
		t.Errorf("需要同步的店铺数量 = %d, want 1", len(needSync))
	}
}

// ==================== Task 并发安全测试 ====================

func TestTask_ConcurrentExecution(t *testing.T) {
	db := setupTaskTestDB(t)

	// 创建测试数据
	for i := 1; i <= 100; i++ {
		db.Create(&TestTaskProxy{
			ID:        int64(i),
			Host:      "1.1.1.1",
			Port:      8080 + i,
			Status:    1,
			FailCount: 0,
		})
	}

	// 并发更新
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	for i := 1; i <= 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := db.Model(&TestTaskProxy{}).Where("id = ?", id).Updates(map[string]interface{}{
				"response_time": id * 10,
				"last_check_at": time.Now(),
			}).Error
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Logf("并发更新错误: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Errorf("并发更新失败数: %d", errorCount)
	}

	// 验证更新结果
	var count int64
	db.Model(&TestTaskProxy{}).Where("response_time > 0").Count(&count)
	if count != 100 {
		t.Errorf("成功更新数量 = %d, want 100", count)
	}
}

// ==================== Task 调度器测试 ====================

type MockTask struct {
	name     string
	interval time.Duration
	runCount int
	mu       sync.Mutex
}

func (t *MockTask) Name() string {
	return t.name
}

func (t *MockTask) Interval() time.Duration {
	return t.interval
}

func (t *MockTask) Run(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runCount++
	return nil
}

func (t *MockTask) GetRunCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.runCount
}

func TestTaskScheduler_BasicExecution(t *testing.T) {
	task := &MockTask{
		name:     "test-task",
		interval: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// 模拟调度器运行
	go func() {
		ticker := time.NewTicker(task.Interval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				task.Run(ctx)
			}
		}
	}()

	<-ctx.Done()
	time.Sleep(10 * time.Millisecond) // 等待最后一次执行完成

	runCount := task.GetRunCount()
	if runCount < 2 {
		t.Errorf("任务执行次数 = %d, want >= 2", runCount)
	}
	t.Logf("任务执行次数: %d", runCount)
}

func TestTaskScheduler_GracefulShutdown(t *testing.T) {
	task := &MockTask{
		name:     "shutdown-test",
		interval: 10 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(task.Interval())
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				done <- true
				return
			case <-ticker.C:
				task.Run(ctx)
			}
		}
	}()

	// 运行一段时间后取消
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		t.Log("调度器正常退出")
	case <-time.After(100 * time.Millisecond):
		t.Error("调度器未能正常退出")
	}
}

// ==================== 辅助函数 ====================

func timePtr(t time.Time) *time.Time {
	return &t
}
