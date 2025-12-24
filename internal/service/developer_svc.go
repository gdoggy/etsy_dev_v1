package service

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/utils"
	"fmt"
	"math/rand/v2"
)

const (
	ETSYPing = "https://api.etsy.com/v3/application/openapi-ping"
)

type DeveloperService struct {
	DeveloperRepo *repository.DeveloperRepo
}

func NewDeveloperService(developerRepo *repository.DeveloperRepo) *DeveloperService {
	return &DeveloperService{DeveloperRepo: developerRepo}
}

func (s *DeveloperService) CreateDeveloper(ctx context.Context, req dto.CreateDeveloperReq) (string, error) {
	// 1. 查重逻辑：防止 apiKey 重复
	existDev, err := s.DeveloperRepo.FindByApiKey(ctx, req.ApiKey)
	if err != nil {
		return "", err
	}
	if existDev != nil {
		return "", errors.New("developer api key already exists")
	}
	// 2. DTO -> Model 转换
	dev := &model.Developer{
		Name:         req.Name,
		LoginEmail:   req.LoginEmail,
		LoginPwd:     req.LoginPwd,
		ApiKey:       req.ApiKey,
		SharedSecret: req.SharedSecret,
	}
	if err = s.InitDeveloper(ctx, dev); err != nil {
		return "", err
	}
	return dev.CallbackURL, nil
}

// InitDeveloper 初始化 developer 生成防关联 callbackURL
func (s *DeveloperService) InitDeveloper(ctx context.Context, developer *model.Developer) error {
	// 1. 找一个随机域名
	domain, err := s.DeveloperRepo.GetRandomActiveDomain(ctx)
	if err != nil {
		return fmt.Errorf("no active domains available err: %s", err)
	}
	// 2. 生成随机字符串
	developer.SubDomain, _ = utils.GenerateRandomString(rand.IntN(5) + 8)
	developer.CallbackPath, _ = utils.GenerateRandomString(rand.IntN(3) + 6)
	developer.DomainPoolID = domain.ID
	// 3. 拼接 url，格式： https://{subdomain}.{host}/api/{path}/auth/callback
	developer.CallbackURL = fmt.Sprintf("https://%s.%s/api/%s/auth/callback",
		developer.SubDomain, domain.Host, developer.CallbackPath)
	// 4. 初始化状态为 pending 未配置
	developer.Status = 0
	return s.DeveloperRepo.Create(ctx, developer)
}
