package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/pkg/net"
)

// ==================== 数据传输对象 ====================

// OrderDetail 订单详情（用于控制器响应）
type OrderDetail struct {
	*model.Order
	ShopName string
}

// ==================== Service 实现 ====================

// OrderService 订单服务
type OrderService struct {
	orderRepo    repository.OrderRepository
	itemRepo     repository.OrderItemRepository
	shipmentRepo repository.ShipmentRepository
	shopRepo     repository.ShopRepository
	dispatcher   net.Dispatcher
}

// NewOrderService 创建订单服务
func NewOrderService(
	orderRepo repository.OrderRepository,
	itemRepo repository.OrderItemRepository,
	shipmentRepo repository.ShipmentRepository,
	shopRepo repository.ShopRepository,
	dispatcher net.Dispatcher,
) *OrderService {
	return &OrderService{
		orderRepo:    orderRepo,
		itemRepo:     itemRepo,
		shipmentRepo: shipmentRepo,
		shopRepo:     shopRepo,
		dispatcher:   dispatcher,
	}
}

// ==================== 查询 ====================

// List 订单列表
func (s *OrderService) List(ctx context.Context, filter repository.OrderFilter) ([]model.Order, int64, error) {
	return s.orderRepo.List(ctx, filter)
}

// GetByID 获取订单详情
func (s *OrderService) GetByID(ctx context.Context, id int64) (*OrderDetail, error) {
	order, err := s.orderRepo.GetByIDWithRelations(ctx, id)
	if err != nil {
		return nil, err
	}

	detail := &OrderDetail{Order: order}

	// 获取店铺名称
	if shop, err := s.shopRepo.GetByID(ctx, order.ShopID); err == nil {
		detail.ShopName = shop.ShopName
	}

	return detail, nil
}

// GetByEtsyReceiptID 根据 Etsy Receipt ID 获取订单
func (s *OrderService) GetByEtsyReceiptID(ctx context.Context, shopID, receiptID int64) (*model.Order, error) {
	return s.orderRepo.GetByEtsyReceiptID(ctx, shopID, receiptID)
}

// ==================== 同步 ====================

// SyncOrders 从 Etsy 同步订单（供 Task 调用）
func (s *OrderService) SyncOrders(ctx context.Context, req *dto.SyncOrdersRequest) (*dto.SyncOrdersResponse, error) {
	return s.SyncFromEtsy(ctx, req.ShopID, req.MinCreated, req.MaxCreated, req.ForceSync)
}

// SyncFromEtsy 从 Etsy 同步订单
func (s *OrderService) SyncFromEtsy(ctx context.Context, shopID int64, minCreated, maxCreated string, forceSync bool) (*dto.SyncOrdersResponse, error) {
	result := &dto.SyncOrdersResponse{}

	// 获取店铺
	shop, err := s.shopRepo.GetByID(ctx, shopID)
	if err != nil {
		return nil, fmt.Errorf("店铺不存在: %v", err)
	}

	// 构建 Etsy API 请求
	params := map[string]string{
		"limit": "100",
	}
	if minCreated != "" {
		params["min_created"] = minCreated
	}
	if maxCreated != "" {
		params["max_created"] = maxCreated
	}
	url := fmt.Sprintf("%s/shops/%d/receipts?%s", EtsyAPIBaseURL, shop.EtsyShopID, params)
	req, err := net.BuildEtsyRequest(ctx, http.MethodGet, url, nil, shop.Developer.ApiKey, shop.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}

	// 调用 Etsy API
	resp, err := s.dispatcher.Send(ctx, shopID, req)
	if err != nil {
		return nil, fmt.Errorf("调用 Etsy API 失败: %v", err)
	}

	// 解析响应
	var etsyResp struct {
		Count   int                   `json:"count"`
		Results []dto.EtsyReceiptData `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&etsyResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	result.TotalFetched = len(etsyResp.Results)

	// 处理每个订单
	for _, receipt := range etsyResp.Results {
		// 检查是否是新订单
		existing, _ := s.orderRepo.GetByEtsyReceiptID(ctx, shopID, receipt.ReceiptID)
		isNew := existing == nil

		if err := s.processReceipt(ctx, shopID, &receipt, existing, forceSync); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("订单 %d: %v", receipt.ReceiptID, err))
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

// processReceipt 处理单个 Etsy Receipt
func (s *OrderService) processReceipt(ctx context.Context, shopID int64, receipt *dto.EtsyReceiptData, existing *model.Order, forceSync bool) error {
	// 构建订单模型
	order := s.buildOrderFromReceipt(shopID, receipt)

	if existing != nil {
		// 更新
		order.ID = existing.ID
		if !forceSync && existing.EtsyUpdatedAt != nil && order.EtsyUpdatedAt != nil {
			if existing.EtsyUpdatedAt.After(*order.EtsyUpdatedAt) {
				return nil // 本地更新时间更新，跳过
			}
		}
		if err := s.orderRepo.Update(ctx, order); err != nil {
			return err
		}
	} else {
		// 新建
		if err := s.orderRepo.Create(ctx, order); err != nil {
			return err
		}
	}

	// 处理订单项
	for _, tx := range receipt.Transactions {
		item := s.buildOrderItemFromTransaction(order.ID, &tx)
		if existing != nil {
			// 更新或创建订单项
			existingItem, _ := s.itemRepo.GetByEtsyTransactionID(ctx, tx.TransactionID)
			if existingItem != nil {
				item.ID = existingItem.ID
				_ = s.itemRepo.Update(ctx, item)
			} else {
				_ = s.itemRepo.Create(ctx, item)
			}
		} else {
			_ = s.itemRepo.Create(ctx, item)
		}
	}

	return nil
}

// buildOrderFromReceipt 从 Etsy Receipt 构建订单
func (s *OrderService) buildOrderFromReceipt(shopID int64, receipt *dto.EtsyReceiptData) *model.Order {
	// 构建收货地址 JSON
	shippingAddr := map[string]interface{}{
		"name":        receipt.Name,
		"first_line":  receipt.FirstLine,
		"second_line": receipt.SecondLine,
		"city":        receipt.City,
		"state":       receipt.State,
		"zip":         receipt.Zip,
		"country_iso": receipt.CountryISO,
	}

	// 确定订单状态
	status := s.mapEtsyStatus(receipt)

	// 时间转换
	var paidAt, etsyCreatedAt, etsyUpdatedAt *time.Time
	if receipt.IsPaid && receipt.CreateTimestamp > 0 {
		t := time.Unix(receipt.CreateTimestamp, 0)
		paidAt = &t
	}
	if receipt.CreatedTimestamp > 0 {
		t := time.Unix(receipt.CreatedTimestamp, 0)
		etsyCreatedAt = &t
	}
	if receipt.UpdatedTimestamp > 0 {
		t := time.Unix(receipt.UpdatedTimestamp, 0)
		etsyUpdatedAt = &t
	}

	// 存储原始数据
	rawData, _ := json.Marshal(receipt)

	order := &model.Order{
		ShopID:            shopID,
		EtsyReceiptID:     receipt.ReceiptID,
		BuyerUserID:       receipt.BuyerUserID,
		BuyerEmail:        receipt.BuyerEmail,
		BuyerName:         receipt.Name,
		Status:            status,
		EtsyStatus:        receipt.Status,
		MessageFromBuyer:  receipt.MessageFromBuyer,
		MessageFromSeller: receipt.MessageFromSeller,
		IsGift:            receipt.IsGift,
		GiftMessage:       receipt.GiftMessage,
		IsPaid:            receipt.IsPaid,
		IsShipped:         receipt.IsShipped,
		PaymentMethod:     receipt.PaymentMethod,

		// 金额转换：Etsy Money → int64（分）
		SubtotalAmount:   s.etsyMoneyToCents(receipt.Subtotal),
		ShippingAmount:   s.etsyMoneyToCents(receipt.TotalShippingCost),
		TaxAmount:        s.etsyMoneyToCents(receipt.TotalTaxCost),
		DiscountAmount:   s.etsyMoneyToCents(receipt.DiscountAmt),
		GrandTotalAmount: s.etsyMoneyToCents(receipt.GrandTotal),
		Currency:         receipt.GrandTotal.CurrencyCode,

		ShippingAddress: shippingAddr,
		EtsyRawData:     rawData,
		PaidAt:          paidAt,
		EtsyCreatedAt:   etsyCreatedAt,
		EtsyUpdatedAt:   etsyUpdatedAt,
	}

	// 同步时间
	now := time.Now()
	order.EtsySyncedAt = &now

	return order
}

// etsyMoneyToCents 将 Etsy Money 对象转换为分（int64）
func (s *OrderService) etsyMoneyToCents(m dto.EtsyMoney) int64 {
	if m.Divisor == 0 {
		return 0
	}
	// Etsy Money: amount / divisor = 实际金额（元）
	// 我们存储分，所以需要：amount * 100 / divisor
	return int64(m.Amount) * 100 / int64(m.Divisor)
}

// buildOrderItemFromTransaction 从 Etsy Transaction 构建订单项
func (s *OrderService) buildOrderItemFromTransaction(orderID int64, tx *dto.EtsyTransactionData) *model.OrderItem {
	// 构建变体信息
	var variations map[string]interface{}
	if len(tx.Variations) > 0 {
		variations = make(map[string]interface{})
		for _, v := range tx.Variations {
			variations[v.FormattedName] = v.FormattedValue
		}
	}

	// 时间转换
	var paidAt, shippedAt *time.Time
	if tx.PaidTimestamp > 0 {
		t := time.Unix(tx.PaidTimestamp, 0)
		paidAt = &t
	}
	if tx.ShippedTimestamp > 0 {
		t := time.Unix(tx.ShippedTimestamp, 0)
		shippedAt = &t
	}

	return &model.OrderItem{
		OrderID:           orderID,
		EtsyTransactionID: tx.TransactionID,
		ListingID:         tx.ListingID,
		ProductID:         tx.ProductID,
		Title:             tx.Title,
		SKU:               tx.SKU,
		Quantity:          tx.Quantity,
		ListingImageID:    tx.ListingImageID,
		IsDigital:         tx.IsDigital,
		PaidAt:            paidAt,
		ShippedAt:         shippedAt,

		// 金额转换：Etsy Money → int64（分）
		PriceAmount:  s.etsyMoneyToCents(tx.Price),
		ShippingCost: s.etsyMoneyToCents(tx.ShippingCost),
		Currency:     tx.Price.CurrencyCode,

		Variations: variations,
	}
}

// mapEtsyStatus 映射 Etsy 状态到内部状态
func (s *OrderService) mapEtsyStatus(receipt *dto.EtsyReceiptData) string {
	if receipt.IsShipped {
		return model.OrderStatusShipped
	}
	if receipt.IsPaid {
		return model.OrderStatusPending
	}
	return model.OrderStatusPending
}

// ==================== 状态更新 ====================

// UpdateOrderStatus 更新订单状态
func (s *OrderService) UpdateOrderStatus(ctx context.Context, id int64, status string) error {
	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("订单不存在")
	}

	// 验证状态转换
	if !s.isValidStatusTransition(order.Status, status) {
		return fmt.Errorf("无效的状态转换: %s -> %s", order.Status, status)
	}

	return s.orderRepo.UpdateFields(ctx, id, map[string]interface{}{
		"status": status,
	})
}

// UpdateOrderNote 更新订单备注
func (s *OrderService) UpdateOrderNote(ctx context.Context, id int64, note string) error {
	return s.orderRepo.UpdateFields(ctx, id, map[string]interface{}{
		"message_from_seller": note,
	})
}

// isValidStatusTransition 验证状态转换是否有效
func (s *OrderService) isValidStatusTransition(from, to string) bool {
	validTransitions := map[string][]string{
		model.OrderStatusPending:    {model.OrderStatusProcessing, model.OrderStatusCanceled},
		model.OrderStatusProcessing: {model.OrderStatusShipped, model.OrderStatusCanceled},
		model.OrderStatusShipped:    {model.OrderStatusDelivering, model.OrderStatusDelivered},
		model.OrderStatusDelivering: {model.OrderStatusDelivered},
		model.OrderStatusDelivered:  {},
		model.OrderStatusCanceled:   {},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// ==================== 统计 ====================

// GetOrderStats 获取订单统计
func (s *OrderService) GetOrderStats(ctx context.Context, req *dto.OrderStatsRequest) (*dto.OrderStatsResponse, error) {
	var start, end time.Time
	if req.StartDate != "" {
		t, _ := time.Parse("2006-01-02", req.StartDate)
		start = t
	}
	if req.EndDate != "" {
		t, _ := time.Parse("2006-01-02", req.EndDate)
		end = t
	}

	stats, err := s.orderRepo.GetStats(ctx, req.ShopID, start, end)
	if err != nil {
		return nil, err
	}

	return &dto.OrderStatsResponse{
		TotalOrders:      stats.TotalOrders,
		TotalAmount:      float64(stats.TotalAmount) / 100, // int64→float64
		Currency:         stats.Currency,
		PendingOrders:    stats.PendingOrders,
		ProcessingOrders: stats.ProcessingOrders,
		ShippedOrders:    stats.ShippedOrders,
		DeliveredOrders:  stats.DeliveredOrders,
		CanceledOrders:   stats.CanceledOrders,
		AvgOrderValue:    float64(stats.AvgOrderValue) / 100,
	}, nil
}

// ==================== 批量获取 ====================

// GetPendingShipmentOrders 获取待发货订单
func (s *OrderService) GetPendingShipmentOrders(ctx context.Context, shopID int64, limit int) ([]model.Order, error) {
	filter := repository.OrderFilter{
		ShopID:   shopID,
		Status:   model.OrderStatusPending,
		PageSize: limit,
	}
	orders, _, err := s.orderRepo.List(ctx, filter)
	return orders, err
}
