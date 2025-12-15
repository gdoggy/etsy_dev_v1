package service

import (
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/core/model"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// GeneratedContent 定义 AI 返回的结构化数据
// 将其公开，以便 Controller 层引用
type GeneratedContent struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
type AIService struct {
	ApiKey       string
	ModelVersion string // 支持配置，如 "gemini-2.5-flash"
}

// NewAIService 支持传入模型版本
func NewAIService(apiKey string, modelVersion string) *AIService {
	if modelVersion == "" {
		modelVersion = "gemini-2.5-flash" // 2025年默认用最新的 Flash
	}
	return &AIService{
		ApiKey:       apiKey,
		ModelVersion: modelVersion,
	}
}

// GenerateProductInfo 生成逻辑
// extraInstruction: 允许用户传入额外的 Prompt 指令，例如 "Use emojis, be funny" 或 "Focus on SEO"
func (s *AIService) GenerateProductInfo(ctx context.Context, proxy *model.Proxy, keywords string, extraInstruction string) (*GeneratedContent, error) {
	// 1. 智能构建 HTTP Client
	var httpClient *http.Client

	// 只有当 proxy 不为空，且包含有效 IP 时，才配置 Transport
	// 在 AWS US 环境下，传入 nil proxy 即可实现直连
	if proxy != nil && proxy.IP != "" {
		proxyURLStr := proxy.ProxyURL()
		if proxyURLStr != "" {
			proxyURL, err := url.Parse(proxyURLStr)
			if err == nil {
				httpClient = &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxyURL),
					},
				}
				fmt.Println("AI Service: 使用代理模式")
			}
		}
	}

	// 如果 httpClient 为 nil，option.WithHTTPClient 会被忽略或使用默认，实现直连
	opts := []option.ClientOption{option.WithAPIKey(s.ApiKey)}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	client, err := genai.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("Gemini 初始化失败: %v", err)
	}
	defer client.Close()

	// 2. 使用配置的模型
	modelAI := client.GenerativeModel(s.ModelVersion)
	modelAI.ResponseMIMEType = "application/json"

	// 优化的 Prompt 构建逻辑
	basePrompt := fmt.Sprintf(`
        You are an SEO expert for Etsy. 
        Generate a listing based on these keywords/features: "%s".
        
        Requirements:
        1. Title: SEO friendly, max 140 chars.
        2. Description: Engaging, sales-oriented.
        3. Tags: 13 comma-separated keywords.
    `, keywords)

	// 如果有额外指令，追加进去
	if extraInstruction != "" {
		basePrompt += fmt.Sprintf("\nAdditional User Instructions: %s", extraInstruction)
	}

	basePrompt += `
        Output Schema (JSON):
        {
            "title": "string",
            "description": "string",
            "tags": ["string", "string"]
        }
    `

	// 3. 发送请求
	resp, err := modelAI.GenerateContent(ctx, genai.Text(basePrompt))
	if err != nil {
		return nil, fmt.Errorf("AI 生成失败: %v", err)
	}

	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return nil, fmt.Errorf("AI 返回为空")
	}

	// 4. 解析结果
	// Gemini 返回的是 Parts，通常第一个 Part 是文本
	var rawJSON string
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			rawJSON = string(txt)
			break
		}
	}

	// 清洗一下可能存在的 markdown 符号 (```json ... ```)
	rawJSON = strings.TrimPrefix(rawJSON, "```json")
	rawJSON = strings.TrimPrefix(rawJSON, "```")
	rawJSON = strings.TrimSuffix(rawJSON, "```")

	var result GeneratedContent
	if err := json.Unmarshal([]byte(rawJSON), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v | 原始数据: %s", err, rawJSON)
	}

	return &result, nil
}
