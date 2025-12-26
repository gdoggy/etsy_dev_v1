package service

import (
	"bytes"
	"context"
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/net"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"gorm.io/datatypes"
)

const etsyBaseURL = "https://openapi.etsy.com/v3"

// ==================== 依赖接口 ====================

// ShopProvider 店铺信息提供者
type ShopProvider interface {
	GetByID(ctx context.Context, id int64) (*ShopInfo, error)
	GetByEtsyShopID(ctx context.Context, etsyShopID int64) (*ShopInfo, error)
}

// ShopInfo 店铺信息
type ShopInfo struct {
	ID           int64
	EtsyShopID   int64
	ShopName     string
	APIKey       string
	AccessToken  string
	CurrencyCode string
}

// ==================== OrderService ====================

// OrderService 订单服务
type OrderService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	shipmentRepo  repository.ShipmentRepository
	shopProvider  ShopProvider
	dispatcher    net.Dispatcher
}

// NewOrderService 创建订单服务
func NewOrderService(
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	shipmentRepo repository.ShipmentRepository,
	shopProvider ShopProvider,
	dispatcher net.Dispatcher,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		shipmentRepo:  shipmentRepo,
		shopProvider:  shopProvider,
		dispatcher:    dispatcher,
	}
}

// ==================== 订单列表 ====================

// ListOrders 获取订单列表
func (s *OrderService) ListOrders(ctx context.Context, req *dto.ListOrdersRequest) (*dto.ListOrdersResponse, error) {
	filter := repository.OrderFilter{
		ShopID:   req.ShopID,
		Status:   req.Status,
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	// 解析日期
	if req.StartDate != "" {
		t, err := time.Parse("2006-01-02", req.StartDate)
		if err == nil {
			filter.StartDate = &t
		}
	}
	if req.EndDate != "" {
		t, err := time.Parse("2006-01-02", req.EndDate)
		if err == nil {
			endOfDay := t.Add(24*time.Hour - time.Second)
			filter.EndDate = &endOfDay
		}
	}

	orders, total, err := s.orderRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("查询订单列表失败: %w", err)
	}

	// 获取店铺名称
	shop, _ := s.shopProvider.GetByID(ctx, req.ShopID)
	shopName := ""
	if shop != nil {
		shopName = shop.ShopName
	}

	// 转换为响应
	list := make([]dto.OrderListItem, len(orders))
	for i, order := range orders {
		list[i] = dto.OrderListItem{
			ID:              order.ID,
			EtsyReceiptID:   order.EtsyReceiptID,
			ShopID:          order.ShopID,
			ShopName:        shopName,
			BuyerName:       order.BuyerName,
			Status:          order.Status,
			EtsyStatus:      order.EtsyStatus,
			ItemCount:       len(order.Items),
			TotalAmount:     order.GetGrandTotal(),
			Currency:        order.Currency,
			ShippingCountry: s.getShippingCountry(&order),
			HasShipment:     order.Shipment != nil,
			CreatedAt:       order.CreatedAt,
			PaidAt:          order.PaidAt,
		}
	}

	return &dto.ListOrdersResponse{
		Total: total,
		List:  list,
	}, nil
}

// ==================== 订单详情 ====================

// GetOrderDetail 获取订单详情
func (s *OrderService) GetOrderDetail(ctx context.Context, orderID int64) (*dto.OrderDetailResponse, error) {
	order, err := s.orderRepo.GetByIDWithRelations(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("订单不存在")
	}

	// 获取店铺名称
	shop, _ := s.shopProvider.GetByID(ctx, order.ShopID)
	shopName := ""
	if shop != nil {
		shopName = shop.ShopName
	}

	// 转换订单 VO
	orderVO := &dto.OrderVO{
		ID:                order.ID,
		EtsyReceiptID:     order.EtsyReceiptID,
		ShopID:            order.ShopID,
		ShopName:          shopName,
		BuyerUserID:       order.BuyerUserID,
		BuyerEmail:        order.BuyerEmail,
		BuyerName:         order.BuyerName,
		Status:            order.Status,
		EtsyStatus:        order.EtsyStatus,
		MessageFromBuyer:  order.MessageFromBuyer,
		MessageFromSeller: order.MessageFromSeller,
		IsGift:            order.IsGift,
		GiftMessage:       order.GiftMessage,
		SubtotalAmount:    order.GetSubtotal(),
		ShippingAmount:    order.GetShipping(),
		TaxAmount:         order.GetTax(),
		DiscountAmount:    order.GetDiscount(),
		GrandTotalAmount:  order.GetGrandTotal(),
		Currency:          order.Currency,
		CreatedAt:         order.CreatedAt,
		UpdatedAt:         order.UpdatedAt,
		PaidAt:            order.PaidAt,
		ShippedAt:         order.ShippedAt,
		EtsySyncedAt:      order.EtsySyncedAt,
	}

	// 转换订单项 VO
	items := make([]dto.OrderItemVO, len(order.Items))
	for i, item := range order.Items {
		variations := ""
		if item.Variations != nil {
			if b, err := json.Marshal(item.Variations); err == nil {
				variations = string(b)
			}
		}
		items[i] = dto.OrderItemVO{
			ID:                item.ID,
			EtsyTransactionID: item.EtsyTransactionID,
			ListingID:         item.ListingID,
			Title:             item.Title,
			SKU:               item.SKU,
			ImageURL:          item.ImageURL,
			Quantity:          item.Quantity,
			Price:             item.GetPrice(),
			ShippingCost:      item.GetShippingCost(),
			Variations:        variations,
		}
	}

	// 转换收货地址
	var shippingAddress *dto.ShippingAddressVO
	if order.ShippingAddress != nil {
		shippingAddress = &dto.ShippingAddressVO{
			Name:        order.GetShippingAddressField("name"),
			FirstLine:   order.GetShippingAddressField("first_line"),
			SecondLine:  order.GetShippingAddressField("second_line"),
			City:        order.GetShippingAddressField("city"),
			State:       order.GetShippingAddressField("state"),
			PostalCode:  order.GetShippingAddressField("postal_code"),
			CountryCode: order.GetShippingAddressField("country_code"),
			CountryName: order.GetShippingAddressField("country_name"),
			Phone:       order.GetShippingAddressField("phone"),
		}
	}

	// 转换发货信息
	var shipmentVO *dto.ShipmentVO
	if order.Shipment != nil {
		shipmentVO = &dto.ShipmentVO{
			ID:                 order.Shipment.ID,
			CarrierCode:        order.Shipment.CarrierCode,
			CarrierName:        order.Shipment.CarrierName,
			TrackingNumber:     order.Shipment.TrackingNumber,
			DestCarrierCode:    order.Shipment.DestCarrierCode,
			DestTrackingNumber: order.Shipment.DestTrackingNumber,
			LabelURL:           order.Shipment.LabelURL,
			Status:             order.Shipment.Status,
			EtsySynced:         order.Shipment.EtsySynced,
			EtsySyncedAt:       order.Shipment.EtsySyncedAt,
			CreatedAt:          order.Shipment.CreatedAt,
		}
	}

	return &dto.OrderDetailResponse{
		Order:           orderVO,
		Items:           items,
		ShippingAddress: shippingAddress,
		Shipment:        shipmentVO,
	}, nil
}

// ==================== 订单同步 ====================

// SyncOrders 同步店铺订单
func (s *OrderService) SyncOrders(ctx context.Context, req *dto.SyncOrdersRequest) (*dto.SyncOrdersResponse, error) {
	shop, err := s.shopProvider.GetByID(ctx, req.ShopID)
	if err != nil {
		return nil, fmt.Errorf("店铺不存在: %w", err)
	}

	// 构建查询参数
	params := url.Values{}
	params.Set("limit", "100")
	params.Set("sort_on", "updated")
	params.Set("sort_order", "desc")

	// 设置时间范围
	if req.MinCreated != "" {
		if ts, err := parseTimestamp(req.MinCreated); err == nil {
			params.Set("min_created", strconv.FormatInt(ts, 10))
		}
	}
	if req.MaxCreated != "" {
		if ts, err := parseTimestamp(req.MaxCreated); err == nil {
			params.Set("max_created", strconv.FormatInt(ts, 10))
		}
	}

	// 如果不是强制同步，默认只获取最近7天的订单
	if !req.ForceSync && req.MinCreated == "" {
		sevenDaysAgo := time.Now().AddDate(0, 0, -7).Unix()
		params.Set("min_created", strconv.FormatInt(sevenDaysAgo, 10))
	}

	// 调用 Etsy API
	apiURL := fmt.Sprintf("%s/application/shops/%d/receipts?%s", etsyBaseURL, shop.EtsyShopID, params.Encode())
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodGet, apiURL, nil, shop.APIKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}

	resp, err := s.dispatcher.Send(ctx, req.ShopID, httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求 Etsy API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Etsy API 错误 [%d]: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var etsyResp struct {
		Count   int                   `json:"count"`
		Results []dto.EtsyReceiptData `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&etsyResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	result := &dto.SyncOrdersResponse{
		TotalFetched: etsyResp.Count,
	}

	// 处理订单
	for _, receipt := range etsyResp.Results {
		isNew, err := s.syncSingleOrder(ctx, req.ShopID, &receipt)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("订单 %d 同步失败: %v", receipt.ReceiptID, err))
			continue
		}
		if isNew {
			result.NewOrders++
		} else {
			result.UpdatedOrders++
		}
	}

	return result, nil
}

// syncSingleOrder 同步单个订单
func (s *OrderService) syncSingleOrder(ctx context.Context, shopID int64, receipt *dto.EtsyReceiptData) (bool, error) {
	// 检查订单是否已存在
	existing, _ := s.orderRepo.GetByEtsyReceiptID(ctx, receipt.ReceiptID)
	isNew := existing == nil

	// 解析 Etsy 状态
	etsyStatus := s.parseEtsyStatus(receipt)

	// 构建订单
	order := &model.Order{
		EtsyReceiptID:     receipt.ReceiptID,
		ShopID:            shopID,
		BuyerUserID:       receipt.BuyerUserID,
		BuyerEmail:        receipt.BuyerEmail,
		BuyerName:         receipt.Name,
		EtsyStatus:        etsyStatus,
		MessageFromBuyer:  receipt.MessageFromBuyer,
		MessageFromSeller: receipt.MessageFromSeller,
		IsGift:            receipt.IsGift,
		GiftMessage:       receipt.GiftMessage,
		ShippingAddress: datatypes.JSONMap{
			"name":         receipt.Name,
			"first_line":   receipt.FirstLine,
			"second_line":  receipt.SecondLine,
			"city":         receipt.City,
			"state":        receipt.State,
			"postal_code":  receipt.Zip,
			"country_code": receipt.CountryISO,
		},
		SubtotalAmount:   int64(receipt.Subtotal.Amount * 100 / maxInt(receipt.Subtotal.Divisor, 1)),
		ShippingAmount:   int64(receipt.TotalShippingCost.Amount * 100 / maxInt(receipt.TotalShippingCost.Divisor, 1)),
		TaxAmount:        int64(receipt.TotalTaxCost.Amount * 100 / maxInt(receipt.TotalTaxCost.Divisor, 1)),
		DiscountAmount:   int64(receipt.DiscountAmt.Amount * 100 / maxInt(receipt.DiscountAmt.Divisor, 1)),
		GrandTotalAmount: int64(receipt.GrandTotal.Amount * 100 / maxInt(receipt.GrandTotal.Divisor, 1)),
		Currency:         receipt.GrandTotal.CurrencyCode,
		PaymentMethod:    receipt.PaymentMethod,
		IsPaid:           receipt.IsPaid,
		IsShipped:        receipt.IsShipped,
	}

	// 设置 ERP 状态
	if isNew {
		order.Status = model.OrderStatusPending
	} else {
		order.ID = existing.ID
		order.Status = existing.Status // 保留 ERP 状态
		order.CreatedAt = existing.CreatedAt
	}

	// 解析时间戳
	if receipt.CreateTimestamp > 0 {
		t := time.Unix(receipt.CreateTimestamp, 0)
		order.EtsyCreatedAt = &t
	}
	if receipt.UpdateTimestamp > 0 {
		t := time.Unix(receipt.UpdateTimestamp, 0)
		order.EtsyUpdatedAt = &t
	}
	if receipt.IsPaid {
		if order.PaidAt == nil && order.EtsyCreatedAt != nil {
			order.PaidAt = order.EtsyCreatedAt
		}
	}

	// 同步时间
	now := time.Now()
	order.EtsySyncedAt = &now

	// 保存原始数据
	if rawData, err := json.Marshal(receipt); err == nil {
		order.EtsyRawData = datatypes.JSON(rawData)
	}

	// 保存订单
	if isNew {
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return false, err
		}
	} else {
		if err := s.orderRepo.Update(ctx, order); err != nil {
			return false, err
		}
	}

	// 同步订单项
	if err := s.syncOrderItems(ctx, order.ID, receipt.Transactions); err != nil {
		return isNew, err
	}

	return isNew, nil
}

// syncOrderItems 同步订单项
func (s *OrderService) syncOrderItems(ctx context.Context, orderID int64, transactions []dto.EtsyTransactionData) error {
	for _, tx := range transactions {
		existing, _ := s.orderItemRepo.GetByEtsyTransactionID(ctx, tx.TransactionID)

		item := &model.OrderItem{
			OrderID:           orderID,
			EtsyTransactionID: tx.TransactionID,
			ListingID:         tx.ListingID,
			ProductID:         tx.ProductID,
			Title:             tx.Title,
			SKU:               tx.SKU,
			Quantity:          tx.Quantity,
			PriceAmount:       int64(tx.Price.Amount * 100 / maxInt(tx.Price.Divisor, 1)),
			ShippingCost:      int64(tx.ShippingCost.Amount * 100 / maxInt(tx.ShippingCost.Divisor, 1)),
			Currency:          tx.Price.CurrencyCode,
			ListingImageID:    tx.ListingImageID,
			IsDigital:         tx.IsDigital,
		}

		// 解析变体
		if len(tx.Variations) > 0 {
			variations := make(datatypes.JSONMap)
			for _, v := range tx.Variations {
				variations[v.FormattedName] = v.FormattedValue
			}
			item.Variations = variations
		}

		// 解析时间
		if tx.PaidTimestamp > 0 {
			t := time.Unix(tx.PaidTimestamp, 0)
			item.PaidAt = &t
		}
		if tx.ShippedTimestamp > 0 {
			t := time.Unix(tx.ShippedTimestamp, 0)
			item.ShippedAt = &t
		}

		if existing == nil {
			if err := s.orderItemRepo.Create(ctx, item); err != nil {
				return err
			}
		} else {
			item.ID = existing.ID
			item.CreatedAt = existing.CreatedAt
			if err := s.orderItemRepo.Update(ctx, item); err != nil {
				return err
			}
		}
	}

	return nil
}

// ==================== 同步发货到 Etsy ====================

// SyncShipmentToEtsy 同步发货信息到 Etsy
func (s *OrderService) SyncShipmentToEtsy(ctx context.Context, orderID int64) error {
	order, err := s.orderRepo.GetByIDWithRelations(ctx, orderID)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}

	if order.Shipment == nil {
		return fmt.Errorf("订单未发货")
	}

	if order.Shipment.EtsySynced {
		return nil // 已同步
	}

	shop, err := s.shopProvider.GetByID(ctx, order.ShopID)
	if err != nil {
		return fmt.Errorf("获取店铺信息失败: %w", err)
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"tracking_code": order.Shipment.TrackingNumber,
		"carrier_name":  mapCarrierToEtsy(order.Shipment.CarrierCode),
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// 调用 Etsy API
	apiURL := fmt.Sprintf("%s/application/shops/%d/receipts/%d/tracking",
		etsyBaseURL, shop.EtsyShopID, order.EtsyReceiptID)
	httpReq, err := net.BuildEtsyRequest(ctx, http.MethodPost, apiURL, bytes.NewReader(bodyBytes), shop.APIKey, shop.AccessToken)
	if err != nil {
		return fmt.Errorf("构建请求失败: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.dispatcher.Send(ctx, order.ShopID, httpReq)
	if err != nil {
		s.shipmentRepo.MarkEtsySyncFailed(ctx, order.Shipment.ID, err.Error())
		return fmt.Errorf("请求 Etsy API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		s.shipmentRepo.MarkEtsySyncFailed(ctx, order.Shipment.ID, errMsg)
		return fmt.Errorf("Etsy API 错误: %s", errMsg)
	}

	// 标记同步成功
	return s.shipmentRepo.MarkEtsySynced(ctx, order.Shipment.ID)
}

// ==================== 订单状态更新 ====================

// UpdateOrderStatus 更新订单状态
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID int64, status string) error {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}

	// 状态校验
	switch status {
	case model.OrderStatusProcessing:
		if !order.CanProcess() {
			return fmt.Errorf("订单状态不允许处理")
		}
	case model.OrderStatusCanceled:
		if !order.CanCancel() {
			return fmt.Errorf("订单状态不允许取消")
		}
	}

	return s.orderRepo.UpdateStatus(ctx, orderID, status)
}

// UpdateOrderNote 更新订单备注
func (s *OrderService) UpdateOrderNote(ctx context.Context, orderID int64, message string) error {
	return s.orderRepo.UpdateFields(ctx, orderID, map[string]interface{}{
		"message_from_seller": message,
	})
}

// ==================== 批量操作 ====================

// BatchUpdateOrderStatus 批量更新订单状态
func (s *OrderService) BatchUpdateOrderStatus(ctx context.Context, orderIDs []int64, status string) (*dto.BatchOperationResponse, error) {
	result := &dto.BatchOperationResponse{}

	for _, id := range orderIDs {
		if err := s.UpdateOrderStatus(ctx, id, status); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("订单 %d: %v", id, err))
		} else {
			result.Success++
		}
	}

	return result, nil
}

// ==================== 订单统计 ====================

// GetOrderStats 获取订单统计
func (s *OrderService) GetOrderStats(ctx context.Context, req *dto.OrderStatsRequest) (*dto.OrderStatsResponse, error) {
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		return nil, fmt.Errorf("起始日期格式错误")
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("结束日期格式错误")
	}
	endDate = endDate.Add(24*time.Hour - time.Second)

	stats, err := s.orderRepo.GetStats(ctx, req.ShopID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	shop, _ := s.shopProvider.GetByID(ctx, req.ShopID)
	currency := "USD"
	if shop != nil && shop.CurrencyCode != "" {
		currency = shop.CurrencyCode
	}

	avgOrderValue := float64(0)
	if stats.TotalOrders > 0 {
		avgOrderValue = float64(stats.TotalAmount) / float64(stats.TotalOrders) / 100
	}

	return &dto.OrderStatsResponse{
		TotalOrders:      int(stats.TotalOrders),
		TotalAmount:      float64(stats.TotalAmount) / 100,
		Currency:         currency,
		PendingOrders:    int(stats.PendingOrders),
		ProcessingOrders: int(stats.ProcessingOrders),
		ShippedOrders:    int(stats.ShippedOrders),
		DeliveredOrders:  int(stats.DeliveredOrders),
		CanceledOrders:   int(stats.CanceledOrders),
		AvgOrderValue:    avgOrderValue,
	}, nil
}

// ==================== 辅助方法 ====================

func (s *OrderService) parseEtsyStatus(receipt *dto.EtsyReceiptData) string {
	if receipt.Status == "Canceled" {
		return model.EtsyStatusCanceled
	}
	if receipt.IsShipped {
		return model.EtsyStatusCompleted
	}
	if receipt.IsPaid {
		return model.EtsyStatusPaid
	}
	return model.EtsyStatusOpen
}

func (s *OrderService) getShippingCountry(order *model.Order) string {
	if order.ShippingAddress == nil {
		return ""
	}
	return order.GetShippingAddressField("country_code")
}

func parseTimestamp(s string) (int64, error) {
	if ts, err := time.Parse("2006-01-02", s); err == nil {
		return ts.Unix(), nil
	}
	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts.Unix(), nil
	}
	return 0, fmt.Errorf("无法解析时间: %s", s)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// mapCarrierToEtsy 将物流商代码映射为 Etsy 支持的格式
func mapCarrierToEtsy(carrierCode string) string {
	carriers := map[string]string{
		//"yanwen":      "china-ems",
		//"wanbang":     "china-ems",
		//"yunexpress":  "china-ems",
		//"cainiao":     "cainiao",
		//"sfexpress":   "sf-express",
		"usps":        "usps",
		"ups":         "ups",
		"fedex":       "fedex",
		"dhl":         "dhl",
		"dhl-express": "dhl",
	}
	if mapped, ok := carriers[carrierCode]; ok {
		return mapped
	}
	return "other"
}
