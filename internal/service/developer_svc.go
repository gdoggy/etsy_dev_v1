package service

import "etsy_dev_v1_202512/internal/repository"

const (
	URLScheme = "https"
	BaseURL   = "test.com"
	ETSYPing  = "https://api.etsy.com/v3/application/openapi-ping"
)

type DeveloperService struct {
	DeveloperRepo *repository.DeveloperRepo
}

func (s *DeveloperService) NewDeveloperService(developerRepo *repository.DeveloperRepo) *DeveloperService {
	return &DeveloperService{DeveloperRepo: developerRepo}
}

// TODO 新建 developer,自动生成 callbackURL
// 测试 apiKey PING ETSY
