package service

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"etsy_dev_v1_202512/core/model"
	"etsy_dev_v1_202512/core/repository"
	"etsy_dev_v1_202512/pkg/utils"

	"github.com/go-resty/resty/v2"
)

// 业务常量
const (
	MaxShopsPerAdapter = 3 // 风控：1个开发账号最多带3个店
	// CallbackURL 必须与 Etsy 后台填写的完全一致
	CallbackURL = "http://localhost:8080/api/auth/callback"
)

type AuthService struct {
	AdapterRepo *repository.AdapterRepository
	ShopRepo    *repository.ShopRepository
}

// NewAuthService 工厂方法
func NewAuthService(ar *repository.AdapterRepository, sr *repository.ShopRepository) *AuthService {
	return &AuthService{AdapterRepo: ar, ShopRepo: sr}
}

// GenerateLoginURL 生成授权链接 (核心风控逻辑)
func (s *AuthService) GenerateLoginURL() (string, error) {
	// 1. 智能调度：找一个没满员的 Adapter
	adapter, err := s.AdapterRepo.FindAvailableAdapter(MaxShopsPerAdapter)
	if err != nil {
		return "", errors.New("资源紧张：没有可用的开发者账号 (所有账号已满员或未启用)")
	}

	// 2. 生成 PKCE 安全参数
	verifier, _ := utils.GenerateRandomString(32)
	challenge := utils.GenerateCodeChallenge(verifier)
	state, _ := utils.GenerateRandomString(16)

	// 3. 缓存 Verifier (重要：格式为 "verifier:adapter_id")
	// 这样回调时我们就知道是哪个 Adapter 发起的请求
	cacheValue := fmt.Sprintf("%s:%d", verifier, adapter.ID)
	utils.SetCache(state, cacheValue)

	// 4. 拼接 Etsy 官方授权 URL
	// 权限: 读取商品、读取交易、更新交易(发货)、读取店铺信息
	scopes := "listings_r transactions_r transactions_w shops_r"
	authURL := fmt.Sprintf(
		"https://www.etsy.com/oauth/connect?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		adapter.EtsyAppKey, CallbackURL, scopes, state, challenge,
	)

	return authURL, nil
}

// HandleCallback 处理 Etsy 回调，换取 Token -> 查 User -> 查 Shop -> 入库
func (s *AuthService) HandleCallback(code, state string) (*model.Shop, error) {
	// 1. 校验 State 并取出缓存
	cachedVal, exists := utils.GetCache(state)
	if !exists {
		return nil, errors.New("授权超时或 State 无效，请重新发起")
	}

	// 2. 解析缓存 "verifier:adapter_id"
	var verifier string
	var adapterID uint
	_, err := fmt.Sscanf(cachedVal, "%s:%d", &verifier, &adapterID)
	if err != nil {
		return nil, errors.New("缓存数据损坏")
	}

	// 3. 查出 Adapter 详情 (为了拿 AppKey 和 Proxy)
	adapter, err := s.AdapterRepo.FindByID(adapterID)
	if err != nil {
		return nil, errors.New("找不到对应的 Adapter 记录")
	}

	// 4. 发起 HTTP 请求换取 Token
	// 注意：这里使用了 Adapter 绑定的专属 Proxy，防关联！
	client := resty.New().SetProxy(adapter.ProxyURL)

	tokenResp, err := s.exchangeToken(client, adapter, code, verifier)
	if err != nil {
		return nil, err
	}

	// 5. 第二步：查询当前用户 ID (User ID) -- 🟢 新增逻辑
	userID, err := s.fetchUserID(client, adapter.EtsyAppKey, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("获取 UserID 失败: %v", err)
	}

	// 6. 第三步：查询店铺信息 (Shop ID) -- 🟢 新增逻辑
	shopInfo, err := s.fetchShopInfo(client, adapter.EtsyAppKey, tokenResp.AccessToken, userID)
	if err != nil {
		// 容错：如果用户还没开店，可能查不到 Shop，这时候不应该报错，而是存个空或者标记
		// 这里为了严谨，如果没有店，我们可以先存个 0，或者直接报错提示用户先去开店
		// 既然是 ERP，默认用户是卖家，这里报错提示更合理
		return nil, fmt.Errorf("获取店铺失败(请确认该账号已在Etsy开通店铺): %v", err)
	}

	// 7. 组装真实数据并入库
	newShop := model.Shop{
		AdapterID:      adapter.ID,
		EtsyUserID:     strconv.FormatInt(userID, 10), // 存真实 UserID
		EtsyShopID:     shopInfo.EtsyShopID,           // 存真实 ShopID
		ShopName:       shopInfo.ShopName,             // 存真实店名
		AccessToken:    tokenResp.AccessToken,
		RefreshToken:   tokenResp.RefreshToken,
		TokenExpiresAt: time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	// 保存或更新 (如果该 EtsyShopID 已存在，应该更新 Token)
	if err := s.ShopRepo.SaveOrUpdate(&newShop); err != nil {
		return nil, err
	}

	return &newShop, nil
}

// 辅助结构体：Token 响应
type etsyTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

// 1. 换取 Token
func (s *AuthService) exchangeToken(client *resty.Client, adapter *model.Adapter, code, verifier string) (*etsyTokenResp, error) {
	var tokenResp etsyTokenResp
	resp, err := client.R().
		SetFormData(map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     adapter.EtsyAppKey,
			"redirect_uri":  CallbackURL,
			"code":          code,
			"code_verifier": verifier,
		}).
		SetResult(&tokenResp).
		Post("https://api.etsy.com/v3/public/oauth/token")

	if err != nil || tokenResp.Error != "" {
		return nil, fmt.Errorf("换取 Token 失败: %s", resp.String())
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
		return 0, fmt.Errorf("响应异常: %s", resp.String())
	}
	return res.UserID, nil
}

// 3. 获取 Shop Info
func (s *AuthService) fetchShopInfo(client *resty.Client, appKey, accessToken string, userID int64) (*model.Shop, error) {
	// Etsy 返回的是一个列表
	type shopNode struct {
		ShopID   int64  `json:"shop_id"`
		ShopName string `json:"shop_name"`
	}
	type shopListResp struct {
		Count   int        `json:"count"`
		Results []shopNode `json:"results"`
	}
	var res shopListResp

	url := fmt.Sprintf("https://api.etsy.com/v3/application/users/%d/shops", userID)
	_, err := client.R().
		SetHeader("x-api-key", appKey).
		SetHeader("Authorization", "Bearer "+accessToken).
		SetResult(&res).
		Get(url)

	if err != nil {
		return nil, err
	}

	// 检查该用户是否有店铺
	if res.Count == 0 || len(res.Results) == 0 {
		return nil, errors.New("该用户名下没有店铺")
	}

	// 返回第一个店铺的信息
	return &model.Shop{
		EtsyShopID: strconv.FormatInt(res.Results[0].ShopID, 10),
		ShopName:   res.Results[0].ShopName,
	}, nil
}
