package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 仓储接口 ====================

// DraftTaskRepository 草稿任务仓储接口
type DraftTaskRepository interface {
	Create(ctx context.Context, task *model.DraftTask) error
	GetByID(ctx context.Context, id int64) (*model.DraftTask, error)
	Update(ctx context.Context, task *model.DraftTask) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter TaskFilter) ([]model.DraftTask, int64, error)
	UpdateStatus(ctx context.Context, id int64, status, aiStatus string) error

	// 过期清理相关
	FindExpired(ctx context.Context, before time.Time) ([]*model.DraftTask, error)
	MarkExpired(ctx context.Context, id int64) error
}

// DraftProductRepository 草稿商品仓储接口
type DraftProductRepository interface {
	Create(ctx context.Context, product *model.DraftProduct) error
	CreateBatch(ctx context.Context, products []model.DraftProduct) error
	GetByID(ctx context.Context, id int64) (*model.DraftProduct, error)
	Update(ctx context.Context, product *model.DraftProduct) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error
	GetByTaskID(ctx context.Context, taskID int64) ([]model.DraftProduct, error)
	ConfirmAll(ctx context.Context, taskID int64) (int64, error)
	CountByTaskID(ctx context.Context, taskID int64) (int64, error)

	// 提交任务相关
	FindPendingSubmit(ctx context.Context, limit int) ([]*model.DraftProduct, error)
	UpdateSyncStatus(ctx context.Context, id int64, status int) error
	MarkSubmitted(ctx context.Context, id int64, listingID int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
	UpdateProductID(ctx context.Context, id int64, productID int64) error
	DeleteByTaskID(ctx context.Context, taskID int64) error
}

// DraftImageRepository 草稿图片仓储接口
type DraftImageRepository interface {
	Create(ctx context.Context, image *model.DraftImage) error
	CreateBatch(ctx context.Context, images []model.DraftImage) error
	GetByID(ctx context.Context, id int64) (*model.DraftImage, error)
	Update(ctx context.Context, image *model.DraftImage) error
	Delete(ctx context.Context, id int64) error
	GetByTaskID(ctx context.Context, taskID int64) ([]model.DraftImage, error)
	GetByGroup(ctx context.Context, taskID int64, groupIndex int) ([]model.DraftImage, error)
	DeleteByTaskID(ctx context.Context, taskID int64) error
	DeleteByGroup(ctx context.Context, taskID int64, groupIndex int) error
}

// ==================== 过滤条件 ====================

// TaskFilter 任务过滤条件
type TaskFilter struct {
	UserID   int64
	Status   string
	AIStatus string
	Page     int
	PageSize int
}

// ==================== DraftTask 仓储实现 ====================

type draftTaskRepo struct {
	db *gorm.DB
}

// NewDraftTaskRepository 创建草稿任务仓储
func NewDraftTaskRepository(db *gorm.DB) DraftTaskRepository {
	return &draftTaskRepo{db: db}
}

func (r *draftTaskRepo) Create(ctx context.Context, task *model.DraftTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *draftTaskRepo) GetByID(ctx context.Context, id int64) (*model.DraftTask, error) {
	var task model.DraftTask
	if err := r.db.WithContext(ctx).First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *draftTaskRepo) Update(ctx context.Context, task *model.DraftTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}

func (r *draftTaskRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.DraftTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *draftTaskRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.DraftTask{}, id).Error
}

func (r *draftTaskRepo) List(ctx context.Context, filter TaskFilter) ([]model.DraftTask, int64, error) {
	var tasks []model.DraftTask
	var total int64

	query := r.db.WithContext(ctx).Model(&model.DraftTask{})

	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.AIStatus != "" {
		query = query.Where("ai_status = ?", filter.AIStatus)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}

	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Limit(filter.PageSize).Offset(offset).Find(&tasks).Error; err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

func (r *draftTaskRepo) UpdateStatus(ctx context.Context, id int64, status, aiStatus string) error {
	updates := make(map[string]interface{})
	if status != "" {
		updates["status"] = status
	}
	if aiStatus != "" {
		updates["ai_status"] = aiStatus
	}
	return r.db.WithContext(ctx).Model(&model.DraftTask{}).Where("id = ?", id).Updates(updates).Error
}

// FindExpired 查找过期的草稿任务
func (r *draftTaskRepo) FindExpired(ctx context.Context, before time.Time) ([]*model.DraftTask, error) {
	var tasks []*model.DraftTask
	err := r.db.WithContext(ctx).
		Where("created_at < ? AND status = ?", before, model.TaskStatusDraft).
		Find(&tasks).Error
	return tasks, err
}

// MarkExpired 标记任务为过期
func (r *draftTaskRepo) MarkExpired(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.DraftTask{}).
		Where("id = ?", id).
		Update("status", model.TaskStatusExpired).Error
}

// ==================== DraftProduct 仓储实现 ====================

type draftProductRepo struct {
	db *gorm.DB
}

// NewDraftProductRepository 创建草稿商品仓储
func NewDraftProductRepository(db *gorm.DB) DraftProductRepository {
	return &draftProductRepo{db: db}
}

func (r *draftProductRepo) Create(ctx context.Context, product *model.DraftProduct) error {
	return r.db.WithContext(ctx).Create(product).Error
}

func (r *draftProductRepo) CreateBatch(ctx context.Context, products []model.DraftProduct) error {
	if len(products) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&products).Error
}

func (r *draftProductRepo) GetByID(ctx context.Context, id int64) (*model.DraftProduct, error) {
	var product model.DraftProduct
	if err := r.db.WithContext(ctx).First(&product, id).Error; err != nil {
		return nil, err
	}
	return &product, nil
}

func (r *draftProductRepo) Update(ctx context.Context, product *model.DraftProduct) error {
	return r.db.WithContext(ctx).Save(product).Error
}

func (r *draftProductRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(&model.DraftProduct{}).Where("id = ?", id).Updates(fields).Error
}

func (r *draftProductRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.DraftProduct{}, id).Error
}

func (r *draftProductRepo) GetByTaskID(ctx context.Context, taskID int64) ([]model.DraftProduct, error) {
	var products []model.DraftProduct
	err := r.db.WithContext(ctx).Where("task_id = ?", taskID).Order("id ASC").Find(&products).Error
	return products, err
}

func (r *draftProductRepo) ConfirmAll(ctx context.Context, taskID int64) (int64, error) {
	result := r.db.WithContext(ctx).
		Model(&model.DraftProduct{}).
		Where("task_id = ? AND status = ?", taskID, model.DraftStatusDraft).
		Updates(map[string]interface{}{
			"status":      model.DraftStatusConfirmed,
			"sync_status": model.DraftSyncStatusPending,
		})
	return result.RowsAffected, result.Error
}

func (r *draftProductRepo) CountByTaskID(ctx context.Context, taskID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.DraftProduct{}).Where("task_id = ?", taskID).Count(&count).Error
	return count, err
}

// FindPendingSubmit 查找待提交的草稿商品
func (r *draftProductRepo) FindPendingSubmit(ctx context.Context, limit int) ([]*model.DraftProduct, error) {
	var products []*model.DraftProduct
	err := r.db.WithContext(ctx).
		Where("status = ? AND sync_status = ?", model.DraftStatusConfirmed, model.DraftSyncStatusPending).
		Limit(limit).
		Find(&products).Error
	return products, err
}

// UpdateSyncStatus 更新同步状态
func (r *draftProductRepo) UpdateSyncStatus(ctx context.Context, id int64, status int) error {
	return r.db.WithContext(ctx).
		Model(&model.DraftProduct{}).
		Where("id = ?", id).
		Update("sync_status", status).Error
}

// MarkSubmitted 标记为已提交
func (r *draftProductRepo) MarkSubmitted(ctx context.Context, id int64, listingID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.DraftProduct{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":      model.DraftStatusSubmitted,
			"sync_status": model.DraftSyncStatusDone,
			"listing_id":  listingID,
			"sync_error":  "",
		}).Error
}

// MarkFailed 标记为失败
func (r *draftProductRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&model.DraftProduct{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"sync_status": model.DraftSyncStatusFailed,
			"sync_error":  errMsg,
		}).Error
}

// UpdateProductID 更新关联的 Product ID
func (r *draftProductRepo) UpdateProductID(ctx context.Context, id int64, productID int64) error {
	return r.db.WithContext(ctx).
		Model(&model.DraftProduct{}).
		Where("id = ?", id).
		Update("product_id", productID).Error
}

// DeleteByTaskID 按任务ID删除
func (r *draftProductRepo) DeleteByTaskID(ctx context.Context, taskID int64) error {
	return r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Delete(&model.DraftProduct{}).Error
}

// ==================== DraftImage 仓储实现 ====================

type draftImageRepo struct {
	db *gorm.DB
}

// NewDraftImageRepository 创建草稿图片仓储
func NewDraftImageRepository(db *gorm.DB) DraftImageRepository {
	return &draftImageRepo{db: db}
}

func (r *draftImageRepo) Create(ctx context.Context, image *model.DraftImage) error {
	return r.db.WithContext(ctx).Create(image).Error
}

func (r *draftImageRepo) CreateBatch(ctx context.Context, images []model.DraftImage) error {
	if len(images) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&images).Error
}

func (r *draftImageRepo) GetByID(ctx context.Context, id int64) (*model.DraftImage, error) {
	var image model.DraftImage
	if err := r.db.WithContext(ctx).First(&image, id).Error; err != nil {
		return nil, err
	}
	return &image, nil
}

func (r *draftImageRepo) Update(ctx context.Context, image *model.DraftImage) error {
	return r.db.WithContext(ctx).Save(image).Error
}

func (r *draftImageRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.DraftImage{}, id).Error
}

func (r *draftImageRepo) GetByTaskID(ctx context.Context, taskID int64) ([]model.DraftImage, error) {
	var images []model.DraftImage
	err := r.db.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("group_index ASC, image_index ASC").
		Find(&images).Error
	return images, err
}

func (r *draftImageRepo) GetByGroup(ctx context.Context, taskID int64, groupIndex int) ([]model.DraftImage, error) {
	var images []model.DraftImage
	err := r.db.WithContext(ctx).
		Where("task_id = ? AND group_index = ?", taskID, groupIndex).
		Order("image_index ASC").
		Find(&images).Error
	return images, err
}

func (r *draftImageRepo) DeleteByTaskID(ctx context.Context, taskID int64) error {
	return r.db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&model.DraftImage{}).Error
}

func (r *draftImageRepo) DeleteByGroup(ctx context.Context, taskID int64, groupIndex int) error {
	return r.db.WithContext(ctx).
		Where("task_id = ? AND group_index = ?", taskID, groupIndex).
		Delete(&model.DraftImage{}).Error
}

// ==================== 事务支持 ====================

// DraftUnitOfWork 草稿工作单元（事务）
type DraftUnitOfWork struct {
	db       *gorm.DB
	Tasks    DraftTaskRepository
	Products DraftProductRepository
	Images   DraftImageRepository
}

// NewDraftUnitOfWork 创建工作单元
func NewDraftUnitOfWork(db *gorm.DB) *DraftUnitOfWork {
	return &DraftUnitOfWork{
		db:       db,
		Tasks:    NewDraftTaskRepository(db),
		Products: NewDraftProductRepository(db),
		Images:   NewDraftImageRepository(db),
	}
}

// Transaction 执行事务
func (u *DraftUnitOfWork) Transaction(ctx context.Context, fn func(uow *DraftUnitOfWork) error) error {
	return u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txUow := &DraftUnitOfWork{
			db:       tx,
			Tasks:    NewDraftTaskRepository(tx),
			Products: NewDraftProductRepository(tx),
			Images:   NewDraftImageRepository(tx),
		}
		return fn(txUow)
	})
}
