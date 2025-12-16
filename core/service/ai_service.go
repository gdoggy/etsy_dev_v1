package service

import (
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/pkg/utils"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	//"golang.org/x/sync/errgroup" // 需要 go get golang.org/x/sync
	"google.golang.org/api/option"

	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/view"
)

// AIConfig AI 服务配置 (支持前端/配置文件动态传入)
type AIConfig struct {
	ApiKey     string
	TextModel  string
	ImageModel string
	VideoModel string
}

type AIService struct {
	Config  AIConfig
	Storage *StorageService
}

func NewAIService(cfg AIConfig) *AIService {
	// 默认配置 2025.12 免费 API 最强模型
	if cfg.TextModel == "" {
		cfg.TextModel = "gemini-2.5-flash"
	}
	if cfg.ImageModel == "" {
		cfg.ImageModel = "imagen-4.0-generate-001"
	}
	if cfg.VideoModel == "" {
		// 免费 API 没有视频生成模型，MOCK 测试
		cfg.VideoModel = "video-placeholder"
	}
	if cfg.ApiKey == "" {
		cfg.ApiKey = "AIzaSyCa9PL-Q3goYFJ7O5QAPxKqkEmoGPKAx88"
	}

	return &AIService{
		Config:  cfg,
		Storage: NewStorageService(),
	}
}

// createClient 内部辅助：创建带代理的 client
func (s *AIService) createClient(ctx context.Context, proxy *model.Proxy) (*genai.Client, error) {
	var httpClient *http.Client
	if proxy != nil && proxy.IP != "" {
		if proxyURL, err := url.Parse(proxy.ProxyURL()); err == nil {
			httpClient = &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
		}
	}

	opts := []option.ClientOption{option.WithAPIKey(s.Config.ApiKey)}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	return genai.NewClient(ctx, opts...)
}

// GeneratePayloadContent 核心入口：一键生成所有素材
func (s *AIService) GeneratePayloadContent(ctx context.Context, proxy *model.Proxy, keyword, extraPrompt, imgUrl string) (*view.AIGenResult, error) {
	// 准备返回结果
	result := &view.AIGenResult{
		Images:   make([]string, 0),
		TextSets: make([]view.TextSetResult, 0),
	}

	// WaitGroup + Mutex，允许部分失败（比如视频生成失败不影响文案）
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	// 视觉逆向工程 (Vision Analysis)
	// 为了保证生成图片/视频的产品一致性，必须先由 Gemini 分析原图的每一个像素细节
	productDescription, err := s.analyzeProductImage(ctx, proxy, keyword, imgUrl)
	if err != nil {
		return nil, fmt.Errorf("视觉分析失败: %v", err)
	}
	// 此时 productDescription 是一段极尽详细的描述，如 "A knitted wool sweater in crimson red, cable knit pattern, v-neck, ribbed cuffs..."
	// 任务 A: 生成 2 套文案 (Text)
	wg.Add(1)
	go func() {
		defer wg.Done()
		texts, err := s.generateTextsSDK(ctx, proxy, keyword, productDescription, extraPrompt)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("Text Gen Error: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.TextSets = texts
		mu.Unlock()
	}()

	// 任务 B: 图片生成 (3批 x 3张 = 9张)
	// 注意：Gemini 2.5/3 Pro 具备原生生图能力 (Imagen 3 集成)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 3批 x 3张，使用不同的 Prompt 角度
		styles := []string{
			"Close-up detailed shot, focus on texture and material quality",            // 批次1：细节
			"Lifestyle shot, worn by a model in a cozy cafe setting, natural lighting", // 批次2：模特/场景
			"Flat lay photography on a wooden table with minimal props, overhead view", // 批次3：平铺/摆拍
		}

		for i, style := range styles {
			// 组合 Prompt：风格 + (AI反推的详细产品描述)
			fullPrompt := fmt.Sprintf("%s. Product details: %s. High quality, 1080p.", style, productDescription)

			urls, err := s.generateImagesREST(ctx, proxy, fullPrompt, 3) // 每批3张
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("ImgBatch%d: %v", i, err))
				mu.Unlock()
				continue
			}
			mu.Lock()
			result.Images = append(result.Images, urls...)
			mu.Unlock()
			time.Sleep(1 * time.Second)
		}
	}()

	// 任务 C: 生成 1 个 8s 视频 (REST + 异步轮询)
	wg.Add(1)
	go func() {
		defer wg.Done()
		// 视频也使用详细描述，确保视频里的产品长得像原图
		videoUrl, err := s.generateVideoREST(ctx, proxy, productDescription)
		if err != nil {
			mu.Lock()
			errs = append(errs, fmt.Sprintf("Video: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.Video = videoUrl
		mu.Unlock()
	}()

	wg.Wait()
	if len(errs) > 0 {
		fmt.Printf("AI 部分生成失败: %v\n", errs)
	}
	return result, nil
}

func (s *AIService) analyzeProductImage(ctx context.Context, proxy *model.Proxy, keyword, imgUrl string) (string, error) {
	// 使用 SDK 的多模态能力
	client, err := s.createGenAIClient(ctx, proxy)
	if err != nil {
		return "", err
	}
	defer client.Close()

	generativeModel := client.GenerativeModel(s.Config.TextModel)

	// 下载图片数据 (Gemini SDK 需要 []byte)
	imgData, err := utils.DownloadImage(imgUrl)
	if err != nil {
		return "", fmt.Errorf("下载参考图失败: %v", err)
	}

	// 自动检测 MIME Type
	// http.DetectContentType 会读取 bytes 的前 512 字节来判断文件类型
	// 它会返回标准的 "image/jpeg", "image/png" 等
	mimeType := http.DetectContentType(imgData)
	mimeFormat := strings.TrimPrefix(mimeType, "image/")
	fmt.Printf("[Debug] Image MIME Type detected: %s\n", mimeFormat)

	if mimeType == mimeFormat {
		mimeType = "jpeg"
	}

	prompt := fmt.Sprintf(`
		Analyze this product image of "%s". 
		Write a highly detailed visual description prompt suitable for an AI image generator (like Midjourney or Imagen).
		Describe the material, color, shape, texture, pattern, and key features in detail.
		Keep it objective and descriptive. Do not include marketing fluff.
	`, keyword)

	resp, err := generativeModel.GenerateContent(ctx,
		genai.Text(prompt),
		genai.ImageData(mimeFormat, imgData))

	if err != nil {
		return "", fmt.Errorf("AI 调用失败: %v", err)
	}
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return "", fmt.Errorf("no analysis returned")
	}

	// 提取文本
	for _, part := range resp.Candidates[0].Content.Parts {
		if txt, ok := part.(genai.Text); ok {
			return string(txt), nil
		}
	}
	return keyword, nil
}

// --- 内部具体实现 ---
// generateTextsSDK 生成文案
func (s *AIService) generateTextsSDK(ctx context.Context, proxy *model.Proxy, keyword, productDesc, extraPrompt string) ([]view.TextSetResult, error) {
	client, err := s.createGenAIClient(ctx, proxy)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	modelAI := client.GenerativeModel(s.Config.TextModel)
	modelAI.ResponseMIMEType = "application/json" // 强约束 JSON

	// Prompt 工程
	prompt := fmt.Sprintf(`
		Role: Etsy SEO Expert.
		Task: Generate 2 DISTINCT sets of listing content.
		Product Keyword: %s
		Visual Details: %s
		User Instructions: %s
		
		Requirements:
		1. Title: SEO optimized, high traffic keywords.
		2. Description: Engaging, include the visual details mentioned.
		3. Tags: 13 relevant tags.
		Output Format (JSON Array):
		[
		  { "title": "SEO Title 1...", "description": "Sales copy...", "tags": ["tag1", "tag2"] },
		  { "title": "SEO Title 2...", "description": "Storytelling copy...", "tags": ["tagA", "tagB"] }
		]
	`, keyword, productDesc, extraPrompt)

	resp, err := modelAI.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	var sets []view.TextSetResult
	if len(resp.Candidates) > 0 {
		for _, part := range resp.Candidates[0].Content.Parts {
			if txt, ok := part.(genai.Text); ok {
				raw := strings.TrimPrefix(string(txt), "```json")
				raw = strings.TrimPrefix(raw, "```")
				raw = strings.TrimSuffix(raw, "```")
				if err := json.Unmarshal([]byte(raw), &sets); err != nil {
					return nil, fmt.Errorf("json parse error: %v", err)
				}
			}
		}
	}
	return sets, nil
}

// Google Imagen 响应体结构
type imagenResponse struct {
	Predictions []struct {
		BytesBase64 string `json:"bytesBase64"` // Imagen 通常返回 Base64
		MimeType    string `json:"mimeType"`
	} `json:"predictions"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// generateImagesREST 生成图片
func (s *AIService) generateImagesREST(ctx context.Context, proxy *model.Proxy, fullPrompt string, count int) ([]string, error) {
	// API 付费，暂时跳过
	return nil, nil

	client := utils.NewProxiedClient(proxy)
	// 使用 beta 接口
	urlGoogle := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:predict?key=%s", s.Config.ImageModel, s.Config.ApiKey)

	// 构造 Imagen 3 标准 Payload
	payload := map[string]interface{}{
		"instances": []map[string]interface{}{
			{"prompt": fullPrompt},
		},
		"parameters": map[string]interface{}{
			"sampleCount": count,
			"aspectRatio": "1:1", // 根据业务需求可改为 3:4
		},
	}

	var res imagenResponse
	resp, err := client.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&res).
		Post(urlGoogle)
	if err != nil {
		return nil, fmt.Errorf("http req failed: %v", err)
	}
	if resp.IsError() || res.Error != nil {
		msg := "unknown"
		if res.Error != nil {
			msg = res.Error.Message
		}
		return nil, fmt.Errorf("api error: %s", msg)
	}

	var savedUrls []string
	for _, pred := range res.Predictions {
		if pred.BytesBase64 != "" {
			localUrl, err := s.Storage.SaveBase64(pred.BytesBase64, "ai_img")
			if err == nil {
				savedUrls = append(savedUrls, localUrl)
			}
		}
	}
	return savedUrls, nil
}

// generateVideo 视频生成 (REST API + 异步轮询 Operation)
// Google Operation 响应结构
type operationResponse struct {
	Name     string `json:"name"` // Operation ID，如 "operations/123456..."
	Metadata struct {
		State string `json:"state"` // PROCESSING, SUCCEEDED, FAILED
	} `json:"metadata"`
	Done     bool     `json:"done"`
	Response struct { // 完成后的结果
		Result struct {
			VideoUri string `json:"videoUri"` // 假设返回的是 URI 或 Base64，具体视模型而定
		} `json:"result"`
	} `json:"response"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// 视频生成
func (s *AIService) generateVideoREST(ctx context.Context, proxy *model.Proxy, productDesc string) (string, error) {
	if s.Config.VideoModel == "video-placeholder" {
		fmt.Println("[Warn] 当前账号无视频模型权限，返回模拟视频以跑通流程。")
		return "test video", nil
	}
	client := utils.NewProxiedClient(proxy)
	submitUrl := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:predict?key=%s", s.Config.VideoModel, s.Config.ApiKey)

	payload := map[string]interface{}{
		"instances": []map[string]interface{}{
			{
				// 使用 Vision 分析后的详细描述，确保视频里的产品一致
				"prompt":       fmt.Sprintf("Cinematic product video of %s. High quality, smooth motion, professional lighting. 8 seconds duration.", productDesc),
				"video_length": "8s",
			},
		},
	}

	// 1. 提交任务
	var opRes operationResponse // 结构体同上一轮
	resp, err := client.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&opRes).
		Post(submitUrl)
	if err != nil {
		return "", err
	}

	// 捕获 API 404 或 400 错误，提示用户检查 URL
	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("视频 API 调用失败 (Code %d). 请检查 submitUrl: %s 是否正确. Err: %v", resp.StatusCode(), submitUrl, resp.String())
	}

	if opRes.Error != nil {
		return "", fmt.Errorf("提交错误: %s", opRes.Error.Message)
	}

	// 2. 轮询 (Polling)
	opName := opRes.Name
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second) // 视频生成较慢，给 2 分钟
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return "", fmt.Errorf("视频生成超时")
		case <-ticker.C:
			// 获取 Operation 状态
			pollUrl := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/%s?key=%s", opName, s.Config.ApiKey)
			var pollRes operationResponse
			client.R().
				SetContext(timeoutCtx).
				SetResult(&pollRes).
				Get(pollUrl)

			if pollRes.Done {
				// 假设返回结构是 response.result.videoUri
				// 具体字段名需根据您拿到的文档核实
				return pollRes.Response.Result.VideoUri, nil
			}
		}
	}
}

// cleanJSON 辅助函数
func cleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return raw
}

type authTransport struct {
	apiKey    string
	transport http.RoundTripper
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("x-goog-api-key", t.apiKey)
	return t.transport.RoundTrip(req)
}

// createGenAIClient 辅助方法：创建 Google GenAI SDK 客户端
func (s *AIService) createGenAIClient(ctx context.Context, proxy *model.Proxy) (*genai.Client, error) {
	// 构造 SDK 选项
	apiKey := s.Config.ApiKey
	opts := []option.ClientOption{
		option.WithAPIKey(apiKey),
	}

	if proxy != nil && proxy.IP != "" {
		// 1. 获取代理 Client
		restyClient := utils.NewProxiedClient(proxy)
		httpClient := restyClient.GetClient()
		httpClient.Timeout = time.Second * 300

		// 2. 包装 Transport 注入 Key
		baseTransport := httpClient.Transport
		if baseTransport == nil {
			baseTransport = http.DefaultTransport
		}
		httpClient.Transport = &authTransport{
			apiKey:    apiKey,
			transport: baseTransport,
		}

		opts = append(opts, option.WithHTTPClient(httpClient))
	}

	return genai.NewClient(ctx, opts...)
}
