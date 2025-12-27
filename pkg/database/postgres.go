package database

import (
	"context"
	"etsy_dev_v1_202512/internal/model"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectDB 初始化数据库连接
// dsn: 数据库连接字符串
func ConnectDB() *gorm.DB {
	// 配置 GORM 的日志模式，开发环境下打印所有 SQL，方便调试
	dbLogger := logger.Default.LogMode(logger.Info)

	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	timezone := os.Getenv("TZ")
	if os.Getenv("APP_ENV") == "dev" {
		host = "localhost"
	}

	// 2. 拼接 DSN (Data Source Name)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=%s",
		host, user, password, dbname, port, timezone)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: dbLogger,
		NowFunc: func() time.Time {
			location, _ := time.LoadLocation(timezone)
			return time.Now().In(location)
		},
	})

	if err != nil {
		log.Fatalf("数据库连接失败 (Database Connection Failed): %v", err)
	}

	// 获取底层的 sqlDB 对象，用于设置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取底层 SQL DB 失败: %v", err)
	}

	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(10)
	// 设置打开数据库连接的最大数量
	sqlDB.SetMaxOpenConns(100)
	// 设置了连接可复用的最大时间
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 1. 确保启用 pg_trgm 扩展 (用于 GIN 索引模糊搜索)
	db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm;")

	log.Println("数据库连接成功 (Database Connected Successfully)")

	return db
}

// InitDatabase 初始化数据库
func InitDatabase() *gorm.DB {
	db := ConnectDB()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 非分区表列表
	nonPartitionedModels := []interface{}{
		// Manager
		&model.SysUser{}, &model.ShopMember{},
		// Account
		&model.Proxy{}, &model.Developer{}, &model.DomainPool{},
		// Shop
		&model.Shop{},
		// Shipping
		&model.ShippingProfile{}, &model.ShippingDestination{}, &model.ShippingUpgrade{}, &model.ReturnPolicy{},
		// Product
		&model.Product{}, &model.ProductImage{}, &model.ProductVariant{},
		// Draft
		&model.DraftTask{}, &model.DraftProduct{}, &model.DraftImage{},
		// 注意：以下表已分区，不在此处
		// - Order, OrderItem
		// - Shipment, TrackingEvent
		// - AICallLog
	}

	// 使用嵌入的 SQL 文件初始化
	init, err := NewInitializer(db, InitOptions{
		EmbedFS:              &PartitionSQL,
		EmbedRoot:            "partitions",
		NonPartitionedModels: nonPartitionedModels,
		FutureMonths:         3,
	})
	if err != nil {
		log.Fatalf("创建数据库初始化器失败: %v", err)
	}

	// 设置全局实例
	SetGlobal(init)
	// 执行初始化
	if err := init.Initialize(ctx); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	return db
}
