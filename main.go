package main

import (
	"fmt"
	"log"
	"time"

	"github.com/go-resty/resty/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// 1. 定义与数据库表对应的结构体
type Adapter struct {
	ID         uint
	Name       string
	ProxyURL   string
	EtsyAppKey string
	Status     int
}

func main() {
	fmt.Println(">>> 开始执行全链路测试...")

	// ------------------------------------------------
	// 2. 连接数据库
	// ------------------------------------------------
	dsn := "host=localhost user=etsy_admin password=1234 dbname=etsy_farm port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ 数据库连接失败: %v", err)
	}
	fmt.Println("✅ 数据库连接成功！")

	// ------------------------------------------------
	// 3. 从数据库读取配置
	// ------------------------------------------------
	var adapter Adapter
	// 查找名字为 Local_Mac_Dev 的配置
	result := db.Where("name = ?", "Local_Mac_Dev").First(&adapter)
	if result.Error != nil {
		log.Fatalf("❌ 未找到 Adapter 配置，请检查数据库是否已插入数据: %v", result.Error)
	}
	fmt.Printf("✅ 读取配置成功: [Name: %s] [Proxy: %s] [Key长度: %d]\n",
		adapter.Name, adapter.ProxyURL, len(adapter.EtsyAppKey))

	// ------------------------------------------------
	// 4. 发起 Etsy API 请求 (Ping)
	// ------------------------------------------------
	client := resty.New()

	// 设置超时和重试，防止网络波动
	client.SetTimeout(10 * time.Second)
	client.SetRetryCount(3)

	// 关键：设置代理
	client.SetProxy(adapter.ProxyURL)

	// 关键：设置 API Key (Etsy 要求 Header 中必须带 x-api-key)
	client.SetHeader("x-api-key", adapter.EtsyAppKey)

	fmt.Println(">>> 正在向 Etsy 发起 Ping 请求...")

	// 请求 Etsy 的公共健康检查接口
	resp, err := client.R().Get("https://api.etsy.com/v3/application/openapi-ping")

	// ------------------------------------------------
	// 5. 结果验证
	// ------------------------------------------------
	if err != nil {
		log.Fatalf("❌ 请求失败 (可能是代理不通): %v", err)
	}

	if resp.StatusCode() == 200 {
		fmt.Println("🎉🎉🎉 测试成功！全链路已打通！")
		fmt.Printf("Etsy 响应: %s\n", resp.String())
	} else {
		fmt.Printf("⚠️ 连接通了，但 Etsy 拒绝了请求 (状态码 %d)\n", resp.StatusCode())
		fmt.Printf("错误信息: %s\n", resp.String())
		fmt.Println("提示: 如果是 403，通常是 API Key 填错了；如果是 429，是请求太快了。")
	}
}
