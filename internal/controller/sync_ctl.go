package controller

import (
	"etsy_dev_v1_202512/internal/task"
	"strconv"

	"github.com/gin-gonic/gin"
)

// SyncController 同步控制器
type SyncController struct {
	taskManager *task.TaskManager
}

// NewSyncController 创建同步控制器
func NewSyncController(taskManager *task.TaskManager) *SyncController {
	return &SyncController{taskManager: taskManager}
}

// ==================== Handler 实现 ====================

// SyncShop 同步单个店铺
// @Summary 手动同步单个店铺
// @Tags Sync
// @Param id path int true "店铺 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/shops/{id} [post]
func (c *SyncController) SyncShop(ctx *gin.Context) {
	shopID := parseID(ctx, "id")
	if shopID == 0 {
		return
	}

	if err := c.taskManager.TriggerShopSync(ctx.Request.Context(), shopID); err != nil {
		ctx.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "店铺同步已触发",
		"data":    gin.H{"shop_id": shopID},
	})
}

// SyncAllShops 同步所有店铺
// @Summary 手动同步所有店铺
// @Tags Sync
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/shops [post]
func (c *SyncController) SyncAllShops(ctx *gin.Context) {
	c.taskManager.TriggerAllShopsSync()

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "所有店铺同步任务已启动",
	})
}

// SyncProducts 同步单个店铺商品
// @Summary 手动同步单个店铺商品
// @Tags Sync
// @Param shop_id path int true "店铺 ID"
// @Param full query bool false "是否全量同步"
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/products/{shop_id} [post]
func (c *SyncController) SyncProducts(ctx *gin.Context) {
	shopID := parseID(ctx, "shop_id")
	if shopID == 0 {
		return
	}

	fullSync := ctx.Query("full") == "true"

	if err := c.taskManager.TriggerProductSync(ctx.Request.Context(), shopID, fullSync); err != nil {
		ctx.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}

	syncType := "增量"
	if fullSync {
		syncType = "全量"
	}

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "商品" + syncType + "同步已触发",
		"data":    gin.H{"shop_id": shopID, "full_sync": fullSync},
	})
}

// SyncAllProducts 同步所有商品
// @Summary 手动同步所有店铺商品
// @Tags Sync
// @Param full query bool false "是否全量同步"
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/products [post]
func (c *SyncController) SyncAllProducts(ctx *gin.Context) {
	fullSync := ctx.Query("full") == "true"
	c.taskManager.TriggerAllProductsSync(fullSync)

	syncType := "增量"
	if fullSync {
		syncType = "全量"
	}

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "所有商品" + syncType + "同步任务已启动",
	})
}

// SyncOrders 同步单个店铺订单
// @Summary 手动同步单个店铺订单
// @Tags Sync
// @Param shop_id path int true "店铺 ID"
// @Param force query bool false "是否强制同步"
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/orders/{shop_id} [post]
func (c *SyncController) SyncOrders(ctx *gin.Context) {
	shopID := parseID(ctx, "shop_id")
	if shopID == 0 {
		return
	}

	forceSync := ctx.Query("force") == "true"

	resp, err := c.taskManager.TriggerOrderSync(ctx.Request.Context(), shopID, forceSync)
	if err != nil {
		ctx.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "订单同步完成",
		"data": gin.H{
			"shop_id":    shopID,
			"new_orders": resp.NewOrders,
			"updated":    resp.UpdatedOrders,
			"force_sync": forceSync,
		},
	})
}

// SyncAllOrders 同步所有订单
// @Summary 手动同步所有店铺订单
// @Tags Sync
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/orders [post]
func (c *SyncController) SyncAllOrders(ctx *gin.Context) {
	c.taskManager.TriggerAllOrdersSync()

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "所有订单同步任务已启动",
	})
}

// RefreshTracking 刷新物流跟踪
// @Summary 手动刷新物流跟踪信息
// @Tags Sync
// @Success 200 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{} "限流中"
// @Router /api/v1/sync/tracking/refresh [post]
func (c *SyncController) RefreshTracking(ctx *gin.Context) {
	c.taskManager.TriggerTrackingRefresh()

	ctx.JSON(200, gin.H{
		"code":    200,
		"message": "物流跟踪刷新任务已启动",
	})
}

// ==================== 工具函数 ====================

func parseID(ctx *gin.Context, key string) int64 {
	idStr := ctx.Param(key)
	var id int64
	if _, err := parseUint(idStr, &id); err != nil {
		ctx.JSON(400, gin.H{"code": 400, "message": "无效的 ID"})
		return 0
	}
	return id
}

func parseUint(s string, v *int64) (bool, error) {
	if s == "" {
		return false, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return false, err
	}
	*v = n
	return true, nil
}
