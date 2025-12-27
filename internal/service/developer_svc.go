package service

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/net"
	"etsy_dev_v1_202512/pkg/utils"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"
)

const (
	ETSYPing = "https://api.etsy.com/v3/application/openapi-ping"
)

type DeveloperService struct {
	DeveloperRepo repository.DeveloperRepository
	ShopRepo      repository.ShopRepository
	Dispatcher    net.Dispatcher
}

func NewDeveloperService(developerRepo repository.DeveloperRepository, shopRepo repository.ShopRepository, dispatcher net.Dispatcher) *DeveloperService {
	return &DeveloperService{
		DeveloperRepo: developerRepo,
		ShopRepo:      shopRepo,
		Dispatcher:    dispatcher,
	}
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

// GetDeveloperList 分页列表查询
func (s *DeveloperService) GetDeveloperList(ctx context.Context, filter repository.DeveloperFilter) ([]dto.DeveloperResp, int64, error) {
	list, total, err := s.DeveloperRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	respList := make([]dto.DeveloperResp, 0, len(list))
	for _, dev := range list {
		respList = append(respList, s.convertToResp(&dev))
	}

	return respList, total, nil
}

// GetDeveloperDetail 详情查询
func (s *DeveloperService) GetDeveloperDetail(ctx context.Context, id int64) (*dto.DeveloperResp, error) {
	dev, err := s.DeveloperRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := s.convertToResp(dev)
	return &resp, nil
}

// UpdateDeveloper 更新开发者信息（仅允许修改 Name/LoginPwd/SharedSecret）
func (s *DeveloperService) UpdateDeveloper(ctx context.Context, id int64, req dto.UpdateDeveloperReq) error {
	dev, err := s.DeveloperRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// 按需更新字段
	if req.Name != "" {
		dev.Name = req.Name
	}
	if req.LoginPwd != "" {
		dev.LoginPwd = req.LoginPwd
	}
	if req.SharedSecret != "" {
		dev.SharedSecret = req.SharedSecret
	}

	return s.DeveloperRepo.Update(ctx, dev)
}

// UpdateStatus 状态变更
func (s *DeveloperService) UpdateStatus(ctx context.Context, id int64, status int) error {
	// 校验状态值合法性
	if status < 0 || status > 2 {
		return errors.New("invalid status value")
	}
	return s.DeveloperRepo.UpdateStatus(ctx, id, status)
}

// DeleteDeveloper 软删除（先解绑关联 Shop）
func (s *DeveloperService) DeleteDeveloper(ctx context.Context, id int64) error {
	// 1. 解绑所有关联的 Shop
	if err := s.DeveloperRepo.UnbindShops(ctx, id); err != nil {
		return fmt.Errorf("解绑关联店铺失败: %v", err)
	}

	// 2. 执行软删除
	return s.DeveloperRepo.Delete(ctx, id)
}

// TestConnectivity 测试 API Key 连通性
// 返回：是否成功、响应耗时(ms)、错误信息
func (s *DeveloperService) TestConnectivity(ctx context.Context, id int64) (bool, int64, error) {
	// 1. 查询 Developer
	dev, err := s.DeveloperRepo.GetByID(ctx, id)
	if err != nil {
		return false, 0, err
	}

	// 2. 构造请求
	req, err := http.NewRequestWithContext(ctx, "GET", ETSYPing, nil)
	if err != nil {
		return false, 0, err
	}
	req.Header.Set("x-api-key", dev.ApiKey)

	// 3. 设置超时
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req = req.WithContext(pingCtx)

	// 4. 通过 Dispatcher 发送（使用随机代理）
	startTime := time.Now()
	resp, err := s.Dispatcher.Ping(pingCtx, req)
	elapsed := time.Since(startTime).Milliseconds()

	if err != nil {
		return false, elapsed, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 5. 判断结果
	if resp.StatusCode == 200 {
		return true, elapsed, nil
	}

	return false, elapsed, fmt.Errorf("ETSY 返回状态码: %d", resp.StatusCode)
}

// convertToResp Model 转 DTO
func (s *DeveloperService) convertToResp(dev *model.Developer) dto.DeveloperResp {
	resp := dto.DeveloperResp{
		ID:           uint(dev.ID),
		CreatedAt:    dev.CreatedAt,
		Name:         dev.Name,
		LoginEmail:   dev.LoginEmail,
		ApiKey:       dev.ApiKey,
		SharedSecret: dev.SharedSecret,
		CallbackURL:  dev.CallbackURL,
		Status:       dev.Status,
		StatusText:   s.getStatusText(dev.Status),
	}
	return resp
}

// getStatusText 状态码转文本
func (s *DeveloperService) getStatusText(status int) string {
	switch status {
	case model.DeveloperStatusPending:
		return "未配置"
	case model.DeveloperStatusActive:
		return "正常"
	case model.DeveloperStatusBanned:
		return "已封禁"
	default:
		return "未知"
	}
}
