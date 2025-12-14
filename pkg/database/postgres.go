package database

import (
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 初始化数据库连接
// dsn: 数据库连接字符串
// models: 需要自动建表/迁移的结构体指针
func InitDB(dsn string, models ...interface{}) *gorm.DB {
	// 配置 GORM 的日志模式，开发环境下打印所有 SQL，方便调试
	dbLogger := logger.Default.LogMode(logger.Info)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: dbLogger,
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

	log.Println("数据库连接成功 (Database Connected Successfully)")

	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			log.Fatalf("自动建表出错： %v", err)
		}
	}

	return db
}
