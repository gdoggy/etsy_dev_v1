package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ShopController struct {
	shopSvc *service.ShopService
}

func NewShopController(shopSvc *service.ShopService) *ShopController {
	return &ShopController{
		shopSvc: shopSvc,
	}
}

// GetShopList 获取店铺列表
// GET /api/v1/shops
func (c *ShopController) GetShopList(ctx *gin.Context) {
	var req dto.ShopListReq
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	resp, err := c.shopSvc.GetShopList(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetShopDetail 获取店铺详情
// GET /api/v1/shops/:id
func (c *ShopController) GetShopDetail(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	resp, err := c.shopSvc.GetShopDetail(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// UpdateShopToEtsy 更新店铺信息到 Etsy
// PUT /api/v1/shops/:id
func (c *ShopController) UpdateShopToEtsy(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	var req dto.ShopUpdateToEtsyReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := c.shopSvc.UpdateShopToEtsy(ctx.Request.Context(), id, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// StopShop 停用店铺
// POST /api/v1/shops/:id/stop
func (c *ShopController) StopShop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.shopSvc.StopShop(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "店铺已停用"})
}

// ResumeShop 恢复店铺（触发重新授权）
// POST /api/v1/shops/:id/resume
func (c *ShopController) ResumeShop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.shopSvc.ResumeShop(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "店铺已设为待授权状态，请重新授权"})
}

// DeleteShop 删除店铺
// DELETE /api/v1/shops/:id
func (c *ShopController) DeleteShop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.shopSvc.DeleteShop(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SyncShop 手动同步店铺数据
// POST /api/v1/shops/:id/sync
func (c *ShopController) SyncShop(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	resp, err := c.shopSvc.ManualSyncShop(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// ==================== Shop Section ====================

// SyncSections 同步店铺分区
// POST /api/v1/shops/:id/sections/sync
func (c *ShopController) SyncSections(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.shopSvc.SyncSectionsFromEtsy(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "分区同步成功"})
}

// CreateSection 创建店铺分区
// POST /api/v1/shops/:id/sections
func (c *ShopController) CreateSection(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	var req dto.ShopSectionCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "分区标题不能为空"})
		return
	}

	resp, err := c.shopSvc.CreateSectionToEtsy(ctx.Request.Context(), shopID, req.Title)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, resp)
}

// UpdateSection 更新店铺分区
// PUT /api/v1/shops/sections/:sectionId
func (c *ShopController) UpdateSection(ctx *gin.Context) {
	sectionID, err := strconv.ParseInt(ctx.Param("sectionId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的分区ID"})
		return
	}

	var req dto.ShopSectionUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if req.Title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "分区标题不能为空"})
		return
	}

	if err := c.shopSvc.UpdateSectionToEtsy(ctx.Request.Context(), sectionID, req.Title); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteSection 删除店铺分区
// DELETE /api/v1/shops/sections/:sectionId
func (c *ShopController) DeleteSection(ctx *gin.Context) {
	sectionID, err := strconv.ParseInt(ctx.Param("sectionId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的分区ID"})
		return
	}

	if err := c.shopSvc.DeleteSectionFromEtsy(ctx.Request.Context(), sectionID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
