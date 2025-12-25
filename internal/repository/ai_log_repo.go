package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== 仓储接口 ====================

// AICallLogRepository AI调用日志仓储接口
type AICallLogRepository interface {
	Create(ctx context.Context, log *model.AICallLog) error
	GetByID(ctx context.Context, id int64) (*model.AICallLog, error)

	// 统计查询
	GetUsageByShop(ctx context.Context, shopID int64, startTime, endTime time.Time) (*AIUsageStats, error)
	GetUsageByTask(ctx context.Context, taskID int64) (*AIUsageStats, error)
	GetDailyUsage(ctx context.Context, startDate, endDate time.Time) ([]DailyUsageStats, error)
	GetTotalCost(ctx context.Context, startTime, endTime time.Time) (float64, error)
}

// ==================== 统计结构 ====================

// AIUsageStats AI用量统计
type AIUsageStats struct {
	TotalCalls        int64   `json:"total_calls"`
	TextCalls         int64   `json:"text_calls"`
	ImageCalls        int64   `json:"image_calls"`
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
	TotalImages       int64   `json:"total_images"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
	SuccessCount      int64   `json:"success_count"`
	FailedCount       int64   `json:"failed_count"`
}

// DailyUsageStats 每日用量统计
type DailyUsageStats struct {
	Date              string  `json:"date"`
	TotalCalls        int64   `json:"total_calls"`
	TotalCostUSD      float64 `json:"total_cost_usd"`
	TotalInputTokens  int64   `json:"total_input_tokens"`
	TotalOutputTokens int64   `json:"total_output_tokens"`
}

// ==================== 仓储实现 ====================

type aiCallLogRepo struct {
	db *gorm.DB
}

// NewAICallLogRepository 创建AI调用日志仓储
func NewAICallLogRepository(db *gorm.DB) AICallLogRepository {
	return &aiCallLogRepo{db: db}
}

func (r *aiCallLogRepo) Create(ctx context.Context, log *model.AICallLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *aiCallLogRepo) GetByID(ctx context.Context, id int64) (*model.AICallLog, error) {
	var log model.AICallLog
	if err := r.db.WithContext(ctx).First(&log, id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *aiCallLogRepo) GetUsageByShop(ctx context.Context, shopID int64, startTime, endTime time.Time) (*AIUsageStats, error) {
	var stats AIUsageStats

	query := r.db.WithContext(ctx).Model(&model.AICallLog{}).Where("shop_id = ?", shopID)
	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		query = query.Where("created_at <= ?", endTime)
	}

	err := query.Select(`
		COUNT(*) as total_calls,
		SUM(CASE WHEN call_type = 'text' THEN 1 ELSE 0 END) as text_calls,
		SUM(CASE WHEN call_type = 'image' THEN 1 ELSE 0 END) as image_calls,
		COALESCE(SUM(input_tokens), 0) as total_input_tokens,
		COALESCE(SUM(output_tokens), 0) as total_output_tokens,
		COALESCE(SUM(image_count), 0) as total_images,
		COALESCE(SUM(cost_usd), 0) as total_cost_usd,
		COALESCE(AVG(duration_ms), 0) as avg_duration_ms,
		SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
		SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
	`).Scan(&stats).Error

	return &stats, err
}

func (r *aiCallLogRepo) GetUsageByTask(ctx context.Context, taskID int64) (*AIUsageStats, error) {
	var stats AIUsageStats

	err := r.db.WithContext(ctx).Model(&model.AICallLog{}).
		Where("task_id = ?", taskID).
		Select(`
			COUNT(*) as total_calls,
			SUM(CASE WHEN call_type = 'text' THEN 1 ELSE 0 END) as text_calls,
			SUM(CASE WHEN call_type = 'image' THEN 1 ELSE 0 END) as image_calls,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens,
			COALESCE(SUM(image_count), 0) as total_images,
			COALESCE(SUM(cost_usd), 0) as total_cost_usd,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed_count
		`).Scan(&stats).Error

	return &stats, err
}

func (r *aiCallLogRepo) GetDailyUsage(ctx context.Context, startDate, endDate time.Time) ([]DailyUsageStats, error) {
	var stats []DailyUsageStats

	err := r.db.WithContext(ctx).Model(&model.AICallLog{}).
		Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Select(`
			DATE(created_at) as date,
			COUNT(*) as total_calls,
			COALESCE(SUM(cost_usd), 0) as total_cost_usd,
			COALESCE(SUM(input_tokens), 0) as total_input_tokens,
			COALESCE(SUM(output_tokens), 0) as total_output_tokens
		`).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&stats).Error

	return stats, err
}

func (r *aiCallLogRepo) GetTotalCost(ctx context.Context, startTime, endTime time.Time) (float64, error) {
	var totalCost float64

	query := r.db.WithContext(ctx).Model(&model.AICallLog{})
	if !startTime.IsZero() {
		query = query.Where("created_at >= ?", startTime)
	}
	if !endTime.IsZero() {
		query = query.Where("created_at <= ?", endTime)
	}

	err := query.Select("COALESCE(SUM(cost_usd), 0)").Scan(&totalCost).Error
	return totalCost, err
}
