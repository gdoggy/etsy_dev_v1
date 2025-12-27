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
// @Summary 获取订单列表
// @Description 分页查询订单，支持按店铺、状态、关键词、日期范围筛选
// @Tags Order (订单管理)
// @Accept json
// @Produce json
// @Param shop_id query int false "店铺ID"
// @Param status query string false "订单状态"
// @Param keyword query string false "关键词搜索（买家名/订单号）"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{} "{"data": dto.ListOrdersResponse}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/orders [get]
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

	list := make([]dto.OrderListItem, len(orders))
	for i, o := range orders {
		list[i] = dto.OrderListItem{
			ID:              o.ID,
			EtsyReceiptID:   o.EtsyReceiptID,
			ShopID:          o.ShopID,
			ShopName:        "",
			BuyerName:       o.BuyerName,
			Status:          o.Status,
			EtsyStatus:      o.EtsyStatus,
			ItemCount:       len(o.Items),
			TotalAmount:     o.GetGrandTotal(),
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
// @Summary 获取订单详情
// @Description 根据订单ID获取完整订单信息，包含订单项、收货地址、发货信息
// @Tags Order (订单管理)
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} map[string]interface{} "{"data": dto.OrderDetailResponse}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 404 {object} map[string]string "订单不存在"
// @Router /api/orders/{id} [get]
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

	resp := c.buildOrderDetailResponse(order)
	ctx.JSON(http.StatusOK, gin.H{"data": resp})
}

// ==================== 订单同步 ====================

// SyncOrders 同步订单
// @Summary 同步 Etsy 订单
// @Description 从 Etsy 拉取订单数据并同步到本地数据库
// @Tags Order (订单管理)
// @Accept json
// @Produce json
// @Param request body dto.SyncOrdersRequest true "同步参数"
// @Success 200 {object} map[string]interface{} "{"data": {}, "message": "订单同步完成"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "同步失败"
// @Router /api/orders/sync [post]
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
// @Summary 更新订单状态
// @Description 更新订单的处理状态（如标记为已处理、已取消等）
// @Tags Order (订单管理)
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param request body dto.UpdateOrderStatusRequest true "状态参数"
// @Success 200 {object} map[string]string "{"message": "状态已更新"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/orders/{id}/status [patch]
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
// @Summary 更新订单备注
// @Description 更新卖家对订单的备注信息
// @Tags Order (订单管理)
// @Accept json
// @Produce json
// @Param id path int true "订单ID"
// @Param request body dto.UpdateOrderNoteRequest true "备注内容"
// @Success 200 {object} map[string]string "{"message": "备注已更新"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/orders/{id}/note [patch]
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
// @Summary 获取订单统计
// @Description 获取订单统计数据（订单数、金额、状态分布等）
// @Tags Order (订单管理)
// @Produce json
// @Param shop_id query int false "店铺ID"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Success 200 {object} map[string]interface{} "{"data": dto.OrderStatsResponse}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/orders/stats [get]
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
// @Summary 获取订单发货信息
// @Description 获取指定订单的发货记录详情
// @Tags Order (订单管理)
// @Produce json
// @Param id path int true "订单ID"
// @Success 200 {object} map[string]interface{} "{"data": dto.ShipmentVO}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 404 {object} map[string]string "发货记录不存在"
// @Failure 503 {object} map[string]string "发货服务未配置"
// @Router /api/orders/{id}/shipment [get]
func (c *OrderController) GetShipment(ctx *gin.Context) {
	orderID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
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
			Price:             item.GetPrice(),
			ShippingCost:      item.GetShippingCost(),
			Variations:        c.variationsToJSON(item.Variations),
		}
	}

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
