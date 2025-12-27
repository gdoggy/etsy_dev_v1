package database

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// PartitionManager 分区管理器
type PartitionManager struct {
	db     *gorm.DB
	config *PartitionConfig
}

// NewPartitionManager 创建分区管理器
func NewPartitionManager(db *gorm.DB, config *PartitionConfig) *PartitionManager {
	return &PartitionManager{db: db, config: config}
}

// ==================== 初始化 ====================

// InitPartitionTables 初始化分区主表
func (m *PartitionManager) InitPartitionTables(ctx context.Context) error {
	for _, table := range m.config.Tables {
		exists, err := m.tableExists(ctx, table.TableName)
		if err != nil {
			return fmt.Errorf("检查表 %s 失败: %w", table.TableName, err)
		}

		if exists {
			log.Printf("[Partition] 表 %s 已存在", table.TableName)
			continue
		}

		log.Printf("[Partition] 创建分区表 %s ...", table.TableName)
		if err := m.db.WithContext(ctx).Exec(table.SQLContent).Error; err != nil {
			return fmt.Errorf("创建表 %s 失败: %w", table.TableName, err)
		}
		log.Printf("[Partition] 表 %s 创建成功", table.TableName)
	}
	return nil
}

func (m *PartitionManager) tableExists(ctx context.Context, tableName string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM pg_tables 
		WHERE schemaname = 'public' AND tablename = ?
	`, tableName).Scan(&count).Error
	return count > 0, err
}

// ==================== 分区创建 ====================

// EnsureFuturePartitions 确保未来 N 个月的分区存在
func (m *PartitionManager) EnsureFuturePartitions(ctx context.Context, monthsAhead int) error {
	now := time.Now()
	for i := 0; i <= monthsAhead; i++ {
		targetMonth := now.AddDate(0, i, 0)
		for _, table := range m.config.Tables {
			if err := m.createPartitionIfNotExists(ctx, table.TableName, targetMonth); err != nil {
				log.Printf("[Partition] 创建 %s 分区失败: %v", table.TableName, err)
			}
		}
	}
	return nil
}

// createPartitionIfNotExists 创建分区（如不存在）
func (m *PartitionManager) createPartitionIfNotExists(ctx context.Context, tableName string, month time.Time) error {
	startDate := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(0, 1, 0)
	partitionName := fmt.Sprintf("%s_y%dm%02d", tableName, startDate.Year(), startDate.Month())

	exists, err := m.partitionExists(ctx, partitionName)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	sql := fmt.Sprintf(
		`CREATE TABLE %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')`,
		partitionName, tableName,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"),
	)

	if err := m.db.WithContext(ctx).Exec(sql).Error; err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("创建分区 %s 失败: %w", partitionName, err)
	}

	log.Printf("[Partition] 创建分区 %s", partitionName)
	return nil
}

func (m *PartitionManager) partitionExists(ctx context.Context, partitionName string) (bool, error) {
	var count int64
	err := m.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM pg_tables 
		WHERE schemaname = 'public' AND tablename = ?
	`, partitionName).Scan(&count).Error
	return count > 0, err
}

// ==================== 分区清理 ====================

// CleanupExpiredPartitions 清理过期分区
func (m *PartitionManager) CleanupExpiredPartitions(ctx context.Context) (int, error) {
	dropped := 0
	for _, table := range m.config.Tables {
		if table.RetentionMonth == 0 {
			continue // 永久保留
		}

		cutoff := time.Now().AddDate(0, -table.RetentionMonth, 0)
		cutoff = time.Date(cutoff.Year(), cutoff.Month(), 1, 0, 0, 0, 0, time.UTC)

		count, err := m.dropPartitionsBefore(ctx, table.TableName, cutoff)
		if err != nil {
			log.Printf("[Partition] 清理 %s 过期分区失败: %v", table.TableName, err)
		}
		dropped += count
	}
	return dropped, nil
}

func (m *PartitionManager) dropPartitionsBefore(ctx context.Context, tableName string, before time.Time) (int, error) {
	partitions, err := m.ListPartitions(ctx, tableName)
	if err != nil {
		return 0, err
	}

	dropped := 0
	for _, p := range partitions {
		partMonth, err := m.parsePartitionMonth(p.Name, tableName)
		if err != nil {
			continue
		}

		if partMonth.Before(before) {
			log.Printf("[Partition] 删除过期分区 %s", p.Name)
			if err := m.db.WithContext(ctx).Exec(
				fmt.Sprintf("DROP TABLE IF EXISTS %s", p.Name),
			).Error; err != nil {
				log.Printf("[Partition] 删除 %s 失败: %v", p.Name, err)
			} else {
				dropped++
			}
		}
	}
	return dropped, nil
}

func (m *PartitionManager) parsePartitionMonth(partitionName, tableName string) (time.Time, error) {
	suffix := strings.TrimPrefix(partitionName, tableName+"_y")
	if len(suffix) < 6 {
		return time.Time{}, fmt.Errorf("无效分区名")
	}
	var year, month int
	if _, err := fmt.Sscanf(suffix, "%dm%d", &year, &month); err != nil {
		return time.Time{}, err
	}
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
}

// ==================== 分区查询 ====================

// PartitionInfo 分区信息
type PartitionInfo struct {
	Name      string `gorm:"column:partition_name"`
	Range     string `gorm:"column:partition_range"`
	SizeBytes int64  `gorm:"column:size_bytes"`
}

// ListPartitions 列出表的所有分区
func (m *PartitionManager) ListPartitions(ctx context.Context, tableName string) ([]PartitionInfo, error) {
	var partitions []PartitionInfo
	err := m.db.WithContext(ctx).Raw(`
		SELECT 
			child.relname AS partition_name,
			pg_get_expr(child.relpartbound, child.oid) AS partition_range,
			pg_total_relation_size(child.oid) AS size_bytes
		FROM pg_inherits
		JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child ON pg_inherits.inhrelid = child.oid
		WHERE parent.relname = ?
		ORDER BY child.relname
	`, tableName).Scan(&partitions).Error
	return partitions, err
}

// TableStats 表统计
type TableStats struct {
	TableName      string `gorm:"column:table_name"`
	PartitionCount int    `gorm:"column:partition_count"`
	TotalSizeBytes int64  `gorm:"column:total_size_bytes"`
}

// GetAllStats 获取所有分区表统计
func (m *PartitionManager) GetAllStats(ctx context.Context) ([]TableStats, error) {
	var stats []TableStats
	tableNames := m.config.GetTableNames()

	if len(tableNames) == 0 {
		return stats, nil
	}

	// 构建占位符和参数
	placeholders := make([]string, len(tableNames))
	args := make([]interface{}, len(tableNames))
	for i, name := range tableNames {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = name
	}

	query := fmt.Sprintf(`
		SELECT 
			parent.relname AS table_name,
			COUNT(child.relname) AS partition_count,
			COALESCE(SUM(pg_total_relation_size(child.oid)), 0) AS total_size_bytes
		FROM pg_inherits
		JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
		JOIN pg_class child ON pg_inherits.inhrelid = child.oid
		WHERE parent.relname IN (%s)
		GROUP BY parent.relname
		ORDER BY parent.relname
	`, strings.Join(placeholders, ","))

	err := m.db.WithContext(ctx).Raw(query, args...).Scan(&stats).Error
	return stats, err
}

// HealthCheck 分区健康检查
func (m *PartitionManager) HealthCheck(ctx context.Context) error {
	now := time.Now()
	current := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	next := current.AddDate(0, 1, 0)

	var missing []string
	for _, table := range m.config.Tables {
		for _, month := range []time.Time{current, next} {
			name := fmt.Sprintf("%s_y%dm%02d", table.TableName, month.Year(), month.Month())
			exists, _ := m.partitionExists(ctx, name)
			if !exists {
				missing = append(missing, name)
			}
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("缺失分区: %v", missing)
	}
	return nil
}
