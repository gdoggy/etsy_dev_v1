package service

import (
	"errors"
	"etsy_dev_v1_202512/internal/core/model"
	"fmt"
	"strconv"
	"strings"
	"time"

	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/utils"

	"github.com/go-resty/resty/v2"
)

// 业务常量
const (
	// CallbackURL 必须与 Etsy 后台填写的完全一致
	// CallbackURL = "http://localhost:8080/api/auth/callback"
	CallbackURL = "https://elizabet-avian-glenna.ngrok-free.dev/api/auth/callback"
)

type AuthService struct {
	ShopRepo *repository.ShopRepository
}

// NewAuthService 工厂方法
func NewAuthService(sr *repository.ShopRepository) *AuthService {
	return &AuthService{ShopRepo: sr}
}

// GenerateLoginURL 生成授权链接
func (s *AuthService) GenerateLoginURL(shopID uint) (string, error) {
	// 1. 获取店铺预配置信息
	var shop model.Shop
	if err := s.ShopRepo.DB.Preload("Developer").First(&shop, shopID).Error; err != nil {
		return "", errors.New("店铺未预置，请先在系统录入店铺信息")
	}

	// 2. 严格校验
	if shop.DeveloperID == 0 || shop.Developer.ID == 0 {
		return "", errors.New("该店铺未绑定开发者账号，请检查配置")
	}
	// 校验 IP 一致性：如果不一致说明数据库脏了
	if shop.ProxyID != shop.Developer.ProxyID {
		return "", errors.New("IP不一致，请检查数据源")
	}

	// 3. 生成 PKCE 安全参数
	verifier, _ := utils.GenerateRandomString(32)
	challenge := utils.GenerateCodeChallenge(verifier)
	state, _ := utils.GenerateRandomString(16)

	// 4. 缓存 Verifier (重要：格式为 "verifier:shop_id")
	// 这样回调时我们就知道是哪个 Adapter 发起的请求
	cacheValue := fmt.Sprintf("%s:%d", verifier, shop.ID)
	utils.SetCache(state, cacheValue)

	// 5. 拼接 Etsy 官方授权 URL
	// 权限: 读取商品、读取交易、更新交易(发货)、读取店铺信息
	scopes := "listings_r transactions_r transactions_w shops_r"
	/*
		etsy 官网案例：
		   https://www.etsy.com/oauth/connect?
		     response_type=code
		     &redirect_uri=https://www.example.com/some/location
		     &scope=transactions_r%20transactions_w
		     &client_id=1aa2bb33c44d55eeeeee6fff&state=superstate
		     &code_challenge=DSWlW2Abh-cf8CeLL8-g3hQ2WQyYdKyiu83u_s7nRhI
		     &code_challenge_method=S256
	*/
	authURL := fmt.Sprintf(
		"https://www.etsy.com/oauth/connect?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		shop.Developer.APIKey, CallbackURL, scopes, state, challenge,
	)

	return authURL, nil
}

// HandleCallback 处理 Etsy 回调，解析 State -> 找到预置 Shop -> 组装 Proxy -> 换 Token -> 补全信息 -> 更新入库
func (s *AuthService) HandleCallback(code, state string) (*model.Shop, error) {
	// 1. 校验 State 并取出缓存
	cachedVal, exists := utils.GetCache(state)
	if !exists {
		return nil, errors.New("授权超时或 State 无效，请重新发起")
	}

	// 2. 解析缓存 "verifier:shop_id"
	parts := strings.Split(cachedVal, ":")

	// 简单的格式校验
	if len(parts) != 2 {
		return nil, fmt.Errorf("缓存数据格式错误，预期 'verifier:shopID'，实际: %s", cachedVal)
	}

	verifier := parts[0]

	// 将字符串转为数字
	shopIDInt, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("缓存中的 ShopID 无效: %v", err)
	}
	shopID := uint(shopIDInt)

	// 3. 查出预置的 Shop
	var shop model.Shop
	if err := s.ShopRepo.DB.Preload("Proxy").Preload("Developer").First(&shop, shopID).Error; err != nil {
		return nil, errors.New("未找到对应的店铺预置信息")
	}

	// 4. 工厂调用 构建专用网络客户端
	client := utils.NewProxiedClient(shop.Proxy)

	// 5. 换取 Token
	tokenResp, err := s.exchangeToken(client, shop.Developer.APIKey, code, verifier)
	if err != nil {
		s.updateTokenStatus(&shop, model.TokenStatusInvalid)
		return nil, fmt.Errorf("换取 Token 失败: %v", err)
	}

	// 6. 获取用户 ID (User ID)
	userID, err := s.fetchUserID(client, shop.Developer.APIKey, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("获取 UserID 失败: %v", err)
	}

	// 7. 获取 ShopInfo
	shopInfo, err := s.fetchShopInfo(client, shop.Developer.APIKey, tokenResp.AccessToken, userID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %v", err)
	}

	// 8. 更新数据
	shop.UserID = userID
	shop.EtsyShopID = shopInfo.EtsyShopID
	shop.ShopName = shopInfo.ShopName
	shop.AccessToken = tokenResp.AccessToken
	shop.RefreshToken = tokenResp.RefreshToken
	shop.TokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	// 入库保存
	if err := s.ShopRepo.DB.Save(&shop).Error; err != nil {
		return nil, fmt.Errorf("店铺入库失败: %v", err)
	}

	return &shop, nil
}

// 辅助结构体：Token 响应
type etsyTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

// 1. 换取 Token
func (s *AuthService) exchangeToken(client *resty.Client, appKey, code, verifier string) (*etsyTokenResp, error) {
	var tokenResp etsyTokenResp
	// 强制设置 Content-Type，防止有些代理或服务器识别不了
	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     appKey,
			"redirect_uri":  CallbackURL, // 必须与 GenerateLoginURL 里的完全一致
			"code":          code,
			"code_verifier": verifier,
		}).
		SetResult(&tokenResp).
		Post("https://api.etsy.com/v3/public/oauth/token")

	if err != nil {
		return nil, fmt.Errorf("网络请求发送失败: %v", err)
	}

	// 如果状态码不是 200，说明 Etsy 拒绝了，无论有没有 error 字段都算失败
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("ETSY 拒绝授权 (Status %d): %s", resp.StatusCode(), resp.String())
	}
	// 如果 Etsy 返回了业务逻辑错误
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("ETSY 业务错误: %s", tokenResp.Error)
	}

	return &tokenResp, nil
}

// 2. 获取 User ID
func (s *AuthService) fetchUserID(client *resty.Client, appKey, accessToken string) (int64, error) {
	type userMeResp struct {
		UserID int64 `json:"user_id"`
	}
	var res userMeResp

	// Etsy v3 必须要 x-api-key 和 Authorization
	resp, err := client.R().
		SetHeader("x-api-key", appKey).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetResult(&res).
		Get("https://api.etsy.com/v3/application/users/me")

	if err != nil {
		return 0, err
	}
	if res.UserID == 0 {
		return 0, fmt.Errorf("响应异常，未获取到 UserID: %s", resp.String())
	}
	return res.UserID, nil
}

// 3. 获取 Shop Info
func (s *AuthService) fetchShopInfo(client *resty.Client, appKey, accessToken string, userID int64) (*model.Shop, error) {
	type etsyShopResp struct {
		ShopID   int64  `json:"shop_id"`
		ShopName string `json:"shop_name"`
		UserID   int64  `json:"user_id"`
	}

	var res etsyShopResp

	url := fmt.Sprintf("https://api.etsy.com/v3/application/users/%d/shops", userID)

	resp, err := client.R().
		SetHeader("x-api-key", appKey).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetResult(&res).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("请求 Etsy 失败: %v", err)
	}

	if res.ShopName == "" {
		return nil, fmt.Errorf("解析失败或响应为空。原始返回: %s", resp.String())
	}

	return &model.Shop{
		EtsyShopID: res.ShopID,
		UserID:     res.UserID,
		ShopName:   res.ShopName,
	}, nil
}

func (s *AuthService) RefreshAccessToken(shop *model.Shop) error {
	// 1. 动态获取代理
	client := utils.NewProxiedClient(shop.Proxy)

	// 2. 发起刷新请求
	var tokenResp etsyTokenResp
	resp, err := client.R().
		SetFormData(map[string]string{
			"grant_type":    "refresh_token",
			"client_id":     shop.Developer.APIKey,
			"refresh_token": shop.RefreshToken,
		}).
		SetResult(&tokenResp).
		Post("https://api.etsy.com/v3/public/oauth/token")

	// --- 3. 错误处理与状态流转 (关键逻辑) ---

	// A. 网络层错误 (超时/DNS失败)
	if err != nil {
		// 策略：网络抖动不应该标记为 Token 失效，保持原状态，等待下一次 Cron 重试
		return fmt.Errorf("网络层错误，保持状态不变: %v", err)
	}

	// B. 业务层错误 (Etsy 拒绝)
	// 400 Bad Request (Invalid Grant) 或 401 Unauthorized 通常意味着 Refresh Token 已脏
	if resp.StatusCode() != 200 || tokenResp.Error != "" {
		// 策略：标记为 Invalid，前端看到这个状态会弹窗提示用户
		s.updateTokenStatus(shop, model.TokenStatusInvalid)
		return fmt.Errorf("刷新被拒，标记为失效: %s (Code: %d)", tokenResp.Error, resp.StatusCode())
	}

	// C. 成功
	shop.AccessToken = tokenResp.AccessToken
	shop.RefreshToken = tokenResp.RefreshToken
	shop.TokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	shop.TokenStatus = model.TokenStatusActive // 恢复为活跃

	// 只更新 Token 相关字段
	if err := s.ShopRepo.DB.Model(shop).
		Select("access_token", "refresh_token", "token_expires_at", "token_status").
		Updates(shop).Error; err != nil {
		return fmt.Errorf("入库失败: %v", err)
	}

	return nil
}

// 辅助方法：只更新状态
func (s *AuthService) updateTokenStatus(shop *model.Shop, status string) {
	shop.TokenStatus = status
	s.ShopRepo.DB.Model(shop).Update("token_status", status)
}
