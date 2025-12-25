package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("连接测试数据库失败: %v", err)
	}

	err = db.AutoMigrate(&model.DraftTask{}, &model.DraftProduct{}, &model.DraftImage{})
	if err != nil {
		t.Fatalf("数据库迁移失败: %v", err)
	}

	return db
}

// ==================== DraftTaskRepository 测试 ====================

func TestDraftTaskRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftTaskRepository(db)
	ctx := context.Background()

	task := &model.DraftTask{
		UserID:         1,
		SourceURL:      "https://detail.1688.com/offer/123.html",
		SourcePlatform: "1688",
		SourceItemID:   "123",
		Status:         model.TaskStatusPending,
		AIStatus:       model.AIStatusPending,
	}

	err := repo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.ID == 0 {
		t.Error("ID 应该被自动分配")
	}
}

func TestDraftTaskRepo_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftTaskRepository(db)
	ctx := context.Background()

	// 创建
	task := &model.DraftTask{UserID: 1, Status: model.TaskStatusDraft}
	repo.Create(ctx, task)

	// 查询
	found, err := repo.GetByID(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.UserID != 1 {
		t.Errorf("UserID = %d, want 1", found.UserID)
	}
}

func TestDraftTaskRepo_UpdateFields(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftTaskRepository(db)
	ctx := context.Background()

	task := &model.DraftTask{UserID: 1, Status: model.TaskStatusPending}
	repo.Create(ctx, task)

	// 更新
	err := repo.UpdateFields(ctx, task.ID, map[string]interface{}{
		"status":    model.TaskStatusProcessing,
		"ai_status": model.AIStatusProcessing,
	})
	if err != nil {
		t.Fatalf("UpdateFields() error = %v", err)
	}

	// 验证
	found, _ := repo.GetByID(ctx, task.ID)
	if found.Status != model.TaskStatusProcessing {
		t.Errorf("Status = %s, want %s", found.Status, model.TaskStatusProcessing)
	}
}

func TestDraftTaskRepo_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftTaskRepository(db)
	ctx := context.Background()

	// 创建测试数据
	for i := 0; i < 5; i++ {
		repo.Create(ctx, &model.DraftTask{
			UserID: 1,
			Status: model.TaskStatusDraft,
		})
	}
	repo.Create(ctx, &model.DraftTask{
		UserID: 2,
		Status: model.TaskStatusDraft,
	})

	// 查询用户1
	tasks, total, err := repo.List(ctx, TaskFilter{UserID: 1, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(tasks) != 5 {
		t.Errorf("len(tasks) = %d, want 5", len(tasks))
	}
}

func TestDraftTaskRepo_GetExpired(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftTaskRepository(db)
	ctx := context.Background()

	now := time.Now()

	// 手动设置 CreatedAt
	db.Exec("INSERT INTO draft_tasks (user_id, source_url, status, ai_status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		1, "url1", model.TaskStatusDraft, model.AIStatusDone, now.Add(-48*time.Hour), now)
	db.Exec("INSERT INTO draft_tasks (user_id, source_url, status, ai_status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		1, "url2", model.TaskStatusDraft, model.AIStatusDone, now.Add(-12*time.Hour), now)

	// 查询过期
	expired, err := repo.GetExpired(ctx, now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("GetExpired() error = %v", err)
	}

	if len(expired) != 1 {
		t.Errorf("len(expired) = %d, want 1", len(expired))
	}
}

// ==================== DraftProductRepository 测试 ====================

func TestDraftProductRepo_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftProductRepository(db)
	ctx := context.Background()

	product := &model.DraftProduct{
		TaskID: 1,
		ShopID: 1,
		Title:  "Test Product",
		Status: model.DraftStatusDraft,
	}

	err := repo.Create(ctx, product)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if product.ID == 0 {
		t.Error("ID 应该被自动分配")
	}
}

func TestDraftProductRepo_CreateBatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftProductRepository(db)
	ctx := context.Background()

	products := []model.DraftProduct{
		{TaskID: 1, ShopID: 1, Title: "Product 1", Status: model.DraftStatusDraft},
		{TaskID: 1, ShopID: 2, Title: "Product 2", Status: model.DraftStatusDraft},
		{TaskID: 1, ShopID: 3, Title: "Product 3", Status: model.DraftStatusDraft},
	}

	err := repo.CreateBatch(ctx, products)
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}

	// 验证
	found, _ := repo.GetByTaskID(ctx, 1)
	if len(found) != 3 {
		t.Errorf("len(found) = %d, want 3", len(found))
	}
}

func TestDraftProductRepo_GetByTaskID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftProductRepository(db)
	ctx := context.Background()

	// 创建
	for i := 0; i < 3; i++ {
		repo.Create(ctx, &model.DraftProduct{TaskID: 1, ShopID: int64(i + 1), Status: model.DraftStatusDraft})
	}
	repo.Create(ctx, &model.DraftProduct{TaskID: 2, ShopID: 1, Status: model.DraftStatusDraft})

	// 查询
	products, err := repo.GetByTaskID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByTaskID() error = %v", err)
	}

	if len(products) != 3 {
		t.Errorf("len(products) = %d, want 3", len(products))
	}
}

func TestDraftProductRepo_ConfirmAll(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftProductRepository(db)
	ctx := context.Background()

	// 创建
	products := []model.DraftProduct{
		{TaskID: 1, ShopID: 1, Title: "P1", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
		{TaskID: 1, ShopID: 2, Title: "P2", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
		{TaskID: 2, ShopID: 1, Title: "P3", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
	}
	repo.CreateBatch(ctx, products)

	// 确认任务1
	affected, err := repo.ConfirmAll(ctx, 1)
	if err != nil {
		t.Fatalf("ConfirmAll() error = %v", err)
	}

	if affected != 2 {
		t.Errorf("affected = %d, want 2", affected)
	}

	// 验证
	found, _ := repo.GetByTaskID(ctx, 1)
	for _, p := range found {
		if p.Status != model.DraftStatusConfirmed {
			t.Errorf("product %d status = %s, want confirmed", p.ID, p.Status)
		}
	}

	// 任务2不受影响
	found2, _ := repo.GetByTaskID(ctx, 2)
	if found2[0].Status != model.DraftStatusDraft {
		t.Error("任务2的商品不应被确认")
	}
}

func TestDraftProductRepo_GetPendingSync(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftProductRepository(db)
	ctx := context.Background()

	// 创建
	products := []model.DraftProduct{
		{TaskID: 1, ShopID: 1, Status: model.DraftStatusConfirmed, SyncStatus: model.DraftSyncStatusPending},
		{TaskID: 1, ShopID: 2, Status: model.DraftStatusConfirmed, SyncStatus: model.DraftSyncStatusPending},
		{TaskID: 1, ShopID: 3, Status: model.DraftStatusConfirmed, SyncStatus: model.DraftSyncStatusDone},
		{TaskID: 1, ShopID: 4, Status: model.DraftStatusDraft, SyncStatus: model.DraftSyncStatusNone},
	}
	repo.CreateBatch(ctx, products)

	// 查询待同步
	pending, err := repo.GetPendingSync(ctx, 10)
	if err != nil {
		t.Fatalf("GetPendingSync() error = %v", err)
	}

	if len(pending) != 2 {
		t.Errorf("len(pending) = %d, want 2", len(pending))
	}
}

// ==================== DraftImageRepository 测试 ====================

func TestDraftImageRepo_CreateBatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftImageRepository(db)
	ctx := context.Background()

	images := []model.DraftImage{
		{TaskID: 1, GroupIndex: 0, ImageIndex: 0, StorageURL: "url1", Status: model.ImageStatusReady},
		{TaskID: 1, GroupIndex: 0, ImageIndex: 1, StorageURL: "url2", Status: model.ImageStatusReady},
		{TaskID: 1, GroupIndex: 1, ImageIndex: 0, StorageURL: "url3", Status: model.ImageStatusReady},
	}

	err := repo.CreateBatch(ctx, images)
	if err != nil {
		t.Fatalf("CreateBatch() error = %v", err)
	}

	// 验证
	found, _ := repo.GetByTaskID(ctx, 1)
	if len(found) != 3 {
		t.Errorf("len(found) = %d, want 3", len(found))
	}
}

func TestDraftImageRepo_GetByGroup(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftImageRepository(db)
	ctx := context.Background()

	// 创建
	images := []model.DraftImage{
		{TaskID: 1, GroupIndex: 0, ImageIndex: 0, StorageURL: "url1"},
		{TaskID: 1, GroupIndex: 0, ImageIndex: 1, StorageURL: "url2"},
		{TaskID: 1, GroupIndex: 1, ImageIndex: 0, StorageURL: "url3"},
	}
	repo.CreateBatch(ctx, images)

	// 查询组0
	group0, err := repo.GetByGroup(ctx, 1, 0)
	if err != nil {
		t.Fatalf("GetByGroup() error = %v", err)
	}

	if len(group0) != 2 {
		t.Errorf("len(group0) = %d, want 2", len(group0))
	}
}

func TestDraftImageRepo_DeleteByGroup(t *testing.T) {
	db := setupTestDB(t)
	repo := NewDraftImageRepository(db)
	ctx := context.Background()

	// 创建
	images := []model.DraftImage{
		{TaskID: 1, GroupIndex: 0, ImageIndex: 0},
		{TaskID: 1, GroupIndex: 0, ImageIndex: 1},
		{TaskID: 1, GroupIndex: 1, ImageIndex: 0},
	}
	repo.CreateBatch(ctx, images)

	// 删除组0
	err := repo.DeleteByGroup(ctx, 1, 0)
	if err != nil {
		t.Fatalf("DeleteByGroup() error = %v", err)
	}

	// 验证
	remaining, _ := repo.GetByTaskID(ctx, 1)
	if len(remaining) != 1 {
		t.Errorf("len(remaining) = %d, want 1", len(remaining))
	}
}

// ==================== UnitOfWork 测试 ====================

func TestDraftUnitOfWork_Transaction(t *testing.T) {
	db := setupTestDB(t)
	uow := NewDraftUnitOfWork(db)
	ctx := context.Background()

	// 成功事务
	err := uow.Transaction(ctx, func(txUow *DraftUnitOfWork) error {
		task := &model.DraftTask{UserID: 1, Status: model.TaskStatusDraft}
		if err := txUow.Tasks.Create(ctx, task); err != nil {
			return err
		}

		product := &model.DraftProduct{TaskID: task.ID, ShopID: 1, Status: model.DraftStatusDraft}
		return txUow.Products.Create(ctx, product)
	})

	if err != nil {
		t.Fatalf("Transaction() error = %v", err)
	}

	// 验证
	tasks, total, _ := uow.Tasks.List(ctx, TaskFilter{UserID: 1})
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(tasks) != 1 {
		t.Error("应该有1个任务")
	}
}

func TestDraftUnitOfWork_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	uow := NewDraftUnitOfWork(db)
	ctx := context.Background()

	// 失败事务
	err := uow.Transaction(ctx, func(txUow *DraftUnitOfWork) error {
		task := &model.DraftTask{UserID: 1, Status: model.TaskStatusDraft}
		if err := txUow.Tasks.Create(ctx, task); err != nil {
			return err
		}

		// 模拟错误
		return gorm.ErrInvalidData
	})

	if err == nil {
		t.Error("应该返回错误")
	}

	// 验证回滚
	_, total, _ := uow.Tasks.List(ctx, TaskFilter{UserID: 1})
	if total != 0 {
		t.Errorf("total = %d, want 0 (已回滚)", total)
	}
}
