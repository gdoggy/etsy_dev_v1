package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"etsy_dev_v1_202512/internal/model"
)

// 测试用 BaseModel（仅用于测试）
type testBaseModel struct {
	ID        int64 `gorm:"primary_key;AUTO_INCREMENT"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	CreatedBy int64
	UpdatedBy int64
}

// 测试用 AICallLog
type testAICallLog struct {
	testBaseModel
	ShopID       int64  `gorm:"index"`
	TaskID       int64  `gorm:"index"`
	CallType     string `gorm:"size:32"`
	ModelName    string `gorm:"size:64"`
	InputTokens  int
	OutputTokens int
	ImageCount   int
	DurationMs   int64
	CostUSD      float64 `gorm:"type:decimal(10,6)"`
	Status       string  `gorm:"size:32"`
	ErrorMsg     string  `gorm:"size:1024"`
}

func (testAICallLog) TableName() string {
	return "ai_call_logs"
}

func setupAILogTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	err = db.AutoMigrate(&testAICallLog{})
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	return db
}

func TestAICallLogRepo_Create(t *testing.T) {
	db := setupAILogTestDB(t)
	repo := NewAICallLogRepository(db)
	ctx := context.Background()

	log := &model.AICallLog{
		ShopID:       1,
		TaskID:       100,
		CallType:     model.AICallTypeText,
		ModelName:    "gemini-2.0-flash",
		InputTokens:  500,
		OutputTokens: 200,
		DurationMs:   1500,
		CostUSD:      0.001,
		Status:       model.AICallStatusSuccess,
	}

	err := repo.Create(ctx, log)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if log.ID == 0 {
		t.Error("ID 应该被自动分配")
	}
}

func TestAICallLogRepo_GetByID(t *testing.T) {
	db := setupAILogTestDB(t)
	repo := NewAICallLogRepository(db)
	ctx := context.Background()

	// 创建
	log := &model.AICallLog{
		ShopID:    1,
		TaskID:    100,
		CallType:  model.AICallTypeImage,
		ModelName: "imagen-3.0",
		Status:    model.AICallStatusSuccess,
	}
	repo.Create(ctx, log)

	// 查询
	found, err := repo.GetByID(ctx, log.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.CallType != model.AICallTypeImage {
		t.Errorf("CallType = %s, want image", found.CallType)
	}
}

func TestAICallLogRepo_GetUsageByShop(t *testing.T) {
	db := setupAILogTestDB(t)
	repo := NewAICallLogRepository(db)
	ctx := context.Background()

	// 创建测试数据
	logs := []*model.AICallLog{
		{ShopID: 1, TaskID: 1, CallType: model.AICallTypeText, InputTokens: 100, OutputTokens: 50, CostUSD: 0.001, Status: model.AICallStatusSuccess},
		{ShopID: 1, TaskID: 2, CallType: model.AICallTypeText, InputTokens: 200, OutputTokens: 100, CostUSD: 0.002, Status: model.AICallStatusSuccess},
		{ShopID: 1, TaskID: 3, CallType: model.AICallTypeImage, ImageCount: 5, CostUSD: 0.01, Status: model.AICallStatusSuccess},
		{ShopID: 1, TaskID: 4, CallType: model.AICallTypeText, Status: model.AICallStatusFailed},
		{ShopID: 2, TaskID: 5, CallType: model.AICallTypeText, InputTokens: 500, CostUSD: 0.005, Status: model.AICallStatusSuccess},
	}
	for _, log := range logs {
		repo.Create(ctx, log)
	}

	// 查询 shop 1 统计
	stats, err := repo.GetUsageByShop(ctx, 1, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("GetUsageByShop() error = %v", err)
	}

	if stats.TotalCalls != 4 {
		t.Errorf("TotalCalls = %d, want 4", stats.TotalCalls)
	}
	if stats.TextCalls != 3 {
		t.Errorf("TextCalls = %d, want 3", stats.TextCalls)
	}
	if stats.ImageCalls != 1 {
		t.Errorf("ImageCalls = %d, want 1", stats.ImageCalls)
	}
	if stats.TotalInputTokens != 300 {
		t.Errorf("TotalInputTokens = %d, want 300", stats.TotalInputTokens)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", stats.SuccessCount)
	}
	if stats.FailedCount != 1 {
		t.Errorf("FailedCount = %d, want 1", stats.FailedCount)
	}
}

func TestAICallLogRepo_GetUsageByTask(t *testing.T) {
	db := setupAILogTestDB(t)
	repo := NewAICallLogRepository(db)
	ctx := context.Background()

	// 同一任务多次调用
	logs := []*model.AICallLog{
		{ShopID: 1, TaskID: 100, CallType: model.AICallTypeText, InputTokens: 500, OutputTokens: 200, CostUSD: 0.003, DurationMs: 1000, Status: model.AICallStatusSuccess},
		{ShopID: 1, TaskID: 100, CallType: model.AICallTypeImage, ImageCount: 10, CostUSD: 0.02, DurationMs: 5000, Status: model.AICallStatusSuccess},
		{ShopID: 1, TaskID: 200, CallType: model.AICallTypeText, InputTokens: 100, Status: model.AICallStatusSuccess},
	}
	for _, log := range logs {
		repo.Create(ctx, log)
	}

	stats, err := repo.GetUsageByTask(ctx, 100)
	if err != nil {
		t.Fatalf("GetUsageByTask() error = %v", err)
	}

	if stats.TotalCalls != 2 {
		t.Errorf("TotalCalls = %d, want 2", stats.TotalCalls)
	}
	if stats.TotalImages != 10 {
		t.Errorf("TotalImages = %d, want 10", stats.TotalImages)
	}
}

func TestAICallLogRepo_GetTotalCost(t *testing.T) {
	db := setupAILogTestDB(t)
	repo := NewAICallLogRepository(db)
	ctx := context.Background()

	logs := []*model.AICallLog{
		{ShopID: 1, CallType: model.AICallTypeText, CostUSD: 0.01, Status: model.AICallStatusSuccess},
		{ShopID: 1, CallType: model.AICallTypeImage, CostUSD: 0.05, Status: model.AICallStatusSuccess},
		{ShopID: 2, CallType: model.AICallTypeText, CostUSD: 0.02, Status: model.AICallStatusSuccess},
	}
	for _, log := range logs {
		repo.Create(ctx, log)
	}

	totalCost, err := repo.GetTotalCost(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("GetTotalCost() error = %v", err)
	}

	expected := 0.08
	if totalCost < expected-0.001 || totalCost > expected+0.001 {
		t.Errorf("TotalCost = %f, want %f", totalCost, expected)
	}
}
