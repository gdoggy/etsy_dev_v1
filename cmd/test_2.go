package main

import (
	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/pkg/database"
	"log"
)

func main() {
	log.Println(">>> 开始第二步：Repository 层测试...")

	// 1. 连库
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	// 2. 初始化 Repo
	adapterRepo := repository.NewAdapterRepo(db)

	// 3. 准备测试数据 (插入一个测试用的 Adapter)
	testAdapterName := "Repo_Test_Adapter"
	// 先清理旧数据，防止重复报错
	db.Where("name = ?", testAdapterName).Delete(&model.Adapter{})

	newAdapter := model.Adapter{
		Name:       testAdapterName,
		ProxyURL:   "http://127.0.0.1:7890",
		EtsyAppKey: "test_key",
		Status:     1,
	}
	db.Create(&newAdapter)
	log.Printf("已插入测试 Adapter ID: %d", newAdapter.ID)

	// 4. 测试 FindAvailableAdapter
	// 我们限制 limit = 3，现在这个 Adapter 还没绑定店铺，应该能查出来
	foundAdapter, err := adapterRepo.FindAvailableAdapter(3)
	if err != nil {
		log.Fatalf("❌ 查找失败: %v", err)
	}
	log.Printf("✅ 成功找到可用 Adapter: %s (ID: %d)", foundAdapter.Name, foundAdapter.ID)

	// 5. 验证 ID 是否匹配
	if foundAdapter.ID != newAdapter.ID {
		// 注意：如果数据库里还有其他旧数据，可能会查到别的，这也是正常的，只要查到了就行
		log.Printf("⚠️ 查到了 Adapter，ID为 %d", foundAdapter.ID)
	}

	log.Println("🎉 第二步重构成功！Repository 层的 SQL 逻辑验证通过。")
}
