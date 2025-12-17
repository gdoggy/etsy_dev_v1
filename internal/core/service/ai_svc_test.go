package service

import (
	"context"
	"etsy_dev_v1_202512/internal/core/model"
	"etsy_dev_v1_202512/pkg/utils"
	"fmt"
	"testing"
	"time"
)

// 运行命令: go test -v internal/service/ai_svc_test.go -run TestVisionToPrompt
func TestVisionToPrompt(t *testing.T) {
	// 1. 配置 Key
	apiKey := "AIzaSyCa9PL-Q3goYFJ7O5QAPxKqkEmoGPKAx88"

	// 准备服务
	cfg := AIConfig{
		ApiKey:     apiKey,
		TextModel:  "gemini-2.5-flash",
		ImageModel: "imagen-4.0-generate-001",
		// VideoModel: "video-placeholder",
	}
	svc := NewAIService(cfg)

	// 2. 准备测试数据
	// 找一张比较复杂的白底产品图 URL (比如 Etsy 或 Amazon 上的)
	// 示例：一件红色针织毛衣
	testImgUrl := "https://i.etsystatic.com/10967397/r/il/2f909b/4068062060/il_794xN.4068062060_n2lc.jpg"
	// ⚠️ 注意：请换成您手里真实的、可以访问的图片 URL，否则 DownloadImage 会报错

	keyword := "Tiny Starburst Stud Earrings"
	ctx := context.Background()

	//var proxy *model.Proxy = nil
	proxy := &model.Proxy{IP: "127.0.0.1", Port: "7897"}

	// ---------------------------------------------------
	// 阶段一：测试“视觉分析”能力
	// ---------------------------------------------------
	fmt.Println("正在进行视觉分析 (Vision Analysis)...")
	start := time.Now()
	desc, err := svc.analyzeProductImage(ctx, proxy, keyword, testImgUrl)
	if err != nil {
		t.Fatalf("视觉分析失败: %v", err)
	}

	fmt.Printf("✅ 分析耗时: %v\n", time.Since(start))
	fmt.Printf("📝 AI生成的逆向Prompt:\n%s\n", desc)
	fmt.Println("---------------------------------------------------")

	// ---------------------------------------------------
	// 阶段二：手动调试图片生成 (直接打印原始响应)
	// ---------------------------------------------------
	fmt.Println("正在测试生成图片 (Debug Mode)...")

	// 1. 构造一个包含 Vision 结果的 Prompt
	fullPrompt := fmt.Sprintf("Professional product photography. %s", desc)

	// 2. 手动构建请求 (为了看清楚报错)
	client := utils.NewProxiedClient(proxy)
	// ⚠️ 尝试回退到 imagen-3.0-generate-001 试试，有时候 4.0 虽然在列表里但无法通过此 endpoint 访问
	// 或者先保持 4.0，看报错说啥
	targetModel := "imagen-4.0-generate-001"
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:predict?key=%s", targetModel, apiKey)

	payload := map[string]interface{}{
		"instances": []map[string]interface{}{
			{"prompt": fullPrompt},
		},
		"parameters": map[string]interface{}{
			"sampleCount": 1,
			"aspectRatio": "1:1",
		},
	}

	resp, err := client.R().
		SetBody(payload).
		Post(url)

	if err != nil {
		t.Fatalf("网络请求失败: %v", err)
	}

	fmt.Printf("🔴 HTTP Status: %d\n", resp.StatusCode())
	fmt.Printf("📜 Raw Response: %s\n", resp.String()) // <--- 这里会告诉我们真相

	if resp.StatusCode() != 200 {
		t.Fatal("图片生成 API 调用失败，请检查上面的 Raw Response")
	}
}
