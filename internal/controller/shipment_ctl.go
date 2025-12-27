package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ShipmentController 发货控制器
type ShipmentController struct {
	svc *service.ShipmentService
}

// NewShipmentController 创建发货控制器
func NewShipmentController(svc *service.ShipmentService) *ShipmentController {
	return &ShipmentController{svc: svc}
}

// ==================== 发货管理 ====================

// Create 创建发货
// @Summary 创建发货记录
// @Description 为订单创建发货记录，填写物流商和运单号
// @Tags Shipment (发货管理)
// @Accept json
// @Produce json
// @Param request body dto.CreateEtsyShipmentRequest true "发货参数"
// @Success 201 {object} map[string]interface{} "{"data": dto.EtsyShipmentResponse, "message": "发货成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/shipments [post]
func (c *ShipmentController) Create(ctx *gin.Context) {
	var req dto.CreateEtsyShipmentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shipment, err := c.svc.CreateShipment(ctx, req.OrderID, req.CarrierCode, req.ServiceCode, req.TrackingNumber, req.Weight)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"data":    c.toResponse(shipment),
		"message": "发货成功",
	})
}

// CreateWithLabel 创建发货并生成面单
// @Summary 创建发货并生成面单
// @Description 为订单创建发货，自动生成物流面单
// @Tags Shipment (发货管理)
// @Accept json
// @Produce json
// @Param request body dto.CreateLabelRequest true "面单参数"
// @Success 201 {object} map[string]interface{} "{"data": dto.EtsyShipmentResponse, "message": "发货成功，面单已生成"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/shipments/with-label [post]
func (c *ShipmentController) CreateWithLabel(ctx *gin.Context) {
	var req dto.CreateLabelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shipment, err := c.svc.CreateShipmentWithLabel(ctx, req.OrderID, req.CarrierCode, req.ServiceCode)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"data":    c.toResponse(shipment),
		"message": "发货成功，面单已生成",
	})
}

// GetByID 获取发货详情
// @Summary 获取发货详情
// @Description 根据发货记录ID获取详细信息
// @Tags Shipment (发货管理)
// @Produce json
// @Param id path int true "发货记录ID"
// @Success 200 {object} map[string]interface{} "{"data": dto.EtsyShipmentResponse}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 404 {object} map[string]string "发货记录不存在"
// @Router /api/shipments/{id} [get]
func (c *ShipmentController) GetByID(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	shipment, err := c.svc.GetByID(ctx, id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "发货记录不存在"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": c.toResponse(shipment)})
}

// GetByOrderID 根据订单获取发货
// @Summary 根据订单ID获取发货信息
// @Description 获取指定订单的发货记录
// @Tags Shipment (发货管理)
// @Produce json
// @Param order_id path int true "订单ID"
// @Success 200 {object} map[string]interface{} "{"data": dto.EtsyShipmentResponse}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 404 {object} map[string]string "发货记录不存在"
// @Router /api/orders/{order_id}/shipment [get]
func (c *ShipmentController) GetByOrderID(ctx *gin.Context) {
	orderID, err := strconv.ParseInt(ctx.Param("order_id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的订单ID"})
		return
	}

	shipment, err := c.svc.GetByOrderID(ctx, orderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "发货记录不存在"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": c.toResponse(shipment)})
}

// List 发货列表
// @Summary 获取发货列表
// @Description 分页查询发货记录，支持多条件筛选
// @Tags Shipment (发货管理)
// @Produce json
// @Param order_id query int false "订单ID"
// @Param carrier_code query string false "物流商代码"
// @Param status query string false "发货状态"
// @Param tracking_number query string false "运单号"
// @Param etsy_synced query bool false "是否已同步Etsy"
// @Param start_date query string false "开始日期 (YYYY-MM-DD)"
// @Param end_date query string false "结束日期 (YYYY-MM-DD)"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} map[string]interface{} "{"data": dto.EtsyShipmentListResponse}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/shipments [get]
func (c *ShipmentController) List(ctx *gin.Context) {
	var filter repository.ShipmentFilter

	if orderID, _ := strconv.ParseInt(ctx.Query("order_id"), 10, 64); orderID > 0 {
		filter.OrderID = orderID
	}
	filter.CarrierCode = ctx.Query("carrier_code")
	filter.Status = ctx.Query("status")
	filter.TrackingNumber = ctx.Query("tracking_number")

	if synced := ctx.Query("etsy_synced"); synced != "" {
		b := synced == "true"
		filter.EtsySynced = &b
	}

	if startDate := ctx.Query("start_date"); startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			filter.StartDate = &t
		}
	}
	if endDate := ctx.Query("end_date"); endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			filter.EndDate = &t
		}
	}

	filter.Page, _ = strconv.Atoi(ctx.DefaultQuery("page", "1"))
	filter.PageSize, _ = strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	shipments, total, err := c.svc.List(ctx, filter)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list := make([]dto.EtsyShipmentResponse, len(shipments))
	for i, s := range shipments {
		list[i] = c.toResponse(&s)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data": dto.EtsyShipmentListResponse{
			List:     list,
			Total:    total,
			Page:     filter.Page,
			PageSize: filter.PageSize,
		},
	})
}

// ==================== 物流跟踪 ====================

// RefreshTracking 刷新物流跟踪
// @Summary 刷新物流跟踪状态
// @Description 手动刷新指定发货记录的物流轨迹
// @Tags Shipment (发货管理)
// @Produce json
// @Param id path int true "发货记录ID"
// @Success 200 {object} map[string]interface{} "{"data": dto.EtsyShipmentResponse, "message": "物流信息已刷新"}"
// @Failure 400 {object} map[string]string "刷新失败"
// @Router /api/shipments/{id}/refresh-tracking [post]
func (c *ShipmentController) RefreshTracking(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := c.svc.RefreshTracking(ctx, id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	shipment, _ := c.svc.GetByID(ctx, id)
	ctx.JSON(http.StatusOK, gin.H{
		"data":    c.toResponse(shipment),
		"message": "物流信息已刷新",
	})
}

// ==================== Etsy 同步 ====================

// SyncToEtsy 同步到 Etsy
// @Summary 同步发货信息到 Etsy
// @Description 将发货信息同步到 Etsy 平台，更新订单的发货状态
// @Tags Shipment (发货管理)
// @Produce json
// @Param id path int true "发货记录ID"
// @Success 200 {object} map[string]string "{"message": "已同步到 Etsy"}"
// @Failure 400 {object} map[string]string "同步失败"
// @Router /api/shipments/{id}/sync-etsy [post]
func (c *ShipmentController) SyncToEtsy(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的ID"})
		return
	}

	if err := c.svc.SyncToEtsy(ctx, id); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "已同步到 Etsy"})
}

// ==================== Webhook ====================

// HandleWebhook 处理 Karrio Webhook
// @Summary 处理 Karrio 物流回调
// @Description 接收 Karrio 推送的物流状态更新事件
// @Tags Shipment (发货管理)
// @Accept json
// @Produce json
// @Param request body object true "Webhook 载荷"
// @Success 200 {object} map[string]string "{"message": "processed"}"
// @Failure 400 {object} map[string]string "处理失败"
// @Router /api/webhooks/karrio/tracking [post]
func (c *ShipmentController) HandleWebhook(ctx *gin.Context) {
	var payload struct {
		Event     string `json:"event"`
		TrackerID string `json:"id"`
		Data      struct {
			TrackingNumber string              `json:"tracking_number"`
			CarrierName    string              `json:"carrier_name"`
			Status         string              `json:"status"`
			Delivered      string              `json:"delivered"`
			Events         []dto.TrackingEvent `json:"events"`
		} `json:"data"`
	}

	if err := ctx.ShouldBindJSON(&payload); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if payload.Event != "tracking.updated" {
		ctx.JSON(http.StatusOK, gin.H{"message": "ignored"})
		return
	}

	if err := c.svc.HandleWebhook(ctx, payload.TrackerID, payload.Data.Delivered, payload.Data.Events); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "processed"})
}

// ==================== 物流商 ====================

// GetCarriers 获取物流商列表
// @Summary 获取支持的物流商列表
// @Description 获取系统支持的所有物流商及其代码
// @Tags Shipment (发货管理)
// @Produce json
// @Success 200 {object} map[string]interface{} "{"data": []}"
// @Router /api/shipments/carriers [get]
func (c *ShipmentController) GetCarriers(ctx *gin.Context) {
	carriers := c.svc.GetSupportedCarriers()
	ctx.JSON(http.StatusOK, gin.H{"data": carriers})
}

// ==================== 响应转换 ====================

func (c *ShipmentController) toResponse(s *model.Shipment) dto.EtsyShipmentResponse {
	resp := dto.EtsyShipmentResponse{
		ID:                   s.ID,
		OrderID:              s.OrderID,
		CarrierCode:          s.CarrierCode,
		CarrierName:          s.CarrierName,
		TrackingNumber:       s.TrackingNumber,
		ServiceCode:          s.ServiceCode,
		DestCarrierCode:      s.DestCarrierCode,
		DestCarrierName:      s.DestCarrierName,
		DestTrackingNumber:   s.DestTrackingNumber,
		LabelURL:             s.LabelURL,
		Weight:               s.Weight,
		WeightUnit:           s.WeightUnit,
		Status:               s.Status,
		StatusText:           c.getStatusText(s.Status),
		EtsySynced:           s.EtsySynced,
		EtsySyncError:        s.EtsySyncError,
		LastTrackingStatus:   s.LastTrackingStatus,
		LastTrackingLocation: s.LastTrackingLocation,
		CreatedAt:            s.CreatedAt.Format(time.RFC3339),
		UpdatedAt:            s.UpdatedAt.Format(time.RFC3339),
	}

	if s.EtsySyncedAt != nil {
		t := s.EtsySyncedAt.Format(time.RFC3339)
		resp.EtsySyncedAt = &t
	}
	if s.LastTrackingTime != nil {
		t := s.LastTrackingTime.Format(time.RFC3339)
		resp.LastTrackingTime = &t
	}
	if s.ShippedAt != nil {
		t := s.ShippedAt.Format(time.RFC3339)
		resp.ShippedAt = &t
	}
	if s.DeliveredAt != nil {
		t := s.DeliveredAt.Format(time.RFC3339)
		resp.DeliveredAt = &t
	}

	if len(s.TrackingEvents) > 0 {
		resp.TrackingEvents = make([]dto.TrackingEventResponse, len(s.TrackingEvents))
		for i, e := range s.TrackingEvents {
			resp.TrackingEvents[i] = dto.TrackingEventResponse{
				ID:          e.ID,
				OccurredAt:  e.OccurredAt.Format(time.RFC3339),
				Status:      e.Status,
				StatusCode:  e.StatusCode,
				Description: e.Description,
				Location:    e.Location,
			}
		}
	}

	return resp
}

func (c *ShipmentController) getStatusText(status string) string {
	texts := map[string]string{
		model.ShipmentStatusCreated:    "已创建",
		model.ShipmentStatusInTransit:  "运输中",
		model.ShipmentStatusArrived:    "已到达",
		model.ShipmentStatusDelivering: "派送中",
		model.ShipmentStatusDelivered:  "已签收",
		model.ShipmentStatusException:  "异常",
		model.ShipmentStatusReturned:   "已退回",
	}
	if text, ok := texts[status]; ok {
		return text
	}
	return status
}
