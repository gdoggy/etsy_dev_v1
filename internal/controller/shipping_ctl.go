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
// @Summary 获取运费模板列表
// @Description 获取指定店铺的所有运费模板
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param shopId path int true "店铺ID"
// @Success 200 {object} dto.ShippingProfileListResp "运费模板列表"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/v1/shops/{shopId}/shipping-profiles [get]
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
// @Summary 获取运费模板详情
// @Description 获取运费模板详细信息，包含目的地和加急选项
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param id path int true "运费模板ID"
// @Success 200 {object} dto.ShippingProfileDetailResp "运费模板详情"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/v1/shipping-profiles/{id} [get]
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
// @Summary 同步运费模板
// @Description 从 Etsy 同步店铺的运费模板数据
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param shopId path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "运费模板同步成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "同步失败"
// @Router /api/v1/shops/{shopId}/shipping-profiles/sync [post]
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
// @Summary 创建运费模板
// @Description 在 Etsy 创建新的运费模板
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param shopId path int true "店铺ID"
// @Param request body dto.ShippingProfileCreateReq true "模板参数"
// @Success 201 {object} dto.ShippingProfileResp "创建结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Router /api/v1/shops/{shopId}/shipping-profiles [post]
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
// @Summary 更新运费模板
// @Description 更新 Etsy 运费模板信息
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param id path int true "运费模板ID"
// @Param request body dto.ShippingProfileUpdateReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/shipping-profiles/{id} [put]
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
// @Summary 删除运费模板
// @Description 从 Etsy 删除运费模板
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param id path int true "运费模板ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/shipping-profiles/{id} [delete]
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
// @Summary 创建运费目的地
// @Description 为运费模板添加目的地国家/地区配置
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param profileId path int true "运费模板ID"
// @Param request body dto.ShippingDestinationCreateReq true "目的地参数"
// @Success 201 {object} dto.ShippingDestinationResp "创建结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Router /api/v1/shipping-profiles/{profileId}/destinations [post]
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
// @Summary 更新运费目的地
// @Description 更新运费目的地配置（运费、时效等）
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param id path int true "目的地ID"
// @Param request body dto.ShippingDestinationUpdateReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/shipping-destinations/{id} [put]
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
// @Summary 删除运费目的地
// @Description 从运费模板中移除目的地配置
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param id path int true "目的地ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/shipping-destinations/{id} [delete]
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
// @Summary 创建加急配送选项
// @Description 为运费模板添加加急/快递配送选项
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param profileId path int true "运费模板ID"
// @Param request body dto.ShippingUpgradeCreateReq true "加急选项参数"
// @Success 201 {object} dto.ShippingUpgradeResp "创建结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Router /api/v1/shipping-profiles/{profileId}/upgrades [post]
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
// @Summary 更新加急配送选项
// @Description 更新加急配送选项的价格和时效
// @Tags ShippingProfile (运费模板)
// @Accept json
// @Produce json
// @Param id path int true "加急选项ID"
// @Param request body dto.ShippingUpgradeUpdateReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/shipping-upgrades/{id} [put]
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
// @Summary 删除加急配送选项
// @Description 从运费模板中移除加急配送选项
// @Tags ShippingProfile (运费模板)
// @Produce json
// @Param id path int true "加急选项ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/shipping-upgrades/{id} [delete]
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

// ==================== Return Policy ====================

type ReturnPolicyController struct {
	policySvc *service.ReturnPolicyService
}

func NewReturnPolicyController(policySvc *service.ReturnPolicyService) *ReturnPolicyController {
	return &ReturnPolicyController{
		policySvc: policySvc,
	}
}

// GetPolicyList 获取退货政策列表
// @Summary 获取退货政策列表
// @Description 获取指定店铺的所有退货政策
// @Tags ReturnPolicy (退货政策)
// @Produce json
// @Param shopId path int true "店铺ID"
// @Success 200 {object} dto.ReturnPolicyListResp "退货政策列表"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/v1/shops/{shopId}/return-policies [get]
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
// @Summary 获取退货政策详情
// @Description 获取退货政策的详细配置信息
// @Tags ReturnPolicy (退货政策)
// @Produce json
// @Param id path int true "退货政策ID"
// @Success 200 {object} dto.ReturnPolicyResp "退货政策详情"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/v1/return-policies/{id} [get]
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
// @Summary 同步退货政策
// @Description 从 Etsy 同步店铺的退货政策数据
// @Tags ReturnPolicy (退货政策)
// @Produce json
// @Param shopId path int true "店铺ID"
// @Success 200 {object} map[string]string "{"message": "退货政策同步成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "同步失败"
// @Router /api/v1/shops/{shopId}/return-policies/sync [post]
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
// @Summary 创建退货政策
// @Description 在 Etsy 创建新的退货政策
// @Tags ReturnPolicy (退货政策)
// @Accept json
// @Produce json
// @Param shopId path int true "店铺ID"
// @Param request body dto.ReturnPolicyCreateReq true "政策参数"
// @Success 201 {object} dto.ReturnPolicyResp "创建结果"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "创建失败"
// @Router /api/v1/shops/{shopId}/return-policies [post]
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
// @Summary 更新退货政策
// @Description 更新 Etsy 退货政策配置
// @Tags ReturnPolicy (退货政策)
// @Accept json
// @Produce json
// @Param id path int true "退货政策ID"
// @Param request body dto.ReturnPolicyUpdateReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "更新成功"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/v1/return-policies/{id} [put]
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
// @Summary 删除退货政策
// @Description 从 Etsy 删除退货政策
// @Tags ReturnPolicy (退货政策)
// @Produce json
// @Param id path int true "退货政策ID"
// @Success 200 {object} map[string]string "{"message": "删除成功"}"
// @Failure 400 {object} map[string]string "ID格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/v1/return-policies/{id} [delete]
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
