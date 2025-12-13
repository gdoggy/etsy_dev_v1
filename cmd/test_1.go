package main

import (
	"log"
	// 请注意：这里的 module 名字必须和你的 go.mod 第一行保持一致
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/pkg/database"
)

func main() {
	log.Println(">>> 开始第一步重构测试...")

	// 1. 测试数据库连接模块
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	// 2. 测试模型迁移 (看看 entity.go 写得对不对)
	log.Println(">>> 正在验证表结构...")
	err := db.AutoMigrate(&model.Adapter{}, &model.Shop{})
	if err != nil {
		log.Fatalf("❌ 模型定义有误: %v", err)
	}

	log.Println("🎉 第一步重构成功！Model 和 Database 模块工作正常。")
}
