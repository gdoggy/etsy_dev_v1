package controller

import "etsy_dev_v1_202512/internal/core/service"

type ShopController struct {
	shopService *service.ShopService
}

func NewShopController(shopService *service.ShopService) *ShopController {
	return &ShopController{shopService: shopService}
}
