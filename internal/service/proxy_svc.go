package service

import (
	"context"
	"errors"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

type ProxyService struct {
	ProxyRepo repository.ProxyRepository
	ShopRepo  repository.ShopRepository

	maxFailCount int // 最大失败次数，超过判定 IP死亡
}

func NewProxyService(proxyRepo repository.ProxyRepository, shopRepo repository.ShopRepository) *ProxyService {
	return &ProxyService{
		ProxyRepo:    proxyRepo,
		ShopRepo:     shopRepo,
		maxFailCount: 10,
	}
}

// 1. 写入逻辑 (Create / Update)

// CreateProxy 创建代理
func (s *ProxyService) CreateProxy(ctx context.Context, req dto.CreateProxyReq, operatorID int64) error {
	// 1. 查重逻辑：防止 IP+Port 重复
	existProxy, err := s.ProxyRepo.FindByEndpoint(ctx, req.IP, req.Port)
	if err != nil {
		return err
	}
	if existProxy != nil {
		return errors.New("proxy already exists (IP:Port conflict)")
	}

	// 2. DTO -> Model 转换
	proxy := &model.Proxy{
		IP:       req.IP,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
		Protocol: req.Protocol,
		Region:   req.Region,
		Capacity: req.Capacity,
		Status:   1,    // 默认正常
		IsActive: true, // 默认启用
	}

	// 3. 审计字段填充
	proxy.CreatedBy = operatorID
	proxy.UpdatedBy = operatorID

	// 4. 落库
	return s.ProxyRepo.Create(ctx, proxy)
}

// UpdateProxy 更新代理
func (s *ProxyService) UpdateProxy(ctx context.Context, req dto.UpdateProxyReq, operatorID int64) error {
	// 1. 检查是否存在
	proxy, err := s.ProxyRepo.GetByID(ctx, req.ID)
	if err != nil {
		return err
	}
	if proxy.ID == 0 {
		return errors.New("proxy not found")
	}

	// 2. 更新字段 (只更新允许修改的)
	if req.IP != "" {
		proxy.IP = req.IP
	}
	if req.Port != "" {
		proxy.Port = req.Port
	}
	if req.Username != "" {
		proxy.Username = req.Username
	}
	if req.Password != "" {
		proxy.Password = req.Password
	}
	if req.Region != "" {
		proxy.Region = req.Region
	}
	if req.Capacity > 0 {
		proxy.Capacity = req.Capacity
	}
	if req.Status > 0 {
		proxy.Status = req.Status
	}

	// 3. 更新审计
	proxy.UpdatedBy = operatorID

	return s.ProxyRepo.Update(ctx, proxy)
}

// 2. 读取逻辑 (List / Get) - 重点在于 DTO 组装

// GetProxyList 获取分页列表 (带关联统计)
func (s *ProxyService) GetProxyList(ctx context.Context, filter repository.ProxyFilter) ([]dto.ProxyResp, int64, error) {
	// 1. 调用 Repo 获取 Model 列表 (Repo 层已经 Preload 了 Shops 和 Developers)
	list, total, err := s.ProxyRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 2. Model -> DTO 转换
	respList := make([]dto.ProxyResp, 0, len(list))
	for _, p := range list {
		respList = append(respList, s.convertToResp(&p, false)) // false = 列表页不需要详情，只需要计数
	}

	return respList, total, nil
}

// GetProxyDetail 获取单个详情 (带完整关联列表)
func (s *ProxyService) GetProxyDetail(ctx context.Context, id int64) (*dto.ProxyResp, error) {
	// 1. 调用 Repo
	proxy, err := s.ProxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 2. 转换 (true = 需要完整的 BoundShops 列表)
	resp := s.convertToResp(proxy, true)
	return &resp, nil
}

// 3. 内部辅助函数

// convertToResp 将 Model 转为 DTO
// withDetails: 是否需要展开 BoundShops 的详细列表
func (s *ProxyService) convertToResp(p *model.Proxy, withDetails bool) dto.ProxyResp {
	resp := dto.ProxyResp{
		ID:            p.ID,
		IP:            p.IP,
		Port:          p.Port,
		Username:      p.Username,
		Password:      p.Password,
		Protocol:      p.Protocol,
		Region:        p.Region,
		Capacity:      p.Capacity,
		Status:        p.Status,
		FailureCount:  p.FailureCount,
		LastCheckTime: p.LastCheckTime.Unix(),
		IsActive:      p.IsActive,
		CreatedAt:     p.CreatedAt.Unix(), // 转时间戳

		CreatedBy: p.CreatedBy,
		// TODO: 如果需要 CreatedByName，这里需要调用 UserService 查名字，或者在 Repo 层 Join 查出来

		// 核心业务：统计数量
		ShopCount: len(p.Shops),
	}

	// 如果需要详情（详情页），则进行深度组装
	if withDetails {
		// 组装店铺列表
		resp.BoundShops = make([]dto.BoundShopItem, 0, len(p.Shops))
		for _, shop := range p.Shops {
			resp.BoundShops = append(resp.BoundShops, dto.BoundShopItem{
				ShopID:     shop.ID,
				ShopName:   shop.ShopName,
				EtsyShopID: shop.EtsyShopID,
				Status:     shop.TokenStatus,
			})
		}
	}

	return resp
}

// 代理检查与处理逻辑（自动从代理池寻找可用 IP，迁移店铺，保证业务不中断）

// VerifyAndHeal 核心原子能力
// 作用：对指定 Proxy 进行一次体检。如果发现死亡，立即触发迁移。
// 场景：Cron 巡检循环调用它；Dispatcher 发现请求失败时单点调用它
func (s *ProxyService) VerifyAndHeal(ctx context.Context, proxy *model.Proxy) error {
	// A. 探测连通性
	isAlive := s.TestConnectivity(proxy)
	var err error
	if isAlive {
		if proxy.FailureCount > 0 || proxy.Status != 1 {
			proxy.FailureCount = 0
			proxy.Status = 1
			err = s.ProxyRepo.UpdateStatusAndCount(ctx, proxy)
			if err != nil {
				log.Printf("[ProxyMonitor] Failed to update proxy status: %v\n", err)
			}
		} else {
			err = s.ProxyRepo.UpdateLastCheckTime(ctx, proxy.ID)
			if err != nil {
				log.Printf("[ProxyMonitor] Failed to update last proxy check time: %v\n", err)
			}
		}
		return err
	}

	// B2. 异常：进入故障处理流程
	proxy.FailureCount++
	log.Printf("Proxy %s connect failed. Count: %d", proxy.IP, proxy.FailureCount)

	// 判定是否“彻底报废”
	if proxy.FailureCount >= s.maxFailCount {
		proxy.Status = 3 // Dead/Banned
		log.Printf("Proxy %s is DEAD (Max fail count reached).", proxy.IP)
	} else {
		proxy.Status = 2 // Unstable
	}

	// 更新状态到数据库
	if err = s.ProxyRepo.UpdateStatusAndCount(ctx, proxy); err != nil {
		log.Println("Update proxy error:", err)
		return err
	}

	// 触发迁移：只有当状态变为不可用时，才需要把店移走
	// 如果它已经是 Status=2 且店都被移走了，这里其实查出来是空列表，不耗性能
	return s.MigrateShops(ctx, proxy)
}

// MigrateShops 迁移绑定在故障代理上的所有店铺
// 作用：将指定坏代理下的所有店铺，自动迁移到同地区的可用代理上
func (s *ProxyService) MigrateShops(ctx context.Context, deadProxy *model.Proxy) error {
	// 1. 查找所有绑定在这个坏代理上的店铺
	shops, err := s.ShopRepo.GetByProxyID(ctx, deadProxy.ID)
	if err != nil {
		log.Printf("Failed to get affected shops: %v", err)
		return err
	}

	if len(shops) == 0 {
		return nil // 没有店铺受影响
	}

	log.Printf("Migrating %d shops from Proxy %d...", len(shops), deadProxy.ID)

	// 2. 逐个迁移
	for _, shop := range shops {
		bestProxy, err := s.ProxyRepo.FindSpareProxy(ctx, deadProxy.Region)
		if err != nil {
			log.Printf("[CRITICAL] No spare proxy for Shop %d (Region %s)!", shop.ID, deadProxy.Region)
			continue
		}

		// 更新绑定
		if err := s.ShopRepo.UpdateFields(ctx, shop.ID, map[string]interface{}{"proxy_id": bestProxy.ID}); err == nil {
			log.Printf("Shop %d migrated to Proxy %d", shop.ID, bestProxy.ID)
		}
	}
	return err
}

// TestConnectivity 真实的连通性测试
// 作用：执行一次物理连接测试
func (s *ProxyService) TestConnectivity(proxy *model.Proxy) bool {
	proxyURL, _ := url.Parse(fmt.Sprintf("%s://%s:%s", proxy.Protocol, proxy.IP, proxy.Port))
	if proxy.Username != "" {
		proxyURL.User = url.UserPassword(proxy.Username, proxy.Password)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 10 * time.Second, // 5秒超时
	}

	// 访问 Etsy 静态资源
	resp, err := client.Get("https://www.etsy.com/robots.txt")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}

// PickBestProxy
// 作用：为指定 Shop 挑选一个最佳（最空闲/同地区）的可用代理
func (s *ProxyService) PickBestProxy(ctx context.Context, region string) (*model.Proxy, error) {
	proxy, err := s.ProxyRepo.FindSpareProxy(ctx, region)
	if err != nil {
		return nil, err
	}
	return proxy, nil
}

func (s *ProxyService) PickRandomProxy(ctx context.Context) (*model.Proxy, error) {
	proxy, err := s.ProxyRepo.GetRandomProxy(ctx)
	if err != nil {
		return nil, err
	}
	return proxy, err
}
