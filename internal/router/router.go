package router

import (
	"etsy_dev_v1_202512/internal/controller"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "etsy_dev_v1_202512/docs"
)

// InitRoutes 注册所有路由
func InitRoutes(r *gin.Engine,
	proxyCtl *controller.ProxyController,
	developerCtl *controller.DeveloperController,
	authCtrl *controller.AuthController,
	shopCtl *controller.ShopController,
	shippingCtl *controller.ShippingProfileController,
	returnPolicyCtl *controller.ReturnPolicyController,
	productCtrl *controller.ProductController) {
	// 1. Swagger 文档路由
	// 访问 http://localhost:8080/swagger/index.html 即可查看
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 2. API 路由组
	api := r.Group("/api")
	{
		// proxy 代理维护
		proxy := api.Group("/proxies")
		{
			// GET /api/proxies
			proxy.GET("", proxyCtl.GetList)
			proxy.GET("/:id", proxyCtl.GetDetail)
			proxy.POST("", proxyCtl.Create)
			proxy.PUT("", proxyCtl.Update)
			// GET /api/proxies/callback
			proxy.GET("/callback", proxyCtl.Callback)
		}
		// developer 开发者账号维护
		developer := api.Group("/developers")
		{
			developer.GET("", developerCtl.GetList)
			developer.GET("/:id", developerCtl.GetDetail)
			developer.POST("", developerCtl.Create)
			developer.PUT("/:id", developerCtl.Update)
			developer.PATCH("/:id/status", developerCtl.UpdateStatus)
			developer.DELETE("/:id", developerCtl.Delete)
			developer.POST("/:id/ping", developerCtl.TestConnectivity)
		}
		// auth 鉴权组
		// 正式处理回调
		api.GET("/:path/auth/callback", authCtrl.Callback)
		auth := api.Group("/oauth")
		{
			auth.GET("/login", authCtrl.Login)
			auth.GET("/callback", authCtrl.Callback)
			auth.POST("/refresh", authCtrl.RefreshToken)
		}
		// shop 店铺管理
		shops := r.Group("/shops")
		{
			shops.GET("", shopCtl.GetShopList)
			shops.GET("/:id", shopCtl.GetShopDetail)
			shops.PUT("/:id", shopCtl.UpdateShopToEtsy)
			shops.DELETE("/:id", shopCtl.DeleteShop)
			shops.POST("/:id/stop", shopCtl.StopShop)
			shops.POST("/:id/resume", shopCtl.ResumeShop)
			shops.POST("/:id/sync", shopCtl.SyncShop)
			// Section
			shops.POST("/:id/sections/sync", shopCtl.SyncSections)
			shops.POST("/:id/sections", shopCtl.CreateSection)
			shops.PUT("/sections/:sectionId", shopCtl.UpdateSection)
			shops.DELETE("/sections/:sectionId", shopCtl.DeleteSection)
			// 店铺下的运费模板 profile
			shops.GET("/:id/shipping-profiles", shippingCtl.GetProfileList)
			shops.POST("/:id/shipping-profiles", shippingCtl.CreateProfile)
			shops.POST("/:id/shipping-profiles/sync", shippingCtl.SyncProfiles)
			// 店铺下的退货政策
			shops.GET("/:id/return-policies", returnPolicyCtl.GetPolicyList)
			shops.POST("/:id/return-policies", returnPolicyCtl.CreatePolicy)
			shops.POST("/:id/return-policies/sync", returnPolicyCtl.SyncPolicies)
		}
		// shipping 发货
		shipping := api.Group("/shipping")
		{
			shipping.GET("/:id", shippingCtl.GetProfileDetail)
			shipping.PUT("/:id", shippingCtl.UpdateProfile)
			shipping.DELETE("/:id", shippingCtl.DeleteProfile)
			// 目的地
			shipping.POST("/:profileID/destinations", shippingCtl.CreateDestination)
			// 升级
			shipping.POST("/:profileID/upgrades", shippingCtl.CreateUpgrade)
			shipping.PUT("/upgrades/:id", shippingCtl.UpdateUpgrade)
			shipping.DELETE("/upgrades/:id", shippingCtl.DeleteUpgrade)
			// 退货政策操作
			shipping.GET("/return-policies/:id", returnPolicyCtl.GetPolicyDetail)
			shipping.PUT("/return-policies/:id", returnPolicyCtl.UpdatePolicy)
			shipping.DELETE("/return-policies/:id", returnPolicyCtl.DeletePolicy)
		}
		//product 组
		products := api.Group("/products")
		{
			products.GET("", productCtrl.GetProducts)
		}
	}
}
