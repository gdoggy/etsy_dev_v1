package service

import (
	"context"
	"encoding/json"
	"errors"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/utils"
)

// 业务常量
const (
	// CallbackURL 必须与 Etsy 后台填写的完全一致
	// CallbackURL = "http://localhost:8080/api/auth/callback"
	// 测试用 URL
	CallbackURL  = "https://elizabet-avian-glenna.ngrok-free.dev/api/oauth/callback"
	EtsyTokenURL = "https://api.etsy.com/v3/public/oauth/token"
)

type AuthService struct {
	ShopRepo   *repository.ShopRepo
	dispatcher net.Dispatcher
}

// NewAuthService 工厂方法
func NewAuthService(shopRepo *repository.ShopRepo, dispatcher net.Dispatcher) *AuthService {
	return &AuthService{
		ShopRepo:   shopRepo,
		dispatcher: dispatcher,
	}
}

// GenerateLoginURL 生成授权链接
// 初次授权 将新建店铺，绑定相同 region 下 且 <2 个 shop的 developer
func (s *AuthService) GenerateLoginURL(ctx context.Context, shopID int64, region string) (string, error) {
	// 1. 查店铺
	var shop model.Shop
	if shopID == 0 {
		// 初次授权
		// TODO.
	} else {
		existingShop, err := s.ShopRepo.GetShopByID(ctx, shopID)
		if err != nil {
			return "", err
		}
		shop = *existingShop
	}

	// 2. 严格校验
	if shop.DeveloperID == 0 || shop.Developer.ID == 0 {
		return "", errors.New("该店铺未绑定开发者账号，请检查配置")
	}

	// 3. 生成 PKCE 安全参数
	verifier, _ := utils.GenerateRandomString(32)
	challenge := utils.GenerateCodeChallenge(verifier)
	state, _ := utils.GenerateRandomString(16)

	// 4. 缓存 Verifier (格式为 key=state, value="verifier:shop_id")
	cacheValue := fmt.Sprintf("%s:%d", verifier, shop.ID)
	utils.SetCache(state, cacheValue)

	// 5. 拼接 Etsy 官方授权 URL 获取所有权限
	scopes := "address_r address_w billing_r cart_r cart_w email_r favorites_r favorites_w feedback_r listings_r listings_w listings_d profile_r profile_w recommend_r recommend_w shops_r shops_w transactions_r transactions_w"
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
	// callback url 需要更新为 shop.Developer.CallbackURL
	authURL := fmt.Sprintf(
		"https://www.etsy.com/oauth/connect?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		shop.Developer.ApiKey, CallbackURL, scopes, state, challenge,
	)
	return authURL, nil
}

// HandleCallback 处理 Etsy 回调 -> 换 Token
func (s *AuthService) HandleCallback(ctx context.Context, code, state string) (*model.Shop, error) {
	var shop *model.Shop
	// 1. 校验 State 取缓存
	cachedVal, exists := utils.GetCache(state)
	if !exists {
		return shop, fmt.Errorf("授权超时或 State 无效，请重新发起")
	}

	// 2. 解析缓存 "verifier:shop_id"
	parts := strings.Split(cachedVal, ":")
	if len(parts) != 2 {
		return shop, fmt.Errorf("缓存数据格式错误，预期 'verifier:shopID'，实际: %s", cachedVal)
	}
	verifier := parts[0]
	shopID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return shop, fmt.Errorf("缓存中的 ShopID 无效: %v", err)
	}

	// 3. 查出 Shop 配置
	shop, err = s.ShopRepo.GetShopByID(ctx, shopID)
	if err != nil {
		log.Printf("get shop ID : %d err %v", shopID, err)
		return shop, err
	}
	// 4. 组装请求
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("client_id", shop.Developer.ApiKey)
	//data.Set("redirect_uri", shop.Developer.CallbackURL)
	data.Set("redirect_uri", CallbackURL)
	data.Set("code", code)
	data.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, "POST", EtsyTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return shop, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// 5. 通过 dispatcher 发送换取 Token
	tokenResp, err := s.dispatcher.Send(ctx, shopID, req)
	if err != nil {
		//s.updateTokenStatus(&shop, model.TokenStatusInvalid)
		return shop, fmt.Errorf("换取 Token 失败: %v", err)
	}
	defer tokenResp.Body.Close()

	// 6. 解析响应
	if tokenResp.StatusCode != 200 {
		return shop, fmt.Errorf("ETSY refused token exchange: status %d", tokenResp.StatusCode)
	}

	var etsyResp etsyTokenResp
	if err = json.NewDecoder(tokenResp.Body).Decode(&etsyResp); err != nil {
		return shop, fmt.Errorf("ETSY json decode failed: %v", err)
	}
	// 8. 更新数据
	shop.AccessToken = etsyResp.AccessToken
	shop.RefreshToken = etsyResp.RefreshToken
	shop.TokenExpiresAt = time.Now().Add(time.Duration(etsyResp.ExpiresIn) * time.Second)
	shop.TokenStatus = model.TokenStatusActive
	// 入库保存
	if err = s.ShopRepo.SaveOrUpdate(ctx, shop); err != nil {
		return shop, fmt.Errorf("店铺入库失败: %v", err)
	}

	return shop, nil
}

// 辅助结构体：Token 响应
type etsyTokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error,omitempty"`
}

// RefreshAccessToken 使用 Dispatcher 刷新 Token
func (s *AuthService) RefreshAccessToken(ctx context.Context, shop *model.Shop) error {
	// 1. 组装请求
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", shop.Developer.ApiKey)
	data.Set("refresh_token", shop.RefreshToken)

	req, _ := http.NewRequestWithContext(ctx, "POST", EtsyTokenURL, strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// 2. 托管发送
	// 注意：这里同样传入 shopID，保证和之前使用同一个代理 IP
	resp, err := s.dispatcher.Send(ctx, shop.ID, req)

	// A. 网络层错误
	if err != nil {
		return fmt.Errorf("refresh network error: %v", err)
	}
	defer resp.Body.Close()

	// B. 业务层错误 (Etsy 明确拒绝)
	if resp.StatusCode != 200 {
		// 只有明确收到 400/401 才标记为失效
		err = s.ShopRepo.UpdateTokenStatus(ctx, shop.ID, model.TokenStatusInvalid)
		return fmt.Errorf("refresh denied by ETSY: %d, err: %v", resp.StatusCode, err)
	}

	// C. 成功处理
	var tokenResp etsyTokenResp
	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	// 更新入库
	shop.AccessToken = tokenResp.AccessToken
	shop.RefreshToken = tokenResp.RefreshToken
	shop.TokenExpiresAt = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return s.ShopRepo.SaveOrUpdate(ctx, shop)
}
