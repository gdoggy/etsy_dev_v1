package repository

import (
	"etsy_dev_v1_202512/core/model"

	"gorm.io/gorm"
)

type ShopRepository struct {
	DB *gorm.DB
}

func NewShopRepo(db *gorm.DB) *ShopRepository {
	return &ShopRepository{DB: db}
}

// Create 创建或保存店铺信息
func (r *ShopRepository) Create(shop *model.Shop) error {
	return r.DB.Create(shop).Error
}

// SaveOrUpdate 保存或更新店铺信息
// 解决重复授权时的唯一键冲突问题
func (r *ShopRepository) SaveOrUpdate(shop *model.Shop) error {
	var existingShop model.Shop

	// 1. 根据业务唯一键 EtsyShopID 查询
	err := r.DB.Where("etsy_shop_id = ?", shop.EtsyShopID).First(&existingShop).Error

	if err == nil {
		// Case A: 记录已存在 -> 执行更新
		// 关键步骤：必须把数据库里的主键 ID 赋给当前的 shop 对象
		// 否则 GORM 不知道更新哪一行，会尝试创建新行导致报错
		shop.ID = existingShop.ID
		shop.CreatedAt = existingShop.CreatedAt

		// Save 会更新所有字段 (包括 AccessToken, RefreshToken 等)
		return r.DB.Save(shop).Error
	}

	// Case B: 记录不存在 -> 执行创建
	return r.DB.Create(shop).Error
}

// FindByEtsyID 根据 Etsy 官方 ShopID 查找
func (r *ShopRepository) FindByEtsyID(etsyShopID string) (*model.Shop, error) {
	var shop model.Shop
	err := r.DB.Where("etsy_shop_id = ?", etsyShopID).First(&shop).Error
	return &shop, err
}
