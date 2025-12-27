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
// @Summary 检查 Karrio 服务状态
// @Description 检查 Karrio 物流网关服务是否可用
// @Tags Karrio (物流网关)
// @Produce json
// @Success 200 {object} map[string]string "{"status": "healthy"}"
// @Failure 503 {object} map[string]string "服务不可用"
// @Router /api/karrio/ping [get]
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
// @Summary 获取物流商连接列表
// @Description 获取已配置的所有物流商连接
// @Tags Karrio (物流网关)
// @Produce json
// @Success 200 {object} map[string]interface{} "{"data": [], "total": 0}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/karrio/connections [get]
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
// @Summary 创建物流商连接
// @Description 添加新的物流商账号连接（如 FedEx、UPS 等）
// @Tags Karrio (物流网关)
// @Accept json
// @Produce json
// @Param request body dto.CreateConnectionRequest true "连接参数"
// @Success 201 {object} map[string]interface{} "{"data": {}, "message": "物流商连接创建成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/karrio/connections [post]
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
// @Summary 删除物流商连接
// @Description 移除指定的物流商账号连接
// @Tags Karrio (物流网关)
// @Produce json
// @Param id path string true "连接ID"
// @Success 200 {object} map[string]string "{"message": "物流商连接已删除"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/karrio/connections/{id} [delete]
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
// @Summary 获取运费报价
// @Description 根据包裹信息获取多个物流商的运费报价
// @Tags Karrio (物流网关)
// @Accept json
// @Produce json
// @Param request body dto.RateRequest true "报价请求参数"
// @Success 200 {object} map[string]interface{} "{"data": []}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/karrio/rates [post]
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
// @Summary 获取跟踪器列表
// @Description 获取所有物流跟踪器，支持按状态筛选
// @Tags Karrio (物流网关)
// @Produce json
// @Param status query string false "状态筛选"
// @Param limit query int false "每页数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{} "{"data": [], "total": 0}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/karrio/trackers [get]
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
// @Summary 创建物流跟踪器
// @Description 为指定运单号创建物流跟踪
// @Tags Karrio (物流网关)
// @Accept json
// @Produce json
// @Param request body dto.CreateTrackerRequest true "跟踪参数"
// @Success 201 {object} map[string]interface{} "{"data": {}, "message": "跟踪器创建成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/karrio/trackers [post]
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
// @Summary 获取跟踪器详情
// @Description 获取指定跟踪器的详细物流信息
// @Tags Karrio (物流网关)
// @Produce json
// @Param id path string true "跟踪器ID"
// @Success 200 {object} map[string]interface{} "{"data": {}}"
// @Failure 404 {object} map[string]string "跟踪器不存在"
// @Router /api/karrio/trackers/{id} [get]
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
// @Summary 刷新跟踪状态
// @Description 手动刷新指定跟踪器的物流状态
// @Tags Karrio (物流网关)
// @Produce json
// @Param id path string true "跟踪器ID"
// @Success 200 {object} map[string]interface{} "{"data": {}, "message": "跟踪状态已刷新"}"
// @Failure 400 {object} map[string]string "刷新失败"
// @Router /api/karrio/trackers/{id}/refresh [post]
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
// @Summary 批量创建跟踪器
// @Description 批量为多个运单号创建物流跟踪
// @Tags Karrio (物流网关)
// @Accept json
// @Produce json
// @Param request body dto.BatchCreateTrackersRequest true "批量跟踪参数"
// @Success 201 {object} map[string]interface{} "{"data": [], "count": 0, "message": "批量创建成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Router /api/karrio/trackers/batch [post]
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
// @Summary 获取 Karrio 运单详情
// @Description 获取 Karrio 系统中的运单详细信息
// @Tags Karrio (物流网关)
// @Produce json
// @Param id path string true "运单ID"
// @Success 200 {object} map[string]interface{} "{"data": {}}"
// @Failure 404 {object} map[string]string "运单不存在"
// @Router /api/karrio/shipments/{id} [get]
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
// @Summary 取消 Karrio 运单
// @Description 取消 Karrio 系统中的运单
// @Tags Karrio (物流网关)
// @Produce json
// @Param id path string true "运单ID"
// @Success 200 {object} map[string]string "{"message": "运单已取消"}"
// @Failure 400 {object} map[string]string "取消失败"
// @Router /api/karrio/shipments/{id}/cancel [post]
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
