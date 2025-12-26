package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// ==================== 接口定义 ====================

// StorageProvider 存储提供者接口
type StorageProvider interface {
	// Upload 上传文件，返回公开访问URL
	Upload(ctx context.Context, data []byte, filename string, contentType string) (url string, err error)

	// UploadFromURL 从URL下载并上传
	UploadFromURL(ctx context.Context, sourceURL string, filename string) (url string, err error)

	// Delete 删除文件
	Delete(ctx context.Context, url string) error

	// GetSignedURL 获取签名URL (私有存储时使用)
	GetSignedURL(ctx context.Context, url string, expires time.Duration) (signedURL string, err error)
}

// ==================== 配置 ====================

type StorageConfig struct {
	Provider  string // "s3" | "cos" | "local"
	Bucket    string
	Region    string
	AccessKey string
	SecretKey string
	Endpoint  string // 自定义端点 (腾讯云COS等)
	CDNDomain string // CDN域名 (可选)
	BasePath  string // 基础路径前缀
}

// ==================== 工厂方法 ====================

func NewStorageProvider(cfg *StorageConfig) (StorageProvider, error) {
	switch cfg.Provider {
	case "s3":
		return NewS3Storage(cfg)
	case "cos":
		return NewCOSStorage(cfg)
	case "local":
		return NewLocalStorage(cfg)
	default:
		return nil, fmt.Errorf("不支持的存储提供者: %s", cfg.Provider)
	}
}

// ==================== StorageService 兼容层 ====================

// StorageService 存储服务（兼容旧代码，包装 StorageProvider）
type StorageService struct {
	provider StorageProvider
	config   *StorageConfig
}

// NewStorageService 创建存储服务
func NewStorageService(cfg *StorageConfig) (*StorageService, error) {
	provider, err := NewStorageProvider(cfg)
	if err != nil {
		return nil, err
	}
	return &StorageService{
		provider: provider,
		config:   cfg,
	}, nil
}

// Upload 上传文件
func (s *StorageService) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	return s.provider.Upload(ctx, data, filename, contentType)
}

// UploadFromURL 从URL下载并上传
func (s *StorageService) UploadFromURL(ctx context.Context, sourceURL string, filename string) (string, error) {
	return s.provider.UploadFromURL(ctx, sourceURL, filename)
}

// Delete 删除文件
func (s *StorageService) Delete(ctx context.Context, url string) error {
	return s.provider.Delete(ctx, url)
}

// GetSignedURL 获取签名URL
func (s *StorageService) GetSignedURL(ctx context.Context, url string, expires time.Duration) (string, error) {
	return s.provider.GetSignedURL(ctx, url, expires)
}

// SaveBase64 保存 Base64 图片（兼容旧接口）
func (s *StorageService) SaveBase64(base64Data string, prefix string) (string, error) {
	return SaveBase64ToStorage(context.Background(), s.provider, base64Data, prefix)
}

// GetProvider 获取底层 Provider（用于需要接口的场景）
func (s *StorageService) GetProvider() StorageProvider {
	return s.provider
}

// ==================== S3 实现 ====================

type S3Storage struct {
	client    *s3.Client
	bucket    string
	region    string
	cdnDomain string
	basePath  string
}

func NewS3Storage(cfg *StorageConfig) (*S3Storage, error) {
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("加载AWS配置失败: %v", err)
	}

	client := s3.NewFromConfig(awsCfg)

	return &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		region:    cfg.Region,
		cdnDomain: cfg.CDNDomain,
		basePath:  cfg.BasePath,
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	key := s.generateKey(filename)

	if contentType == "" {
		contentType = detectContentType(data)
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("上传S3失败: %v", err)
	}

	return s.getPublicURL(key), nil
}

func (s *S3Storage) UploadFromURL(ctx context.Context, sourceURL string, filename string) (string, error) {
	data, contentType, err := downloadFile(ctx, sourceURL)
	if err != nil {
		return "", err
	}
	return s.Upload(ctx, data, filename, contentType)
}

func (s *S3Storage) Delete(ctx context.Context, url string) error {
	key := s.extractKey(url)
	if key == "" {
		return fmt.Errorf("无法解析文件路径")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Storage) GetSignedURL(ctx context.Context, url string, expires time.Duration) (string, error) {
	key := s.extractKey(url)
	if key == "" {
		return "", fmt.Errorf("无法解析文件路径")
	}

	presignClient := s3.NewPresignClient(s.client)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}

	return presignedURL.URL, nil
}

func (s *S3Storage) generateKey(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	datePath := time.Now().Format("2006/01/02")
	if s.basePath != "" {
		return fmt.Sprintf("%s/%s/%s", s.basePath, datePath, newFilename)
	}
	return fmt.Sprintf("%s/%s", datePath, newFilename)
}

func (s *S3Storage) getPublicURL(key string) string {
	if s.cdnDomain != "" {
		return fmt.Sprintf("https://%s/%s", s.cdnDomain, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
}

func (s *S3Storage) extractKey(url string) string {
	// 从URL中提取key
	if s.cdnDomain != "" && strings.Contains(url, s.cdnDomain) {
		return strings.TrimPrefix(url, fmt.Sprintf("https://%s/", s.cdnDomain))
	}
	prefix := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/", s.bucket, s.region)
	return strings.TrimPrefix(url, prefix)
}

// ==================== 腾讯云COS 实现 ====================

type COSStorage struct {
	client    *s3.Client
	bucket    string
	region    string
	cdnDomain string
	basePath  string
}

func NewCOSStorage(cfg *StorageConfig) (*COSStorage, error) {
	// COS兼容S3协议
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://cos.%s.myqcloud.com", cfg.Region)
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("加载COS配置失败: %v", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &COSStorage{
		client:    client,
		bucket:    cfg.Bucket,
		region:    cfg.Region,
		cdnDomain: cfg.CDNDomain,
		basePath:  cfg.BasePath,
	}, nil
}

func (s *COSStorage) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	key := s.generateKey(filename)

	if contentType == "" {
		contentType = detectContentType(data)
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("上传COS失败: %v", err)
	}

	return s.getPublicURL(key), nil
}

func (s *COSStorage) UploadFromURL(ctx context.Context, sourceURL string, filename string) (string, error) {
	data, contentType, err := downloadFile(ctx, sourceURL)
	if err != nil {
		return "", err
	}
	return s.Upload(ctx, data, filename, contentType)
}

func (s *COSStorage) Delete(ctx context.Context, url string) error {
	key := s.extractKey(url)
	if key == "" {
		return fmt.Errorf("无法解析文件路径")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *COSStorage) GetSignedURL(ctx context.Context, url string, expires time.Duration) (string, error) {
	key := s.extractKey(url)
	if key == "" {
		return "", fmt.Errorf("无法解析文件路径")
	}

	presignClient := s3.NewPresignClient(s.client)
	presignedURL, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expires))
	if err != nil {
		return "", err
	}

	return presignedURL.URL, nil
}

func (s *COSStorage) generateKey(filename string) string {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	newFilename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	datePath := time.Now().Format("2006/01/02")
	if s.basePath != "" {
		return fmt.Sprintf("%s/%s/%s", s.basePath, datePath, newFilename)
	}
	return fmt.Sprintf("%s/%s", datePath, newFilename)
}

func (s *COSStorage) getPublicURL(key string) string {
	if s.cdnDomain != "" {
		return fmt.Sprintf("https://%s/%s", s.cdnDomain, key)
	}
	return fmt.Sprintf("https://%s.cos.%s.myqcloud.com/%s", s.bucket, s.region, key)
}

func (s *COSStorage) extractKey(url string) string {
	if s.cdnDomain != "" && strings.Contains(url, s.cdnDomain) {
		return strings.TrimPrefix(url, fmt.Sprintf("https://%s/", s.cdnDomain))
	}
	prefix := fmt.Sprintf("https://%s.cos.%s.myqcloud.com/", s.bucket, s.region)
	return strings.TrimPrefix(url, prefix)
}

// ==================== 本地存储 (开发测试用) ====================

type LocalStorage struct {
	basePath string
	baseURL  string
}

func NewLocalStorage(cfg *StorageConfig) (*LocalStorage, error) {
	basePath := cfg.BasePath
	if basePath == "" {
		basePath = "./uploads"
	}
	baseURL := cfg.Endpoint
	if baseURL == "" {
		baseURL = "http://localhost:8080/uploads"
	}

	return &LocalStorage{
		basePath: basePath,
		baseURL:  baseURL,
	}, nil
}

func (s *LocalStorage) Upload(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	// 本地存储实现略，开发环境可用
	return "", fmt.Errorf("本地存储暂未完整实现")
}

func (s *LocalStorage) UploadFromURL(ctx context.Context, sourceURL string, filename string) (string, error) {
	data, contentType, err := downloadFile(ctx, sourceURL)
	if err != nil {
		return "", err
	}
	return s.Upload(ctx, data, filename, contentType)
}

func (s *LocalStorage) Delete(ctx context.Context, url string) error {
	return nil
}

func (s *LocalStorage) GetSignedURL(ctx context.Context, url string, expires time.Duration) (string, error) {
	return url, nil // 本地存储无需签名
}

// ==================== 工具函数 ====================

func downloadFile(ctx context.Context, url string) ([]byte, string, error) {
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

	contentType := resp.Header.Get("Content-Type")
	return data, contentType, nil
}

func detectContentType(data []byte) string {
	return http.DetectContentType(data)
}

// SaveBase64ToStorage 保存Base64编码的图片
func SaveBase64ToStorage(ctx context.Context, provider StorageProvider, base64Data string, prefix string) (string, error) {
	// 去除可能的data URL前缀
	if idx := strings.Index(base64Data, ","); idx != -1 {
		base64Data = base64Data[idx+1:]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("Base64 解码失败: %v", err)
	}

	filename := fmt.Sprintf("%s_%s.jpg", prefix, uuid.New().String()[:8])
	return provider.Upload(ctx, data, filename, "image/jpeg")
}
