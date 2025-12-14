package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/pkg/utils"

	"github.com/go-resty/resty/v2"
)

// 业务常量
const (
	// CallbackURL 必须与 Etsy 后台填写的完全一致
	//CallbackURL = "http://localhost:8080/api/auth/callback"
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
	if shop.DeveloperID == nil || shop.Developer.ID == 0 {
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
		shop.Developer.AppKey, CallbackURL, scopes, state, challenge,
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

	// 4. 严谨校验配置完整性
	if shop.Proxy.ID == 0 {
		return nil, errors.New("该店铺未配置代理 IP")
	}
	if shop.Developer.ID == 0 || shop.Developer.AppKey == "" {
		return nil, errors.New("该店铺未绑定开发者账号或 AppKey 缺失")
	}
	// 5. 构造 HTTP 客户端 (使用 Proxy 表拼接 URL)
	// 格式通常为: protocol://user:pass@ip:port
	// 如果没有账号密码，格式为: protocol://ip:port
	var proxyURL string
	if shop.Proxy.Username != "" && shop.Proxy.Password != "" {
		proxyURL = fmt.Sprintf("%s://%s:%s@%s:%s",
			shop.Proxy.Protocol, shop.Proxy.Username, shop.Proxy.Password, shop.Proxy.IP, shop.Proxy.Port)
	} else {
		proxyURL = fmt.Sprintf("%s://%s:%s",
			shop.Proxy.Protocol, shop.Proxy.IP, shop.Proxy.Port)
	}

	//client := resty.New().SetProxy(proxyURL)
	fmt.Println(proxyURL)

	client := resty.New().SetDebug(true)

	// 6. 第一步：换取 Token
	tokenResp, err := s.exchangeToken(client, shop.Developer.AppKey, code, verifier)
	if err != nil {
		return nil, err
	}

	// 7. 第二步：查询当前用户 ID (User ID)
	userID, err := s.fetchUserID(client, shop.Developer.AppKey, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("获取 UserID 失败: %v", err)
	}

	// 8. 第三步：查询店铺信息 (Shop ID)
	shopInfo, err := s.fetchShopInfo(client, shop.Developer.AppKey, tokenResp.AccessToken, userID)
	if err != nil {
		return nil, fmt.Errorf("获取店铺信息失败: %v", err)
	}

	// 9. 更新数据
	shop.EtsyUserID = strconv.FormatInt(userID, 10)
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
	fmt.Println("\n=========== Token Exchange Debug ===========")
	fmt.Printf("1. Client ID (AppKey): [%s]\n", appKey)
	fmt.Printf("2. Redirect URI:       [%s]\n", CallbackURL)
	fmt.Printf("3. Code:               [%s...]\n", code[:10]) // 只打前10位
	fmt.Printf("4. Verifier:           [%s]\n", verifier)
	fmt.Println("============================================")

	// 强制设置 Content-Type，防止有些代理或服务器识别不了
	resp, err := client.R().
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetFormData(map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     appKey,
			"redirect_uri":  CallbackURL, // ⚠️ 必须与 GenerateLoginURL 里的完全一致
			"code":          code,
			"code_verifier": verifier,
		}).
		SetResult(&tokenResp).
		Post("https://api.etsy.com/v3/public/oauth/token")

	// 🛠️ 调试：打印最原始的响应结果
	fmt.Println("\n=========== Etsy Response Debug ===========")
	fmt.Printf("Status Code: %d\n", resp.StatusCode())
	fmt.Printf("Raw Body:    %s\n", resp.String())
	fmt.Printf("Error Obj:   %+v\n", tokenResp)
	fmt.Println("===========================================")

	if err != nil {
		return nil, fmt.Errorf("网络请求发送失败: %v", err)
	}

	// 如果状态码不是 200，说明 Etsy 拒绝了，无论有没有 error 字段都算失败
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("Etsy 拒绝授权 (Status %d): %s", resp.StatusCode(), resp.String())
	}

	// 如果 Etsy 返回了业务逻辑错误
	if tokenResp.Error != "" {
		return nil, fmt.Errorf("Etsy 业务错误: %s", tokenResp.Error)
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

	// 安全类型转换 (interface{} -> string)
	shopIDStr := strconv.FormatInt(res.ShopID, 10)
	userIDStr := strconv.FormatInt(res.UserID, 10)

	return &model.Shop{
		EtsyShopID: shopIDStr,
		EtsyUserID: userIDStr,
		ShopName:   res.ShopName,
	}, nil
}
