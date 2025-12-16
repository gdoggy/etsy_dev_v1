package service

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// StorageService 负责文件保存
type StorageService struct {
	BaseDir string // 本地存放目录，如 "./static/uploads"
	BaseUrl string // 访问前缀，如 "http://localhost:8080/static/uploads"
}

func NewStorageService() *StorageService {
	// 确保目录存在
	dir := "./static/uploads"
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0755)
	}
	return &StorageService{
		BaseDir: dir,
		BaseUrl: "/static/uploads", // 前端访问路径
	}
}

// SaveBase64 将 Base64 字符串保存为图片文件
func (s *StorageService) SaveBase64(b64Str string, prefix string) (string, error) {
	// 1. 清洗 Data URI 前缀 (data:image/jpeg;base64,...)
	if idx := strings.Index(b64Str, ","); idx != -1 {
		b64Str = b64Str[idx+1:]
	}

	// 2. 解码
	data, err := base64.StdEncoding.DecodeString(b64Str)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %v", err)
	}

	// 3. 生成文件名
	fileName := fmt.Sprintf("%s_%d.png", prefix, time.Now().UnixNano())
	filePath := filepath.Join(s.BaseDir, fileName)

	// 4. 写入磁盘
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("write file failed: %v", err)
	}

	// 5. 返回可访问的 URL
	return fmt.Sprintf("%s/%s", s.BaseUrl, fileName), nil
}
