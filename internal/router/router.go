package router

import (
	"etsy_dev_v1_202512/internal/controller"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "etsy_dev_v1_202512/docs"
)

// ==================== 控制器集合 ====================

// Controllers 控制器集合
type Controllers struct {
	Proxy        *controller.ProxyController
	Developer    *controller.DeveloperController
	Auth         *controller.AuthController
	Shop         *controller.ShopController
	Shipping     *controller.ShippingProfileController
	ReturnPolicy *controller.ReturnPolicyController
	Product      *controller.ProductController
	Draft        *controller.DraftController
}

// ==================== 主路由设置 ====================

// SetupRouter 初始化路由
func SetupRouter(ctrl *Controllers) *gin.Engine {
	r := gin.Default()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())

	// Swagger 文档
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API 路由组
	api := r.Group("/api")
	{
		registerProxyRoutes(api, ctrl.Proxy)
		registerDeveloperRoutes(api, ctrl.Developer)
		registerAuthRoutes(api, ctrl.Auth)
		registerShopRoutes(api, ctrl.Shop, ctrl.Shipping, ctrl.ReturnPolicy)
		registerShippingRoutes(api, ctrl.Shipping, ctrl.ReturnPolicy)
		registerProductRoutes(api, ctrl.Product)
		registerDraftRoutes(api, ctrl.Draft)
	}

	return r
}

// ==================== 模块路由注册 ====================

// registerProxyRoutes 代理模块路由
func registerProxyRoutes(api *gin.RouterGroup, ctl *controller.ProxyController) {
	proxy := api.Group("/proxies")
	{
		proxy.GET("", ctl.GetList)
		proxy.GET("/:id", ctl.GetDetail)
		proxy.POST("", ctl.Create)
		proxy.PUT("", ctl.Update)
		proxy.GET("/callback", ctl.Callback)
	}
}

// registerDeveloperRoutes 开发者模块路由
func registerDeveloperRoutes(api *gin.RouterGroup, ctl *controller.DeveloperController) {
	developer := api.Group("/developers")
	{
		developer.GET("", ctl.GetList)
		developer.GET("/:id", ctl.GetDetail)
		developer.POST("", ctl.Create)
		developer.PUT("/:id", ctl.Update)
		developer.PATCH("/:id/status", ctl.UpdateStatus)
		developer.DELETE("/:id", ctl.Delete)
		developer.POST("/:id/ping", ctl.TestConnectivity)
	}
}

// registerAuthRoutes 鉴权模块路由
func registerAuthRoutes(api *gin.RouterGroup, ctl *controller.AuthController) {
	// 通用回调
	api.GET("/:path/auth/callback", ctl.Callback)

	auth := api.Group("/oauth")
	{
		auth.GET("/login", ctl.Login)
		auth.GET("/callback", ctl.Callback)
		auth.POST("/refresh", ctl.RefreshToken)
	}
}

// registerShopRoutes 店铺模块路由
func registerShopRoutes(
	api *gin.RouterGroup,
	shopCtl *controller.ShopController,
	shippingCtl *controller.ShippingProfileController,
	returnPolicyCtl *controller.ReturnPolicyController,
) {
	shops := api.Group("/shops")
	{
		// 店铺基础操作
		shops.GET("", shopCtl.GetShopList)
		shops.GET("/:id", shopCtl.GetShopDetail)
		shops.PUT("/:id", shopCtl.UpdateShopToEtsy)
		shops.DELETE("/:id", shopCtl.DeleteShop)
		shops.POST("/:id/stop", shopCtl.StopShop)
		shops.POST("/:id/resume", shopCtl.ResumeShop)
		shops.POST("/:id/sync", shopCtl.SyncShop)

		// Section 管理
		shops.POST("/:id/sections/sync", shopCtl.SyncSections)
		shops.POST("/:id/sections", shopCtl.CreateSection)
		shops.PUT("/sections/:sectionId", shopCtl.UpdateSection)
		shops.DELETE("/sections/:sectionId", shopCtl.DeleteSection)

		// 店铺下的运费模板
		shops.GET("/:id/shipping-profiles", shippingCtl.GetProfileList)
		shops.POST("/:id/shipping-profiles", shippingCtl.CreateProfile)
		shops.POST("/:id/shipping-profiles/sync", shippingCtl.SyncProfiles)

		// 店铺下的退货政策
		shops.GET("/:id/return-policies", returnPolicyCtl.GetPolicyList)
		shops.POST("/:id/return-policies", returnPolicyCtl.CreatePolicy)
		shops.POST("/:id/return-policies/sync", returnPolicyCtl.SyncPolicies)
	}
}

// registerShippingRoutes 运费模块路由
func registerShippingRoutes(
	api *gin.RouterGroup,
	shippingCtl *controller.ShippingProfileController,
	returnPolicyCtl *controller.ReturnPolicyController,
) {
	shipping := api.Group("/shipping")
	{
		// 运费模板
		shipping.GET("/:id", shippingCtl.GetProfileDetail)
		shipping.PUT("/:id", shippingCtl.UpdateProfile)
		shipping.DELETE("/:id", shippingCtl.DeleteProfile)

		// 目的地
		shipping.POST("/:profileID/destinations", shippingCtl.CreateDestination)

		// 升级选项
		shipping.POST("/:profileID/upgrades", shippingCtl.CreateUpgrade)
		shipping.PUT("/upgrades/:id", shippingCtl.UpdateUpgrade)
		shipping.DELETE("/upgrades/:id", shippingCtl.DeleteUpgrade)

		// 退货政策
		shipping.GET("/return-policies/:id", returnPolicyCtl.GetPolicyDetail)
		shipping.PUT("/return-policies/:id", returnPolicyCtl.UpdatePolicy)
		shipping.DELETE("/return-policies/:id", returnPolicyCtl.DeletePolicy)
	}
}

// registerProductRoutes 商品模块路由
func registerProductRoutes(api *gin.RouterGroup, ctl *controller.ProductController) {
	products := api.Group("/products")
	{
		// 查询
		products.GET("", ctl.GetProducts)
		products.GET("/stats", ctl.GetProductStats)
		products.GET("/:id", ctl.GetProduct)

		// CRUD
		products.POST("", ctl.CreateProduct)
		products.PATCH("/:id", ctl.UpdateProduct)
		products.DELETE("/:id", ctl.DeleteProduct)

		// 状态变更
		products.POST("/:id/activate", ctl.ActivateProduct)
		products.POST("/:id/deactivate", ctl.DeactivateProduct)

		// AI 草稿
		products.POST("/ai/generate", ctl.GenerateAIDraft)
		products.POST("/:id/approve", ctl.ApproveAIDraft)

		// 同步 & 图片
		products.POST("/sync", ctl.SyncProducts)
		products.POST("/:id/images", ctl.UploadImage)
	}
}

// registerDraftRoutes 草稿模块路由
func registerDraftRoutes(api *gin.RouterGroup, ctl *controller.DraftController) {
	drafts := api.Group("/drafts")
	{
		// 任务列表与创建
		drafts.GET("", ctl.ListDraftTasks)
		drafts.POST("", ctl.CreateDraft)

		// 支持的平台
		drafts.GET("/platforms", ctl.GetSupportedPlatforms)

		// 任务详情与操作
		drafts.GET("/:task_id", ctl.GetDraftDetail)
		drafts.GET("/:task_id/stream", ctl.StreamProgress)
		drafts.POST("/:task_id/confirm-all", ctl.ConfirmAllDrafts)
		drafts.POST("/:task_id/regenerate-images", ctl.RegenerateImages)

		// 草稿商品操作
		draftProducts := drafts.Group("/products")
		{
			draftProducts.PATCH("/:product_id", ctl.UpdateDraftProduct)
			draftProducts.POST("/:product_id/confirm", ctl.ConfirmDraftProduct)
		}
	}
}

// ==================== 中间件 ====================

// CORSMiddleware 跨域中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
