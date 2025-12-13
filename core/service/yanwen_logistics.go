package service

import (
	"errors"

	"github.com/go-resty/resty/v2"
)

type YanwenService struct {
	ApiURL    string // API 地址 (生产或测试环境)
	UserToken string // 用户密钥 (即 UserId)
	ApiToken  string // 接口密钥 (即 ApiToken)
	Client    *resty.Client
}

// NewYanwenService 初始化
func NewYanwenService(url, userToken, apiToken string) *YanwenService {
	return &YanwenService{
		ApiURL:    url,
		UserToken: userToken,
		ApiToken:  apiToken,
		Client:    resty.New().SetDebug(true), // 开启调试模式，方便看请求日志
	}
}

// TestConnection 测试连通性 (占位)
func (s *YanwenService) TestConnection() error {
	// 具体的请求逻辑取决于协议是 JSON 还是 XML
	return errors.New("等待确认协议格式")
}
