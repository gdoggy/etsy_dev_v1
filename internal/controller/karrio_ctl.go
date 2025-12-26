package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// KarrioController Karrio 物流网关控制器
type KarrioController struct {
	karrio *service.KarrioClient
}

// NewKarrioController 创建控制器
func NewKarrioController(karrio *service.KarrioClient) *KarrioController {
	return &KarrioController{karrio: karrio}
}

// ==================== 健康检查 ====================

// Ping 检查 Karrio 服务状态
// GET /api/karrio/ping
func (c *KarrioController) Ping(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	if err := c.karrio.Ping(ctx); err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// ==================== 物流商连接管理 ====================

// ListConnections 列出物流商连接
// GET /api/karrio/connections
func (c *KarrioController) ListConnections(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	resp, err := c.karrio.ListConnections(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  resp.Results,
		"total": resp.Count,
	})
}

// CreateConnection 创建物流商连接
// POST /api/karrio/connections
func (c *KarrioController) CreateConnection(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	var req dto.CreateConnectionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conn, err := c.karrio.CreateConnection(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"data":    conn,
		"message": "物流商连接创建成功",
	})
}

// DeleteConnection 删除物流商连接
// DELETE /api/karrio/connections/:id
func (c *KarrioController) DeleteConnection(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	connID := ctx.Param("id")
	if connID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的连接ID"})
		return
	}

	if err := c.karrio.DeleteConnection(ctx, connID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "物流商连接已删除"})
}

// ==================== 运费报价 ====================

// GetRates 获取运费报价
// POST /api/karrio/rates
func (c *KarrioController) GetRates(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	var req dto.RateRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.karrio.GetRates(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": resp.Rates})
}

// ==================== 跟踪器管理 ====================

// ListTrackers 列出跟踪器
// GET /api/karrio/trackers
func (c *KarrioController) ListTrackers(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	status := ctx.Query("status")
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(ctx.DefaultQuery("offset", "0"))

	resp, err := c.karrio.ListTrackers(ctx, status, limit, offset)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":  resp.Results,
		"total": resp.Count,
	})
}

// CreateTracker 创建跟踪器
// POST /api/karrio/trackers
func (c *KarrioController) CreateTracker(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	var req dto.CreateTrackerRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tracker, err := c.karrio.CreateTracker(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"data":    tracker,
		"message": "跟踪器创建成功",
	})
}

// GetTracker 获取跟踪详情
// GET /api/karrio/trackers/:id
func (c *KarrioController) GetTracker(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	trackerID := ctx.Param("id")
	if trackerID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的跟踪器ID"})
		return
	}

	tracker, err := c.karrio.GetTracker(ctx, trackerID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": tracker})
}

// RefreshTracker 刷新跟踪状态
// POST /api/karrio/trackers/:id/refresh
func (c *KarrioController) RefreshTracker(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	trackerID := ctx.Param("id")
	if trackerID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的跟踪器ID"})
		return
	}

	tracker, err := c.karrio.RefreshTracker(ctx, trackerID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":    tracker,
		"message": "跟踪状态已刷新",
	})
}

// BatchCreateTrackers 批量创建跟踪器
// POST /api/karrio/trackers/batch
func (c *KarrioController) BatchCreateTrackers(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	var req dto.BatchCreateTrackersRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trackers, err := c.karrio.BatchCreateTrackers(ctx, &req)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"data":    trackers,
		"count":   len(trackers),
		"message": "批量创建成功",
	})
}

// ==================== 运单管理 ====================

// GetShipment 获取运单详情
// GET /api/karrio/shipments/:id
func (c *KarrioController) GetShipment(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	shipmentID := ctx.Param("id")
	if shipmentID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运单ID"})
		return
	}

	shipment, err := c.karrio.GetShipment(ctx, shipmentID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"data": shipment})
}

// CancelShipment 取消运单
// POST /api/karrio/shipments/:id/cancel
func (c *KarrioController) CancelShipment(ctx *gin.Context) {
	if c.karrio == nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": "Karrio 客户端未配置"})
		return
	}

	shipmentID := ctx.Param("id")
	if shipmentID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运单ID"})
		return
	}

	if err := c.karrio.CancelShipment(ctx, shipmentID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "运单已取消"})
}
