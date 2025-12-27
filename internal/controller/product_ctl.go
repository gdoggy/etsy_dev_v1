package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/internal/service"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProductController struct {
	productService *service.ProductService
}

func NewProductController(productService *service.ProductService) *ProductController {
	return &ProductController{productService: productService}
}

// ==================== 查询接口 ====================

// GetProducts 获取商品列表
// @Summary 获取指定店铺的商品列表
// @Tags Product
// @Param shop_id query int true "店铺ID"
// @Param state query string false "状态筛选"
// @Param keyword query string false "标题搜索"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} dto.ProductListResp
// @Router /api/products [get]
func (ctrl *ProductController) GetProducts(c *gin.Context) {
	shopIDStr := c.Query("shop_id")
	shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil || shopID <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的 shop_id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	var products []model.Product
	var total int64

	ctx := c.Request.Context()
	if keyword != "" {
		products, total, err = ctrl.productService.SearchProducts(ctx, shopID, keyword, page, pageSize)
	} else {
		products, total, err = ctrl.productService.GetShopProducts(shopID, page, pageSize)
	}

	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
		return
	}

	respList := make([]dto.ProductResp, 0, len(products))
	for _, p := range products {
		respList = append(respList, ctrl.productService.ToProductResp(&p))
	}

	c.JSON(200, dto.ProductListResp{
		Code:     0,
		Message:  "success",
		Data:     respList,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// GetProduct 获取商品详情
// @Summary 获取单个商品详情
// @Tags Product
// @Param id path int true "商品ID"
// @Success 200 {object} dto.ProductResp
// @Router /api/products/{id} [get]
func (ctrl *ProductController) GetProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	ctx := c.Request.Context()
	product, err := ctrl.productService.GetProductByID(ctx, id)
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "商品不存在"})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    ctrl.productService.ToProductResp(product),
	})
}

// GetProductStats 获取商品统计
// @Summary 获取店铺商品统计信息
// @Tags Product
// @Param shop_id query int true "店铺ID"
// @Success 200 {object} dto.ProductStatsResp
// @Router /api/products/stats [get]
func (ctrl *ProductController) GetProductStats(c *gin.Context) {
	shopIDStr := c.Query("shop_id")
	shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil || shopID <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的 shop_id"})
		return
	}

	ctx := c.Request.Context()
	stats, err := ctrl.productService.GetProductStats(ctx, shopID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ==================== AI 草稿接口 ====================

// GenerateAIDraft AI 生成商品草稿
// @Summary 调用 AI 生成商品草稿
// @Tags Product
// @Accept json
// @Produce json
// @Param body body dto.AIGenerateReq true "生成参数"
// @Success 200 {object} dto.ProductResp
// @Router /api/products/ai/generate [post]
func (ctrl *ProductController) GenerateAIDraft(c *gin.Context) {
	var req dto.AIGenerateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	ctx := c.Request.Context()
	product, err := ctrl.productService.GenerateAIDraft(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "生成失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    ctrl.productService.ToProductResp(product),
	})
}

// ApproveAIDraft 审核通过 AI 草稿
// @Summary 审核通过 AI 生成的草稿
// @Tags Product
// @Param id path int true "商品ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/{id}/approve [post]
func (ctrl *ProductController) ApproveAIDraft(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.productService.ApproveAIDraft(ctx, id); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "审核失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "审核通过"})
}

// ==================== CRUD 接口 ====================

// CreateProduct 创建商品 (直接推送 Etsy)
// @Summary 创建商品并推送到 Etsy
// @Tags Product
// @Accept json
// @Produce json
// @Param body body dto.CreateProductReq true "商品信息"
// @Success 201 {object} dto.ProductResp
// @Router /api/products [post]
func (ctrl *ProductController) CreateProduct(c *gin.Context) {
	var req dto.CreateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	ctx := c.Request.Context()
	product, err := ctrl.productService.CreateDraftListing(ctx, &req)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"code":    0,
		"message": "success",
		"data":    ctrl.productService.ToProductResp(product),
	})
}

// UpdateProduct 更新商品
// @Summary 更新商品信息并同步到 Etsy
// @Tags Product
// @Accept json
// @Produce json
// @Param id path int true "商品ID"
// @Param body body dto.UpdateProductReq true "更新内容"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/{id} [patch]
func (ctrl *ProductController) UpdateProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	var req dto.UpdateProductReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	req.ID = id

	ctx := c.Request.Context()
	if err := ctrl.productService.UpdateListing(ctx, &req); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteProduct 删除商品
// @Summary 删除商品 (同时删除 Etsy 远程)
// @Tags Product
// @Param id path int true "商品ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/{id} [delete]
func (ctrl *ProductController) DeleteProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.productService.DeleteListing(ctx, id); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "删除成功"})
}

// ==================== 状态变更接口 ====================

// ActivateProduct 上架商品
// @Summary 将商品状态改为 active
// @Tags Product
// @Param id path int true "商品ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/{id}/activate [post]
func (ctrl *ProductController) ActivateProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.productService.ActivateListing(ctx, id); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "上架失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "上架成功"})
}

// DeactivateProduct 下架商品
// @Summary 将商品状态改为 inactive
// @Tags Product
// @Param id path int true "商品ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/{id}/deactivate [post]
func (ctrl *ProductController) DeactivateProduct(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.productService.DeactivateListing(ctx, id); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "下架失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "下架成功"})
}

// ==================== 同步接口 ====================

// SyncProducts 同步店铺商品
// @Summary 从 Etsy 全量同步商品
// @Tags Product
// @Param shop_id query int true "店铺ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/products/sync [post]
func (ctrl *ProductController) SyncProducts(c *gin.Context) {
	shopIDStr := c.Query("shop_id")
	shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil || shopID <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的 shop_id"})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.productService.SyncListingsFromEtsy(ctx, shopID); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "同步失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"code": 0, "message": "同步成功"})
}

// ==================== 图片接口 ====================

// UploadImage 上传商品图片
// @Summary 上传图片到 Etsy
// @Tags Product
// @Accept multipart/form-data
// @Param id path int true "商品ID"
// @Param image formData file true "图片文件"
// @Param rank formData int false "排序" default(1)
// @Success 201 {object} dto.ProductImageResp
// @Router /api/products/{id}/images [post]
func (ctrl *ProductController) UploadImage(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效的商品ID"})
		return
	}

	// 获取文件
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "请上传图片文件"})
		return
	}
	defer file.Close()

	// 读取文件内容
	imageData := make([]byte, header.Size)
	if _, err := file.Read(imageData); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "读取文件失败"})
		return
	}

	// 获取 rank
	rank, _ := strconv.Atoi(c.DefaultPostForm("rank", "1"))

	ctx := c.Request.Context()
	image, err := ctrl.productService.UploadListingImage(ctx, id, imageData, header.Filename, rank)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "上传失败: " + err.Error()})
		return
	}

	c.JSON(201, gin.H{
		"code":    0,
		"message": "success",
		"data": dto.ProductImageResp{
			ID:          image.ID,
			EtsyImageID: image.EtsyImageID,
			Url:         image.EtsyUrl,
			Rank:        image.Rank,
			Width:       image.Width,
			Height:      image.Height,
		},
	})
}
