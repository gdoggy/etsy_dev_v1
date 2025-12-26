package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
)

// OrderController 订单控制器
type OrderController struct {
	svc         *service.OrderService
	shipmentSvc *service.ShipmentService
}

// NewOrderController 创建订单控制器
func NewOrderController(svc *service.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

// SetShipmentService 设置发货服务（可选注入）
func (c *OrderController) SetShipmentService(svc *service.ShipmentService) {
	c.shipmentSvc = svc
}

// ==================== 订单列表与详情 ====================

// List 订单列表
// GET /api/orders
func (c *OrderController) List(ctx *gin.Context) {
	var req dto.ListOrdersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter := repository.OrderFilter{
		ShopID:   req.ShopID,
		Status:   req.Status,
		Keyword:  req.Keyword,
		Page:     req.Page,
		PageSize: req.PageSize,
	}

	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			filter.StartDate = &t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse("2006-01-02", req.EndDate); err == nil {
			filter.EndDate = &t
		}
	}

	orders, total, err := c.svc.List(ctx, filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 转换为列表项
	list := make([]dto.OrderListItem, len(orders))
	for i, o := range orders {
		list[i] = dto.OrderListItem{
			ID:              o.ID,
			EtsyReceiptID:   o.EtsyReceiptID,
			ShopID:          o.ShopID,
			ShopName:        "", // 列表不加载店铺名
			BuyerName:       o.BuyerName,
			Status:          o.Status,
			EtsyStatus:      o.EtsyStatus,
			ItemCount:       len(o.Items),
			TotalAmount:     o.GetGrandTotal(), // int64 → float64
			Currency:        o.Currency,
			ShippingCountry: o.GetShippingAddressField("country_iso"),
			HasShipment:     o.Shipment != nil,
			CreatedAt:       o.CreatedAt,
			PaidAt:          o.PaidAt,
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": dto.ListOrdersResponse{
			Total: total,
			List:  list,
		},
	})
}

// GetByID 获取订单详情
// GET /api/orders/:id
func (c *OrderController) GetByID(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	order, err := c.svc.GetByID(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "订单不存在"})
		return
	}

	// 构建响应
	resp := c.buildOrderDetailResponse(order)
	ctx.JSON(http.StatusOK, gin.H{"data": resp})
}

// ==================== 订单同步 ====================

// SyncOrders 同步订单
// POST /api/orders/sync
func (c *OrderController) SyncOrders(ctx *gin.Context) {
	var req dto.SyncOrdersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := c.svc.SyncOrders(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":    result,
		"message": "订单同步完成",
	})
}

// ==================== 订单状态更新 ====================

// UpdateStatus 更新订单状态
// PATCH /api/orders/:id/status
func (c *OrderController) UpdateStatus(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req dto.UpdateOrderStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.svc.UpdateOrderStatus(ctx, id, req.Status); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "状态已更新"})
}

// UpdateNote 更新订单备注
// PATCH /api/orders/:id/note
func (c *OrderController) UpdateNote(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	var req dto.UpdateOrderNoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.svc.UpdateOrderNote(ctx, id, req.MessageFromSeller); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "备注已更新"})
}

// ==================== 订单统计 ====================

// GetStats 获取订单统计
// GET /api/orders/stats
func (c *OrderController) GetStats(ctx *gin.Context) {
	var req dto.OrderStatsRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	stats, err := c.svc.GetOrderStats(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": stats})
}

// ==================== 订单发货信息 ====================

// GetShipment 获取订单的发货信息
// GET /api/orders/:order_id/shipment
func (c *OrderController) GetShipment(ctx *gin.Context) {
	orderID, err := strconv.ParseInt(ctx.Param("order_id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的订单ID"})
		return
	}

	if c.shipmentSvc == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "发货服务未配置"})
		return
	}

	shipment, err := c.shipmentSvc.GetByOrderID(ctx, orderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "发货记录不存在"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": &dto.ShipmentVO{
			ID:                 shipment.ID,
			CarrierCode:        shipment.CarrierCode,
			CarrierName:        shipment.CarrierName,
			TrackingNumber:     shipment.TrackingNumber,
			DestCarrierCode:    shipment.DestCarrierCode,
			DestTrackingNumber: shipment.DestTrackingNumber,
			LabelURL:           shipment.LabelURL,
			Status:             shipment.Status,
			EtsySynced:         shipment.EtsySynced,
			EtsySyncedAt:       shipment.EtsySyncedAt,
			CreatedAt:          shipment.CreatedAt,
		},
	})
}

// ==================== 响应构建 ====================

func (c *OrderController) buildOrderDetailResponse(order *service.OrderDetail) *dto.OrderDetailResponse {
	resp := &dto.OrderDetailResponse{
		Order: &dto.OrderVO{
			ID:                order.ID,
			EtsyReceiptID:     order.EtsyReceiptID,
			ShopID:            order.ShopID,
			ShopName:          order.ShopName,
			BuyerUserID:       order.BuyerUserID,
			BuyerEmail:        order.BuyerEmail,
			BuyerName:         order.BuyerName,
			Status:            order.Status,
			EtsyStatus:        order.EtsyStatus,
			MessageFromBuyer:  order.MessageFromBuyer,
			MessageFromSeller: order.MessageFromSeller,
			IsGift:            order.IsGift,
			GiftMessage:       order.GiftMessage,
			// 使用 Model 辅助方法：int64（分）→ float64（元）
			SubtotalAmount:   order.GetSubtotal(),
			ShippingAmount:   order.GetShipping(),
			TaxAmount:        order.GetTax(),
			DiscountAmount:   order.GetDiscount(),
			GrandTotalAmount: order.GetGrandTotal(),
			Currency:         order.Currency,
			CreatedAt:        order.CreatedAt,
			UpdatedAt:        order.UpdatedAt,
			PaidAt:           order.PaidAt,
			ShippedAt:        order.ShippedAt,
			EtsySyncedAt:     order.EtsySyncedAt,
		},
		ShippingAddress: &dto.ShippingAddressVO{
			Name:        order.GetShippingAddressField("name"),
			FirstLine:   order.GetShippingAddressField("first_line"),
			SecondLine:  order.GetShippingAddressField("second_line"),
			City:        order.GetShippingAddressField("city"),
			State:       order.GetShippingAddressField("state"),
			PostalCode:  order.GetShippingAddressField("zip"),
			CountryCode: order.GetShippingAddressField("country_iso"),
			CountryName: order.GetShippingAddressField("country_name"),
		},
	}

	// 订单项
	resp.Items = make([]dto.OrderItemVO, len(order.Items))
	for i, item := range order.Items {
		resp.Items[i] = dto.OrderItemVO{
			ID:                item.ID,
			EtsyTransactionID: item.EtsyTransactionID,
			ListingID:         item.ListingID,
			Title:             item.Title,
			SKU:               item.SKU,
			ImageURL:          item.ImageURL,
			Quantity:          item.Quantity,
			// 使用 Model 辅助方法：int64（分）→ float64（元）
			Price:        item.GetPrice(),
			ShippingCost: item.GetShippingCost(),
			Variations:   c.variationsToJSON(item.Variations),
		}
	}

	// 发货信息
	if order.Shipment != nil {
		resp.Shipment = &dto.ShipmentVO{
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

	return resp
}

// variationsToJSON 将变体 map 转换为 JSON 字符串
func (c *OrderController) variationsToJSON(v interface{}) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
