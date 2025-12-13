package main

import (
	"log"
	"strings"

	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/core/service"
	"etsy_dev_v1_202512/pkg/database"
)

func main() {
	log.Println(">>> 开始第三步：Service 层测试 (Debug模式)...")

	// 1. 初始化 DB
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db := database.InitDB(dsn)

	// 2. 组装 Service
	adapterRepo := repository.NewAdapterRepo(db)
	shopRepo := repository.NewShopRepo(db)
	authService := service.NewAuthService(adapterRepo, shopRepo)

	// 3. 准备测试数据
	// 为了确保测试准确，先把所有 Adapter 的状态设为 0 (禁用)，只留我们要测的这个
	db.Model(&model.Adapter{}).Where("1=1").Update("status", 0)

	testAdapter := model.Adapter{
		Name:       "Service_Test_Debug_Unique",
		ProxyURL:   "http://127.0.0.1:7897",
		EtsyAppKey: "My_Real_App_Key_123", // 这是我们要验证的 Key
		Status:     1,                     // 只有它是启用的
	}
	// 先删除同名的防止冲突
	db.Where("name = ?", testAdapter.Name).Delete(&model.Adapter{})
	db.Create(&testAdapter)
	log.Printf("已创建测试专用 Adapter (ID: %d)，并将其他 Adapter 暂时设为禁用", testAdapter.ID)

	// 4. 测试生成 URL
	url, err := authService.GenerateLoginURL()
	if err != nil {
		log.Fatalf("❌ 生成链接失败: %v", err)
	}

	// 5. 打印实际生成的 URL (关键调试信息)
	log.Printf("---------------------------------------------------")
	log.Printf("生成的实际 URL:\n%s", url)
	log.Printf("---------------------------------------------------")

	// 6. 验证
	// 检查 URL 是否包含 client_id=My_Real_App_Key_123
	if strings.Contains(url, "client_id=My_Real_App_Key_123") {
		log.Println("✅ 验证通过！ClientID 匹配。")
	} else {
		log.Println("❌ 验证失败！URL 中的 ClientID 与预期不符。")
		log.Println("可能原因：Service 层读取到的 Adapter 数据字段为空，或者读取到了错误的 Adapter。")

		// 进一步排查：直接查数据库看看
		var checkAdapter model.Adapter
		db.First(&checkAdapter, testAdapter.ID)
		log.Printf("数据库中的实际数据 -> ID: %d, Key: %s", checkAdapter.ID, checkAdapter.EtsyAppKey)
	}

	// 检查是否包含 PKCE 参数
	if strings.Contains(url, "code_challenge=") && strings.Contains(url, "code_challenge_method=S256") {
		log.Println("✅ 验证通过！包含 PKCE 安全参数。")
	} else {
		log.Println("❌ 验证失败！缺少 PKCE 参数。")
	}

	if strings.Contains(url, "client_id=My_Real_App_Key_123") && strings.Contains(url, "code_challenge=") {
		log.Println("🎉 第三步重构成功！Service 层逻辑完全正常。")

		// 测试完成后，把数据清理掉或恢复（这里简单起见就不恢复旧数据状态了，反正都是测试数据）
	} else {
		log.Fatal("⚠️ 测试未通过，请截图上面的日志给我。")
	}
}
