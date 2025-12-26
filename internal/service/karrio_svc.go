package service

import (
	"bytes"
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KarrioConfig Karrio 配置
type KarrioConfig struct {
	BaseURL string // e.g. http://localhost:5002
	APIKey  string
	Timeout time.Duration
}

// KarrioClient Karrio API 客户端
type KarrioClient struct {
	config     KarrioConfig
	httpClient *http.Client
}

// NewKarrioClient 创建客户端
func NewKarrioClient(cfg KarrioConfig) *KarrioClient {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &KarrioClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// ==================== Shipment 运单 ====================

// CreateShipment 创建运单
func (c *KarrioClient) CreateShipment(ctx context.Context, req *dto.CreateShipmentRequest) (*dto.ShipmentResponse, error) {
	var resp dto.ShipmentResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/shipments", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("创建运单失败: %w", err)
	}
	return &resp, nil
}

// GetShipment 获取运单详情
func (c *KarrioClient) GetShipment(ctx context.Context, shipmentID string) (*dto.ShipmentResponse, error) {
	var resp dto.ShipmentResponse
	err := c.doRequest(ctx, http.MethodGet, "/v1/shipments/"+shipmentID, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("获取运单失败: %w", err)
	}
	return &resp, nil
}

// CancelShipment 取消运单
func (c *KarrioClient) CancelShipment(ctx context.Context, shipmentID string) error {
	err := c.doRequest(ctx, http.MethodPost, "/v1/shipments/"+shipmentID+"/cancel", nil, nil)
	if err != nil {
		return fmt.Errorf("取消运单失败: %w", err)
	}
	return nil
}

// ==================== Tracker 跟踪 ====================

// CreateTracker 创建跟踪器
func (c *KarrioClient) CreateTracker(ctx context.Context, req *dto.CreateTrackerRequest) (*dto.TrackerResponse, error) {
	var resp dto.TrackerResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/trackers", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("创建跟踪器失败: %w", err)
	}
	return &resp, nil
}

// BatchCreateTrackers 批量创建跟踪器
func (c *KarrioClient) BatchCreateTrackers(ctx context.Context, req *dto.BatchCreateTrackersRequest) ([]dto.TrackerResponse, error) {
	var resp struct {
		Trackers []dto.TrackerResponse `json:"trackers"`
	}
	err := c.doRequest(ctx, http.MethodPost, "/v1/batches/trackers", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("批量创建跟踪器失败: %w", err)
	}
	return resp.Trackers, nil
}

// GetTracker 获取跟踪详情
func (c *KarrioClient) GetTracker(ctx context.Context, trackerID string) (*dto.TrackerResponse, error) {
	var resp dto.TrackerResponse
	err := c.doRequest(ctx, http.MethodGet, "/v1/trackers/"+trackerID, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("获取跟踪详情失败: %w", err)
	}
	return &resp, nil
}

// RefreshTracker 刷新跟踪状态
func (c *KarrioClient) RefreshTracker(ctx context.Context, trackerID string) (*dto.TrackerResponse, error) {
	var resp dto.TrackerResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/trackers/"+trackerID+"/refresh", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("刷新跟踪状态失败: %w", err)
	}
	return &resp, nil
}

// ListTrackers 列出跟踪器
func (c *KarrioClient) ListTrackers(ctx context.Context, status string, limit, offset int) (*dto.KarrioListResponse[dto.TrackerResponse], error) {
	path := fmt.Sprintf("/v1/trackers?limit=%d&offset=%d", limit, offset)
	if status != "" {
		path += "&status=" + status
	}

	var resp dto.KarrioListResponse[dto.TrackerResponse]
	err := c.doRequest(ctx, http.MethodGet, path, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("列出跟踪器失败: %w", err)
	}
	return &resp, nil
}

// ==================== Rate 运费报价 ====================

// GetRates 获取运费报价
func (c *KarrioClient) GetRates(ctx context.Context, req *dto.RateRequest) (*dto.RateResponse, error) {
	var resp dto.RateResponse
	err := c.doRequest(ctx, http.MethodPost, "/v1/proxy/rates", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("获取运费报价失败: %w", err)
	}
	return &resp, nil
}

// ==================== Connection 连接管理 ====================

// CreateConnection 创建物流商连接
func (c *KarrioClient) CreateConnection(ctx context.Context, req *dto.CreateConnectionRequest) (*dto.CarrierConnection, error) {
	var resp dto.CarrierConnection
	err := c.doRequest(ctx, http.MethodPost, "/v1/connections", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("创建连接失败: %w", err)
	}
	return &resp, nil
}

// ListConnections 列出连接
func (c *KarrioClient) ListConnections(ctx context.Context) (*dto.KarrioListResponse[dto.CarrierConnection], error) {
	var resp dto.KarrioListResponse[dto.CarrierConnection]
	err := c.doRequest(ctx, http.MethodGet, "/v1/connections", nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("列出连接失败: %w", err)
	}
	return &resp, nil
}

// DeleteConnection 删除连接
func (c *KarrioClient) DeleteConnection(ctx context.Context, connectionID string) error {
	err := c.doRequest(ctx, http.MethodDelete, "/v1/connections/"+connectionID, nil, nil)
	if err != nil {
		return fmt.Errorf("删除连接失败: %w", err)
	}
	return nil
}

// ==================== HTTP 请求封装 ====================

func (c *KarrioClient) doRequest(ctx context.Context, method, path string, body, result interface{}) error {
	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求失败: %w", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.config.BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp dto.KarrioErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && len(errResp.Errors) > 0 {
			return fmt.Errorf("Karrio API 错误 [%d]: %s", resp.StatusCode, errResp.Errors[0].Message)
		}
		return fmt.Errorf("Karrio API 错误 [%d]: %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("解析响应失败: %w", err)
		}
	}

	return nil
}

// ==================== 健康检查 ====================

// Ping 检查 Karrio 服务状态
func (c *KarrioClient) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.BaseURL+"/", nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Karrio 服务不可用: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("Karrio 服务异常: HTTP %d", resp.StatusCode)
	}

	return nil
}
