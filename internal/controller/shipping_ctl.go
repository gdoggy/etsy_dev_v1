package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ShippingProfileController struct {
	profileSvc *service.ShippingProfileService
}

func NewShippingProfileController(profileSvc *service.ShippingProfileService) *ShippingProfileController {
	return &ShippingProfileController{
		profileSvc: profileSvc,
	}
}

// ==================== Profile ====================

// GetProfileList 获取运费模板列表
// GET /api/v1/shops/:shopId/shipping-profiles
func (c *ShippingProfileController) GetProfileList(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	resp, err := c.profileSvc.GetProfileList(ctx.Request.Context(), shopID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetProfileDetail 获取运费模板详情
// GET /api/v1/shipping-profiles/:id
func (c *ShippingProfileController) GetProfileDetail(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费模板ID"})
		return
	}

	resp, err := c.profileSvc.GetProfileDetail(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// SyncProfiles 同步运费模板
// POST /api/v1/shops/:shopId/shipping-profiles/sync
func (c *ShippingProfileController) SyncProfiles(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.profileSvc.SyncProfilesFromEtsy(ctx.Request.Context(), shopID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "运费模板同步成功"})
}

// CreateProfile 创建运费模板
// POST /api/v1/shops/:shopId/shipping-profiles
func (c *ShippingProfileController) CreateProfile(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	var req dto.ShippingProfileCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	req.ShopID = shopID

	if req.Title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "模板标题不能为空"})
		return
	}
	if req.OriginCountryISO == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "发货国家不能为空"})
		return
	}

	resp, err := c.profileSvc.CreateProfileToEtsy(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, resp)
}

// UpdateProfile 更新运费模板
// PUT /api/v1/shipping-profiles/:id
func (c *ShippingProfileController) UpdateProfile(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费模板ID"})
		return
	}

	var req dto.ShippingProfileUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := c.profileSvc.UpdateProfileToEtsy(ctx.Request.Context(), id, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteProfile 删除运费模板
// DELETE /api/v1/shipping-profiles/:id
func (c *ShippingProfileController) DeleteProfile(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费模板ID"})
		return
	}

	if err := c.profileSvc.DeleteProfileFromEtsy(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== Destination ====================

// CreateDestination 创建运费目的地
// POST /api/v1/shipping-profiles/:profileId/destinations
func (c *ShippingProfileController) CreateDestination(ctx *gin.Context) {
	profileID, err := strconv.ParseInt(ctx.Param("profileId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费模板ID"})
		return
	}

	var req dto.ShippingDestinationCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	req.ShippingProfileID = profileID

	if req.DestinationCountryISO == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "目的地国家不能为空"})
		return
	}

	resp, err := c.profileSvc.CreateDestinationToEtsy(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, resp)
}

// UpdateDestination 更新运费目的地
// PUT /api/v1/shipping-destinations/:id
func (c *ShippingProfileController) UpdateDestination(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费目的地ID"})
		return
	}

	var req dto.ShippingDestinationUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := c.profileSvc.UpdateDestinationToEtsy(ctx.Request.Context(), id, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteDestination 删除运费目的地
// DELETE /api/v1/shipping-destinations/:id
func (c *ShippingProfileController) DeleteDestination(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费目的地ID"})
		return
	}

	if err := c.profileSvc.DeleteDestinationFromEtsy(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ==================== Upgrade ====================

// CreateUpgrade 创建加急配送选项
// POST /api/v1/shipping-profiles/:profileId/upgrades
func (c *ShippingProfileController) CreateUpgrade(ctx *gin.Context) {
	profileID, err := strconv.ParseInt(ctx.Param("profileId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的运费模板ID"})
		return
	}

	var req dto.ShippingUpgradeCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	req.ShippingProfileID = profileID

	if req.UpgradeName == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "升级选项名称不能为空"})
		return
	}
	if req.Type != 0 && req.Type != 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的类型，0=国内，1=国际"})
		return
	}

	resp, err := c.profileSvc.CreateUpgradeToEtsy(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, resp)
}

// UpdateUpgrade 更新加急配送选项
// PUT /api/v1/shipping-upgrades/:id
func (c *ShippingProfileController) UpdateUpgrade(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的加急配送选项ID"})
		return
	}

	var req dto.ShippingUpgradeUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	if err := c.profileSvc.UpdateUpgradeToEtsy(ctx.Request.Context(), id, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeleteUpgrade 删除加急配送选项
// DELETE /api/v1/shipping-upgrades/:id
func (c *ShippingProfileController) DeleteUpgrade(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的加急配送选项ID"})
		return
	}

	if err := c.profileSvc.DeleteUpgradeFromEtsy(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// =========  退货 ===========
type ReturnPolicyController struct {
	policySvc *service.ReturnPolicyService
}

func NewReturnPolicyController(policySvc *service.ReturnPolicyService) *ReturnPolicyController {
	return &ReturnPolicyController{
		policySvc: policySvc,
	}
}

// GetPolicyList 获取退货政策列表
// GET /api/v1/shops/:shopId/return-policies
func (c *ReturnPolicyController) GetPolicyList(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	resp, err := c.policySvc.GetPolicyList(ctx.Request.Context(), shopID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// GetPolicyDetail 获取退货政策详情
// GET /api/v1/return-policies/:id
func (c *ReturnPolicyController) GetPolicyDetail(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的退货政策ID"})
		return
	}

	resp, err := c.policySvc.GetPolicyDetail(ctx.Request.Context(), id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// SyncPolicies 同步退货政策
// POST /api/v1/shops/:shopId/return-policies/sync
func (c *ReturnPolicyController) SyncPolicies(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	if err := c.policySvc.SyncPoliciesFromEtsy(ctx.Request.Context(), shopID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "退货政策同步成功"})
}

// CreatePolicy 创建退货政策
// POST /api/v1/shops/:shopId/return-policies
func (c *ReturnPolicyController) CreatePolicy(ctx *gin.Context) {
	shopID, err := strconv.ParseInt(ctx.Param("shopId"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的店铺ID"})
		return
	}

	var req dto.ReturnPolicyCreateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	req.ShopID = shopID

	// 如果接受退货，则必须设置退货期限
	if req.AcceptsReturns && req.ReturnDeadline <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "接受退货时必须设置退货期限"})
		return
	}

	resp, err := c.policySvc.CreatePolicyToEtsy(ctx.Request.Context(), req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, resp)
}

// UpdatePolicy 更新退货政策
// PUT /api/v1/return-policies/:id
func (c *ReturnPolicyController) UpdatePolicy(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的退货政策ID"})
		return
	}

	var req dto.ReturnPolicyUpdateReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	// 如果接受退货，则必须设置退货期限
	if req.AcceptsReturns && req.ReturnDeadline <= 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "接受退货时必须设置退货期限"})
		return
	}

	if err := c.policySvc.UpdatePolicyToEtsy(ctx.Request.Context(), id, req); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// DeletePolicy 删除退货政策
// DELETE /api/v1/return-policies/:id
func (c *ReturnPolicyController) DeletePolicy(ctx *gin.Context) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "无效的退货政策ID"})
		return
	}

	if err := c.policySvc.DeletePolicyFromEtsy(ctx.Request.Context(), id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
