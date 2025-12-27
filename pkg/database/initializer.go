package database

import (
	"context"
	"embed"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Initializer 数据库初始化器
type Initializer struct {
	db             *gorm.DB
	config         *PartitionConfig
	manager        *PartitionManager
	nonPartitioned []interface{}
	futureMonths   int
}

// InitOptions 初始化选项
type InitOptions struct {
	// 嵌入文件系统（推荐）
	EmbedFS   *embed.FS
	EmbedRoot string

	// 外部目录（可选，用于开发调试）
	SQLDir string

	// 非分区表 Model
	NonPartitionedModels []interface{}

	// 创建未来几个月的分区（默认 3）
	FutureMonths int
}

// NewInitializer 创建初始化器
func NewInitializer(db *gorm.DB, opts InitOptions) (*Initializer, error) {
	var config *PartitionConfig
	var err error

	// 加载配置
	if opts.EmbedFS != nil {
		config, err = LoadPartitionConfig(*opts.EmbedFS, opts.EmbedRoot)
	} else if opts.SQLDir != "" {
		config, err = LoadPartitionConfigFromDir(opts.SQLDir)
	} else {
		return nil, fmt.Errorf("必须指定 EmbedFS 或 SQLDir")
	}
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	if opts.FutureMonths == 0 {
		opts.FutureMonths = 3
	}

	return &Initializer{
		db:             db,
		config:         config,
		manager:        NewPartitionManager(db, config),
		nonPartitioned: opts.NonPartitionedModels,
		futureMonths:   opts.FutureMonths,
	}, nil
}

// Initialize 执行初始化
func (i *Initializer) Initialize(ctx context.Context) error {
	log.Println("[DB] 开始数据库初始化...")
	start := time.Now()

	// 1. 创建分区主表
	log.Println("[DB] 1/3 创建分区主表...")
	if err := i.manager.InitPartitionTables(ctx); err != nil {
		return fmt.Errorf("创建分区表失败: %w", err)
	}

	// 2. 创建分区
	log.Printf("[DB] 2/3 创建未来 %d 个月分区...", i.futureMonths)
	if err := i.manager.EnsureFuturePartitions(ctx, i.futureMonths); err != nil {
		return fmt.Errorf("创建分区失败: %w", err)
	}

	// 3. AutoMigrate 非分区表
	if len(i.nonPartitioned) > 0 {
		log.Printf("[DB] 3/3 AutoMigrate %d 个非分区表...", len(i.nonPartitioned))
		if err := i.db.WithContext(ctx).AutoMigrate(i.nonPartitioned...); err != nil {
			return fmt.Errorf("AutoMigrate 失败: %w", err)
		}
	}

	// 打印统计
	i.printStats(ctx)

	log.Printf("[DB] 初始化完成，耗时 %v", time.Since(start))
	return nil
}

func (i *Initializer) printStats(ctx context.Context) {
	stats, err := i.manager.GetAllStats(ctx)
	if err != nil {
		return
	}
	for _, s := range stats {
		log.Printf("[DB] %s: %d 分区, %.2f MB",
			s.TableName, s.PartitionCount, float64(s.TotalSizeBytes)/1024/1024)
	}
}

// GetManager 获取分区管理器
func (i *Initializer) GetManager() *PartitionManager {
	return i.manager
}

// GetConfig 获取配置
func (i *Initializer) GetConfig() *PartitionConfig {
	return i.config
}

// IsPartitionedTable 检查是否为分区表
func (i *Initializer) IsPartitionedTable(name string) bool {
	return i.config.IsPartitionedTable(name)
}

// ==================== 全局实例 ====================

var globalInit *Initializer

// SetGlobal 设置全局实例
func SetGlobal(init *Initializer) {
	globalInit = init
}

// Global 获取全局实例
func Global() *Initializer {
	return globalInit
}

// QuickInit 快速初始化
func QuickInit(db *gorm.DB, models []interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	init, err := NewInitializer(db, InitOptions{
		EmbedFS:              &PartitionSQL,
		EmbedRoot:            "partitions",
		NonPartitionedModels: models,
		FutureMonths:         3,
	})
	if err != nil {
		return err
	}

	SetGlobal(init)
	return init.Initialize(ctx)
}
