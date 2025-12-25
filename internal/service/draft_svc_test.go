package service

import (
	"context"
	"etsy_dev_v1_202512/internal/api/dto"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
)

// ==================== Mock 实现 ====================

type mockScraper struct {
	parseURLFn     func(url string) (string, string, error)
	fetchProductFn func(ctx context.Context, platform, itemID string) (*ScrapedProduct, error)
}

func (m *mockScraper) ParseURL(url string) (string, string, error) {
	if m.parseURLFn != nil {
		return m.parseURLFn(url)
	}
	return "1688", "123456", nil
}

func (m *mockScraper) FetchProduct(ctx context.Context, platform, itemID string) (*ScrapedProduct, error) {
	if m.fetchProductFn != nil {
		return m.fetchProductFn(ctx, platform, itemID)
	}
	return &ScrapedProduct{
		Platform:    platform,
		ItemID:      itemID,
		Title:       "测试商品",
		Price:       99.99,
		Currency:    "CNY",
		Images:      []string{"https://example.com/img1.jpg"},
		Description: "测试描述",
	}, nil
}

type mockAI struct {
	generateContentFn func(ctx context.Context, title, styleHint string) (*TextGenerateResult, error)
	generateImagesFn  func(ctx context.Context, prompt, refURL string, count int) ([]string, error)
}

func (m *mockAI) GenerateProductContent(ctx context.Context, title, styleHint string) (*TextGenerateResult, error) {
	if m.generateContentFn != nil {
		return m.generateContentFn(ctx, title, styleHint)
	}
	return &TextGenerateResult{
		Title:       "AI Generated: " + title,
		Description: "AI generated description",
		Tags:        []string{"handmade", "vintage", "gift"},
	}, nil
}

func (m *mockAI) GenerateImages(ctx context.Context, prompt, refURL string, count int) ([]string, error) {
	if m.generateImagesFn != nil {
		return m.generateImagesFn(ctx, prompt, refURL, count)
	}
	images := make([]string, count)
	for i := 0; i < count; i++ {
		images[i] = "base64encodedimage"
	}
	return images, nil
}

type mockStorage struct {
	saveBase64Fn func(data, prefix string) (string, error)
}

func (m *mockStorage) SaveBase64(data, prefix string) (string, error) {
	if m.saveBase64Fn != nil {
		return m.saveBase64Fn(data, prefix)
	}
	return "https://storage.example.com/" + prefix + ".jpg", nil
}

// ==================== 测试辅助函数 ====================

func setupServiceTestDB(t *testing.T) *gorm.DB {
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

func newTestService(t *testing.T) (*DraftService, *gorm.DB) {
	db := setupServiceTestDB(t)
	uow := repository.NewDraftUnitOfWork(db)

	svc := NewDraftService(
		uow,
		&mockScraper{},
		&mockAI{},
		&mockStorage{},
	)

	return svc, db
}

// ==================== Subscribe 测试 ====================

func TestDraftService_Subscribe(t *testing.T) {
	svc, _ := newTestService(t)

	taskID := int64(1)

	// 订阅
	ch := svc.Subscribe(taskID)
	if ch == nil {
		t.Fatal("Subscribe() 返回 nil")
	}

	// 发送事件
	go func() {
		svc.notifyProgress(taskID, dto.ProgressEvent{
			TaskID:   taskID,
			Stage:    "test",
			Progress: 50,
		})
	}()

	// 接收
	select {
	case event := <-ch:
		if event.Progress != 50 {
			t.Errorf("Progress = %d, want 50", event.Progress)
		}
	case <-time.After(time.Second):
		t.Error("超时等待事件")
	}

	// 取消订阅
	svc.Unsubscribe(taskID, ch)
}

// ==================== ListTasks 测试 ====================

func TestDraftService_ListTasks(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 创建测试数据
	for i := 0; i < 5; i++ {
		db.Create(&model.DraftTask{
			UserID: 1,
			Status: model.TaskStatusDraft,
		})
	}
	db.Create(&model.DraftTask{
		UserID: 2,
		Status: model.TaskStatusDraft,
	})

	// 查询
	tasks, total, err := svc.ListTasks(ctx, &dto.ListDraftTasksRequest{
		UserID:   1,
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}

	if len(tasks) != 5 {
		t.Errorf("len(tasks) = %d, want 5", len(tasks))
	}
}

// ==================== GetTaskDetail 测试 ====================

func TestDraftService_GetTaskDetail(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 创建任务
	task := &model.DraftTask{
		UserID:         1,
		SourceURL:      "https://detail.1688.com/offer/123.html",
		SourcePlatform: "1688",
		SourceItemID:   "123",
		Status:         model.TaskStatusDraft,
		AIStatus:       model.AIStatusDone,
		SourceData: model.JSONMap{
			"title": "测试商品",
			"price": 99.99,
		},
		AITextResult: model.JSONMap{
			"title":       "AI Title",
			"description": "AI Desc",
			"tags":        []interface{}{"tag1", "tag2"},
		},
		AIImages: model.StringSlice{"img1.jpg", "img2.jpg"},
	}
	db.Create(task)

	// 创建商品
	db.Create(&model.DraftProduct{
		TaskID: task.ID,
		ShopID: 1,
		Title:  "Product",
		Status: model.DraftStatusDraft,
	})

	// 查询
	detail, err := svc.GetTaskDetail(ctx, task.ID)
	if err != nil {
		t.Fatalf("GetTaskDetail() error = %v", err)
	}

	if detail.Task.ID != task.ID {
		t.Errorf("Task.ID = %d, want %d", detail.Task.ID, task.ID)
	}

	if len(detail.Products) != 1 {
		t.Errorf("len(Products) = %d, want 1", len(detail.Products))
	}

	if detail.AIResult.Title != "AI Title" {
		t.Errorf("AIResult.Title = %s, want AI Title", detail.AIResult.Title)
	}
}

func TestDraftService_GetTaskDetail_NotFound(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	_, err := svc.GetTaskDetail(ctx, 99999)
	if err == nil {
		t.Error("应该返回错误")
	}
}

// ==================== UpdateDraftProduct 测试 ====================

func TestDraftService_UpdateDraftProduct(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 创建商品
	product := &model.DraftProduct{
		TaskID: 1,
		ShopID: 1,
		Title:  "Original",
		Status: model.DraftStatusDraft,
	}
	db.Create(product)

	// 更新
	newTitle := "Updated Title"
	newPrice := 199.99
	err := svc.UpdateDraftProduct(ctx, product.ID, &dto.UpdateDraftProductRequest{
		Title: &newTitle,
		Price: &newPrice,
	})

	if err != nil {
		t.Fatalf("UpdateDraftProduct() error = %v", err)
	}

	// 验证
	var updated model.DraftProduct
	db.First(&updated, product.ID)

	if updated.Title != newTitle {
		t.Errorf("Title = %s, want %s", updated.Title, newTitle)
	}

	if updated.PriceAmount != 19999 {
		t.Errorf("PriceAmount = %d, want 19999", updated.PriceAmount)
	}
}

func TestDraftService_UpdateDraftProduct_NotDraft(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 创建已确认商品
	product := &model.DraftProduct{
		TaskID: 1,
		ShopID: 1,
		Title:  "Original",
		Status: model.DraftStatusConfirmed,
	}
	db.Create(product)

	// 尝试更新
	newTitle := "Updated"
	err := svc.UpdateDraftProduct(ctx, product.ID, &dto.UpdateDraftProductRequest{
		Title: &newTitle,
	})

	if err == nil {
		t.Error("应该返回错误：只能修改草稿状态的商品")
	}
}

// ==================== ConfirmDraft 测试 ====================

func TestDraftService_ConfirmDraft(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	product := &model.DraftProduct{
		TaskID:            1,
		ShopID:            1,
		Title:             "Test Product",
		TaxonomyID:        123,
		ShippingProfileID: 456,
		Status:            model.DraftStatusDraft,
	}
	db.Create(product)

	err := svc.ConfirmDraft(ctx, product.ID)
	if err != nil {
		t.Fatalf("ConfirmDraft() error = %v", err)
	}

	// 验证
	var updated model.DraftProduct
	db.First(&updated, product.ID)

	if updated.Status != model.DraftStatusConfirmed {
		t.Errorf("Status = %s, want confirmed", updated.Status)
	}

	if updated.SyncStatus != int(model.ProductSyncStatusPending) {
		t.Errorf("SyncStatus = %d, want %d", updated.SyncStatus, model.ProductSyncStatusPending)
	}
}

func TestDraftService_ConfirmDraft_ValidationError(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		product *model.DraftProduct
		wantErr string
	}{
		{
			name: "标题为空",
			product: &model.DraftProduct{
				TaskID:            1,
				ShopID:            1,
				Title:             "",
				TaxonomyID:        123,
				ShippingProfileID: 456,
				Status:            model.DraftStatusDraft,
			},
			wantErr: "标题不能为空",
		},
		{
			name: "分类为空",
			product: &model.DraftProduct{
				TaskID:            1,
				ShopID:            1,
				Title:             "Test",
				TaxonomyID:        0,
				ShippingProfileID: 456,
				Status:            model.DraftStatusDraft,
			},
			wantErr: "请选择商品分类",
		},
		{
			name: "运费模板为空",
			product: &model.DraftProduct{
				TaskID:            1,
				ShopID:            1,
				Title:             "Test",
				TaxonomyID:        123,
				ShippingProfileID: 0,
				Status:            model.DraftStatusDraft,
			},
			wantErr: "请选择运费模板",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db.Create(tt.product)

			err := svc.ConfirmDraft(ctx, tt.product.ID)
			if err == nil {
				t.Error("应该返回错误")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// ==================== ConfirmAllDrafts 测试 ====================

func TestDraftService_ConfirmAllDrafts(t *testing.T) {
	svc, db := newTestService(t)
	ctx := context.Background()

	// 创建测试数据
	products := []model.DraftProduct{
		{TaskID: 1, ShopID: 1, Title: "P1", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
		{TaskID: 1, ShopID: 2, Title: "P2", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
		{TaskID: 2, ShopID: 1, Title: "P3", TaxonomyID: 1, ShippingProfileID: 1, Status: model.DraftStatusDraft},
	}
	for _, p := range products {
		db.Create(&p)
	}

	// 确认任务1
	affected, err := svc.ConfirmAllDrafts(ctx, 1)
	if err != nil {
		t.Fatalf("ConfirmAllDrafts() error = %v", err)
	}

	if affected != 2 {
		t.Errorf("affected = %d, want 2", affected)
	}

	// 验证
	var confirmed []model.DraftProduct
	db.Where("task_id = ? AND status = ?", 1, model.DraftStatusConfirmed).Find(&confirmed)

	if len(confirmed) != 2 {
		t.Errorf("len(confirmed) = %d, want 2", len(confirmed))
	}
}

// ==================== GetSupportedPlatforms 测试 ====================

func TestDraftService_GetSupportedPlatforms(t *testing.T) {
	svc, _ := newTestService(t)

	result := svc.GetSupportedPlatforms()

	if len(result.Platforms) == 0 {
		t.Error("平台列表不应为空")
	}

	found := false
	for _, p := range result.Platforms {
		if p.Code == "1688" {
			found = true
			break
		}
	}

	if !found {
		t.Error("应该包含 1688 平台")
	}
}

// ==================== Model 辅助方法测试 ====================

func TestDraftProduct_GetPrice(t *testing.T) {
	p := &model.DraftProduct{
		PriceAmount:  19999,
		PriceDivisor: 100,
	}

	price := p.GetPrice()
	if price != 199.99 {
		t.Errorf("GetPrice() = %f, want 199.99", price)
	}
}

func TestDraftProduct_SetPrice(t *testing.T) {
	p := &model.DraftProduct{}
	p.SetPrice(199.99)

	if p.PriceAmount != 19999 {
		t.Errorf("PriceAmount = %d, want 19999", p.PriceAmount)
	}
}

func TestDraftProduct_CanConfirm(t *testing.T) {
	tests := []struct {
		name    string
		product *model.DraftProduct
		wantErr bool
	}{
		{
			name: "valid",
			product: &model.DraftProduct{
				Title:             "Test",
				TaxonomyID:        123,
				ShippingProfileID: 456,
				Status:            model.DraftStatusDraft,
			},
			wantErr: false,
		},
		{
			name: "not draft",
			product: &model.DraftProduct{
				Title:             "Test",
				TaxonomyID:        123,
				ShippingProfileID: 456,
				Status:            model.DraftStatusConfirmed,
			},
			wantErr: true,
		},
		{
			name: "empty title",
			product: &model.DraftProduct{
				Title:             "",
				TaxonomyID:        123,
				ShippingProfileID: 456,
				Status:            model.DraftStatusDraft,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.product.CanConfirm()
			if (err != nil) != tt.wantErr {
				t.Errorf("CanConfirm() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
