package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"etsy_dev_v1_202512/internal/repository"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ==================== 配置 ====================

// AIConfig AI 服务配置
type AIConfig struct {
	ApiKey     string
	TextModel  string
	ImageModel string
}

// ==================== 服务 ====================

type AIService struct {
	Config      *AIConfig
	Storage     *StorageService
	callLogRepo repository.AICallLogRepository
}

// NewAIService 创建 AI 服务
func NewAIService(cfg *AIConfig, storage *StorageService, callLogRepo repository.AICallLogRepository) *AIService {
	// 固定模型配置
	if cfg.TextModel == "" {
		cfg.TextModel = "gemini-3-flash"
	}
	if cfg.ImageModel == "" {
		cfg.ImageModel = "gemini-3-pro-image-preview-2k"
	}

	return &AIService{
		Config:      cfg,
		Storage:     storage,
		callLogRepo: callLogRepo,
	}
}

// ==================== 文案生成 ====================

// TextGenerateResult 文案生成结果
type TextGenerateResult struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// GenerateProductContent 根据商品标题生成 Etsy 文案
func (s *AIService) GenerateProductContent(ctx context.Context, productTitle, styleHint string) (*TextGenerateResult, error) {
	if s.Config.ApiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 未配置")
	}

	prompt := fmt.Sprintf(`You are an Etsy SEO expert. Generate optimized listing content for:

Product: %s
Style Hint: %s

Requirements:
1. Title: SEO optimized, max 140 characters, include high-traffic keywords
2. Description: Engaging sales copy, 200-400 words, highlight features and benefits
3. Tags: 13 relevant Etsy tags for search visibility

Output Format (JSON only, no markdown):
{
  "title": "Your SEO Title Here",
  "description": "Your engaging description here...",
  "tags": ["tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10", "tag11", "tag12", "tag13"]
}`, productTitle, styleHint)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		s.Config.TextModel, s.Config.ApiKey)

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": []map[string]interface{}{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Gemini API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("无生成结果")
	}

	// 提取 JSON 文本
	var jsonText string
	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				jsonText = part.Text
				break
			}
		}
	}

	// 解析生成结果
	var result TextGenerateResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("解析生成结果失败: %v, raw: %s", err, jsonText)
	}

	//startTime := time.Now()
	//
	//// 记录调用日志
	//s.callLogRepo.Create(ctx, &model.AICallLog{
	//	ShopID:     shopID,  // 从 ctx 或参数传入
	//	TaskID:     taskID,
	//	CallType:   model.AICallTypeImage,
	//	ModelName:  s.Config.ImageModel,
	//	ImageCount: len(images),
	//	DurationMs: time.Since(startTime).Milliseconds(),
	//	CostUSD:    calculateImageCost(len(images)),
	//	Status:     model.AICallStatusSuccess,
	//})

	return &result, nil
}

// ==================== 图片生成 ====================

// GenerateImages 调用 Gemini 多模态能力生成图片
// 返回 Base64 编码的图片数据
func (s *AIService) GenerateImages(ctx context.Context, prompt, referenceImageURL string, count int) ([]string, error) {
	if s.Config.ApiKey == "" {
		return nil, fmt.Errorf("Gemini API Key 未配置")
	}

	// 下载参考图片
	var referenceImageData []byte
	var referenceImageMimeType string
	if referenceImageURL != "" {
		data, mimeType, err := downloadImageData(ctx, referenceImageURL)
		if err != nil {
			fmt.Printf("下载参考图片失败: %v, 继续使用纯文本生成\n", err)
		} else {
			referenceImageData = data
			referenceImageMimeType = mimeType
		}
	}

	// 构建提示词
	fullPrompt := fmt.Sprintf(`You are a professional product photographer. 
Generate a high-quality product image based on the following description:

%s

Requirements:
- Professional studio lighting
- Clean, appealing composition  
- High resolution, suitable for e-commerce
- Focus on product details and quality`, prompt)

	// 调用 Gemini API 生成图片
	images := make([]string, 0, count)

	for i := 0; i < count; i++ {
		imageData, err := s.callGeminiImageGeneration(ctx, fullPrompt, referenceImageData, referenceImageMimeType)
		if err != nil {
			fmt.Printf("生成第 %d 张图片失败: %v\n", i+1, err)
			continue
		}
		images = append(images, imageData)

		// 避免请求过快
		if i < count-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("所有图片生成均失败")
	}

	return images, nil
}

// callGeminiImageGeneration 调用 Gemini 图片生成API
func (s *AIService) callGeminiImageGeneration(ctx context.Context, prompt string, referenceImage []byte, mimeType string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		s.Config.ImageModel, s.Config.ApiKey)

	// 构建请求体
	parts := []map[string]interface{}{
		{"text": prompt},
	}

	// 如果有参考图片，添加到请求中
	if len(referenceImage) > 0 {
		parts = append(parts, map[string]interface{}{
			"inline_data": map[string]interface{}{
				"mime_type": mimeType,
				"data":      base64.StdEncoding.EncodeToString(referenceImage),
			},
		})
	}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"parts": parts},
		},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"TEXT", "IMAGE"},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	// 解析响应，提取生成的图片
	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text       string `json:"text,omitempty"`
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData,omitempty"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}

	if geminiResp.Error != nil {
		return "", fmt.Errorf("API错误: %s", geminiResp.Error.Message)
	}

	// 查找图片数据
	for _, candidate := range geminiResp.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				return part.InlineData.Data, nil
			}
		}
	}

	return "", fmt.Errorf("响应中未找到图片数据")
}

// ==================== Imagen API (备选方案) ====================

// GenerateImagesWithImagen 使用 Imagen 生成图片
func (s *AIService) GenerateImagesWithImagen(ctx context.Context, prompt string, count int) ([]string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/imagen-3.0-generate-002:predict?key=%s",
		s.Config.ApiKey)

	reqBody := map[string]interface{}{
		"instances": []map[string]interface{}{
			{"prompt": prompt},
		},
		"parameters": map[string]interface{}{
			"sampleCount": count,
			"aspectRatio": "1:1",
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Imagen API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	var imagenResp struct {
		Predictions []struct {
			BytesBase64Encoded string `json:"bytesBase64Encoded"`
			MimeType           string `json:"mimeType"`
		} `json:"predictions"`
	}

	if err := json.Unmarshal(respBody, &imagenResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	images := make([]string, 0, len(imagenResp.Predictions))
	for _, pred := range imagenResp.Predictions {
		if pred.BytesBase64Encoded != "" {
			images = append(images, pred.BytesBase64Encoded)
		}
	}

	return images, nil
}

// ==================== 图片上传辅助 ====================

// GenerateAndUploadImages 生成图片并上传到云存储
func (s *AIService) GenerateAndUploadImages(ctx context.Context, prompt, referenceImageURL string, count int, prefix string) ([]string, error) {
	if s.Storage == nil {
		return nil, fmt.Errorf("StorageService 未配置")
	}

	// 生成图片
	base64Images, err := s.GenerateImages(ctx, prompt, referenceImageURL, count)
	if err != nil {
		return nil, err
	}

	// 上传到云存储
	urls := make([]string, 0, len(base64Images))
	for i, imgData := range base64Images {
		url, err := s.Storage.SaveBase64(imgData, fmt.Sprintf("%s_%d", prefix, i))
		if err != nil {
			fmt.Printf("上传第 %d 张图片失败: %v\n", i+1, err)
			continue
		}
		urls = append(urls, url)
	}

	return urls, nil
}

// ==================== 工具函数 ====================

func downloadImageData(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取失败: %v", err)
	}

	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = http.DetectContentType(data)
	}

	return data, mimeType, nil
}
