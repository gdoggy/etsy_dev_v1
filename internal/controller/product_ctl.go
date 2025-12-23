package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
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

// GetProducts 获取商品列表
// @Summary 获取指定店铺的商品列表
// @Tags Product
// @Accept json
// @Produce json
// @Param shop_id query int true "店铺 ID"
// @Param page query int false "页码 (默认1)" default(1)
// @Param page_size query int false "每页数量 (默认20)" default(20)
// @Success 200 {object} view.ProductListResp
// @Router /api/products [get]
func (ctrl *ProductController) GetProducts(c *gin.Context) {
	// 1. 参数解析
	shopIDStr := c.Query("shop_id")
	shopID, err := strconv.ParseInt(shopIDStr, 10, 64)
	if err != nil || shopID <= 0 {
		c.JSON(400, gin.H{"error": "无效的 shop_id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	// 2. 调用 Service 查库
	products, total, err := ctrl.productService.GetShopProducts(shopID, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": "查询失败: " + err.Error()})
		return
	}

	// 3. 数据转换 (Model -> VO)
	// 使用 make 初始化切片，防止结果为空时返回 null，而是返回 []
	respList := make([]dto.ProductResp, 0, len(products))
	for _, p := range products {
		respList = append(respList, dto.ToProductVO(&p))
	}

	// 4. 返回
	c.JSON(200, dto.ProductListResp{
		Code:     0,
		Message:  "success",
		Data:     respList,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}
