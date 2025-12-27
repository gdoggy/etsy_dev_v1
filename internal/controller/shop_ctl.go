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
// @Summary 获取店铺列表
// @Description 分页查询店铺，支持按名称、状态筛选
// @Tags Shop (店铺管理)
// @Accept json
// @Produce json
// @Param keyword query string false "店铺名称关键词"
// @Param status query int false "状态筛选"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.ShopListResp "店铺列表"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/v1/shops [get]
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
// @Summary 获取店铺详情
// @Description 根据店铺ID获取详细信息，包含Token状态、关联的开发者账号等
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} dto.ShopDetailResp "店铺详情"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/v1/shops/{id} [get]
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
// @Summary 更新店铺信息
// @Description 更新店铺信息并同步到 Etsy 平台
// @Tags Shop (店铺管理)
// @Accept json
// @Produce json
// @Param id path int true "店铺ID"
// @Param request body dto.ShopUpdateToEtsyReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/shops/{id} [put]
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
// @Summary 停用店铺
// @Description 停用店铺，停止Token刷新和数据同步
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "店铺已停用"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "操作失败"
// @Router /api/v1/shops/{id}/stop [post]
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

// ResumeShop 恢复店铺
// @Summary 恢复店铺
// @Description 恢复店铺，设为待授权状态等待重新授权
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "店铺已设为待授权状态，请重新授权"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "操作失败"
// @Router /api/v1/shops/{id}/resume [post]
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
// @Summary 删除店铺
// @Description 软删除店铺记录
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/shops/{id} [delete]
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
// @Summary 手动同步店铺
// @Description 从 Etsy 拉取最新店铺数据并更新本地记录
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} dto.ShopSyncResp "同步结果"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "同步失败"
// @Router /api/v1/shops/{id}/sync [post]
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
// @Summary 同步店铺分区
// @Description 从 Etsy 同步店铺分区（Shop Section）数据
// @Tags Shop (店铺管理)
// @Produce json
// @Param id path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "分区同步成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "同步失败"
// @Router /api/v1/shops/{id}/sections/sync [post]
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
// @Summary 创建店铺分区
// @Description 在 Etsy 创建新的店铺分区
// @Tags Shop (店铺管理)
// @Accept json
// @Produce json
// @Param id path int true "店铺ID"
// @Param request body dto.ShopSectionCreateReq true "分区参数"
// @Success 201 {object} dto.ShopSectionResp "创建结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Router /api/v1/shops/{id}/sections [post]
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
// @Summary 更新店铺分区
// @Description 更新 Etsy 店铺分区标题
// @Tags Shop (店铺管理)
// @Accept json
// @Produce json
// @Param sectionId path int true "分区ID"
// @Param request body dto.ShopSectionUpdateReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/shops/sections/{sectionId} [put]
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
// @Summary 删除店铺分区
// @Description 从 Etsy 删除店铺分区
// @Tags Shop (店铺管理)
// @Produce json
// @Param sectionId path int true "分区ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/shops/sections/{sectionId} [delete]
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
