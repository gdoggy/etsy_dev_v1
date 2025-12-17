package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// 运行命令: go test -v internal/service/check_models_test.go -run TestListAvailableModels
func TestListAvailableModels(t *testing.T) {
	// ⚠️ 必须填入您的真实 API Key
	apiKey := "AIzaSyCa9PL-Q3goYFJ7O5QAPxKqkEmoGPKAx88"

	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("网络请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// 简单的结构体用于解析
	type ModelInfo struct {
		Name         string   `json:"name"`
		DisplayName  string   `json:"displayName"`
		SupportedGen []string `json:"supportedGenerationMethods"`
	}
	var result struct {
		Models []ModelInfo `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		t.Logf("原始响应: %s", string(body))
		t.Fatalf("JSON解析失败: %v", err)
	}

	fmt.Println("\n================= 您的账号可用模型列表 =================")
	fmt.Printf("%-30s | %-20s | %s\n", "模型ID (Model Name)", "显示名称", "能力 (简略)")
	fmt.Println(strings.Repeat("-", 80))

	for _, m := range result.Models {
		// 过滤掉只能 embedding 的模型，只看生成的
		isGen := false
		for _, method := range m.SupportedGen {
			if method == "generateContent" || method == "predict" {
				isGen = true
				break
			}
		}
		if !isGen {
			continue
		}

		// 简化 ID (去掉 models/ 前缀)
		shortName := strings.Replace(m.Name, "models/", "", 1)
		fmt.Printf("%-30s | %-20s | %v\n", shortName, m.DisplayName, m.SupportedGen)
	}
}
