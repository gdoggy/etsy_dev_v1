package service

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNewAIService_DefaultConfig(t *testing.T) {
	svc := NewAIService(&AIConfig{}, nil, nil)

	if svc.Config.TextModel != "gemini-3-flash" {
		t.Errorf("默认 TextModel 不正确: got %s, want gemini-3-flash", svc.Config.TextModel)
	}

	if svc.Config.ImageModel != "gemini-3-pro-image-preview-2k" {
		t.Errorf("默认 ImageModel 不正确: got %s", svc.Config.ImageModel)
	}
}

func TestAIService_GenerateProductContent(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("跳过: 需要设置 GEMINI_API_KEY 环境变量")
	}

	svc := NewAIService(&AIConfig{
		ApiKey: apiKey,
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tests := []struct {
		name      string
		title     string
		styleHint string
		wantErr   bool
	}{
		{
			name:      "生成手工艺品文案",
			title:     "Handmade Ceramic Mug with Gold Rim",
			styleHint: "vintage, elegant",
			wantErr:   false,
		},
		{
			name:      "生成珠宝文案",
			title:     "Sterling Silver Moon Phase Necklace",
			styleHint: "minimalist, bohemian",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.GenerateProductContent(ctx, tt.title, tt.styleHint)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateProductContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != nil {
				if result.Title == "" {
					t.Error("生成的标题为空")
				}
				if result.Description == "" {
					t.Error("生成的描述为空")
				}
				if len(result.Tags) == 0 {
					t.Error("生成的标签为空")
				}

				t.Logf("生成标题: %s", result.Title)
				t.Logf("标签数量: %d", len(result.Tags))
				t.Logf("描述长度: %d 字符", len(result.Description))
			}
		})
	}
}

func TestAIService_GenerateImages(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("跳过: 需要设置 GEMINI_API_KEY 环境变量")
	}

	svc := NewAIService(&AIConfig{
		ApiKey: apiKey,
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	tests := []struct {
		name          string
		prompt        string
		referenceURL  string
		count         int
		wantMinImages int
		wantErr       bool
	}{
		{
			name:          "生成产品图片",
			prompt:        "A handmade ceramic coffee mug, white with gold rim, professional studio lighting, e-commerce style",
			referenceURL:  "",
			count:         1,
			wantMinImages: 1,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images, err := svc.GenerateImages(ctx, tt.prompt, tt.referenceURL, tt.count)

			if (err != nil) != tt.wantErr {
				t.Errorf("GenerateImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(images) < tt.wantMinImages {
					t.Errorf("GenerateImages() 返回图片数量 = %d, 期望至少 %d", len(images), tt.wantMinImages)
				}

				for i, img := range images {
					if len(img) < 100 {
						t.Errorf("图片 %d 的 base64 数据过短", i)
					}
					t.Logf("图片 %d: base64 长度 %d", i, len(img))
				}
			}
		})
	}
}

func TestAIService_GenerateImages_NoAPIKey(t *testing.T) {
	svc := NewAIService(&AIConfig{
		ApiKey: "",
	}, nil, nil)

	ctx := context.Background()
	_, err := svc.GenerateImages(ctx, "test prompt", "", 1)

	if err == nil {
		t.Error("期望返回错误，但未返回")
	}
}

func TestAIService_GenerateProductContent_NoAPIKey(t *testing.T) {
	svc := NewAIService(&AIConfig{
		ApiKey: "",
	}, nil, nil)

	ctx := context.Background()
	_, err := svc.GenerateProductContent(ctx, "test", "")

	if err == nil {
		t.Error("期望返回错误，但未返回")
	}
}
