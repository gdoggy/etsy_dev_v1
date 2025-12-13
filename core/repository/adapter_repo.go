package repository

import (
	"etsy_dev_v1_202512/core/model"

	"gorm.io/gorm"
)

type AdapterRepository struct {
	DB *gorm.DB
}

func NewAdapterRepo(db *gorm.DB) *AdapterRepository {
	return &AdapterRepository{DB: db}
}

// FindAvailableAdapter 寻找一个绑定店铺数少于 limit 的可用 Adapter
func (r *AdapterRepository) FindAvailableAdapter(limit int) (*model.Adapter, error) {
	var adapter model.Adapter

	// 复杂的 SQL 逻辑：
	// 查找所有 status=1 的 adapter
	// 关联 shops 表，统计数量
	// 筛选出 数量 < limit 的第一个 adapter
	err := r.DB.Model(&model.Adapter{}).
		Select("adapters.*, count(shops.id) as shop_count").
		Joins("LEFT JOIN shops ON shops.adapter_id = adapters.id").
		Where("adapters.status = ?", 1).
		Group("adapters.id").
		Having("count(shops.id) < ?", limit).
		First(&adapter).Error

	return &adapter, err
}

// FindByID 根据 ID 获取 Adapter
func (r *AdapterRepository) FindByID(id uint) (*model.Adapter, error) {
	var adapter model.Adapter
	err := r.DB.First(&adapter, id).Error
	return &adapter, err
}
