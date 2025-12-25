package repository

import (
	"context"
	"etsy_dev_v1_202512/internal/model"

	"gorm.io/gorm"
)

// ShippingProfileRepo 运费模板
type ShippingProfileRepo struct {
	db *gorm.DB
}

func NewShippingProfileRepo(db *gorm.DB) *ShippingProfileRepo {
	return &ShippingProfileRepo{db: db}
}

// Create 创建运费模板
func (r *ShippingProfileRepo) Create(ctx context.Context, profile *model.ShippingProfile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

// GetByID 根据ID获取运费模板
func (r *ShippingProfileRepo) GetByID(ctx context.Context, id int64) (*model.ShippingProfile, error) {
	var profile model.ShippingProfile
	err := r.db.WithContext(ctx).First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetByIDWithRelations 根据ID获取运费模板（含关联数据）
func (r *ShippingProfileRepo) GetByIDWithRelations(ctx context.Context, id int64) (*model.ShippingProfile, error) {
	var profile model.ShippingProfile
	err := r.db.WithContext(ctx).
		Preload("Destinations").
		Preload("Upgrades").
		First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// GetByShopID 根据店铺ID获取所有运费模板
func (r *ShippingProfileRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ShippingProfile, error) {
	var list []model.ShippingProfile
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// GetByShopIDWithRelations 根据店铺ID获取所有运费模板（含关联数据）
func (r *ShippingProfileRepo) GetByShopIDWithRelations(ctx context.Context, shopID int64) ([]model.ShippingProfile, error) {
	var list []model.ShippingProfile
	err := r.db.WithContext(ctx).
		Preload("Destinations").
		Preload("Upgrades").
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// GetByEtsyProfileID 根据Etsy运费模板ID获取
func (r *ShippingProfileRepo) GetByEtsyProfileID(ctx context.Context, shopID int64, etsyProfileID int64) (*model.ShippingProfile, error) {
	var profile model.ShippingProfile
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_profile_id = ?", shopID, etsyProfileID).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// Update 更新运费模板
func (r *ShippingProfileRepo) Update(ctx context.Context, profile *model.ShippingProfile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

// UpdateFields 更新指定字段
func (r *ShippingProfileRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 删除运费模板
func (r *ShippingProfileRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingProfile{}, id).Error
}

// DeleteByShopID 根据店铺ID删除所有运费模板
func (r *ShippingProfileRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ShippingProfile{}).Error
}

// BatchUpsert 批量更新或创建（用于同步）
func (r *ShippingProfileRepo) BatchUpsert(ctx context.Context, shopID int64, profiles []model.ShippingProfile) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, profile := range profiles {
			profile.ShopID = shopID
			err := tx.Where("shop_id = ? AND etsy_profile_id = ?", shopID, profile.EtsyProfileID).
				Assign(profile).
				FirstOrCreate(&profile).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateEtsySyncedAt 更新Etsy同步时间
func (r *ShippingProfileRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}

// Count 统计店铺运费模板数量
func (r *ShippingProfileRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}

// ShippingDestinationRepo 运费目的地
type ShippingDestinationRepo struct {
	db *gorm.DB
}

func NewShippingDestinationRepo(db *gorm.DB) *ShippingDestinationRepo {
	return &ShippingDestinationRepo{db: db}
}

// Create 创建运费目的地
func (r *ShippingDestinationRepo) Create(ctx context.Context, dest *model.ShippingDestination) error {
	return r.db.WithContext(ctx).Create(dest).Error
}

// GetByID 根据ID获取运费目的地
func (r *ShippingDestinationRepo) GetByID(ctx context.Context, id int64) (*model.ShippingDestination, error) {
	var dest model.ShippingDestination
	err := r.db.WithContext(ctx).First(&dest, id).Error
	if err != nil {
		return nil, err
	}
	return &dest, nil
}

// GetByProfileID 根据运费模板ID获取所有目的地
func (r *ShippingDestinationRepo) GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingDestination, error) {
	var list []model.ShippingDestination
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// GetByEtsyDestinationID 根据Etsy目的地ID获取
func (r *ShippingDestinationRepo) GetByEtsyDestinationID(ctx context.Context, profileID int64, etsyDestinationID int64) (*model.ShippingDestination, error) {
	var dest model.ShippingDestination
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ? AND etsy_destination_id = ?", profileID, etsyDestinationID).
		First(&dest).Error
	if err != nil {
		return nil, err
	}
	return &dest, nil
}

// Update 更新运费目的地
func (r *ShippingDestinationRepo) Update(ctx context.Context, dest *model.ShippingDestination) error {
	return r.db.WithContext(ctx).Save(dest).Error
}

// UpdateFields 更新指定字段
func (r *ShippingDestinationRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingDestination{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 删除运费目的地
func (r *ShippingDestinationRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingDestination{}, id).Error
}

// DeleteByProfileID 根据运费模板ID删除所有目的地
func (r *ShippingDestinationRepo) DeleteByProfileID(ctx context.Context, profileID int64) error {
	return r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Delete(&model.ShippingDestination{}).Error
}

// BatchCreate 批量创建目的地
func (r *ShippingDestinationRepo) BatchCreate(ctx context.Context, destinations []model.ShippingDestination) error {
	if len(destinations) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&destinations).Error
}

// BatchUpsert 批量更新或创建（用于同步）
func (r *ShippingDestinationRepo) BatchUpsert(ctx context.Context, profileID int64, destinations []model.ShippingDestination) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, dest := range destinations {
			dest.ShippingProfileID = profileID
			err := tx.Where("shipping_profile_id = ? AND etsy_destination_id = ?", profileID, dest.EtsyDestinationID).
				Assign(dest).
				FirstOrCreate(&dest).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Count 统计运费模板目的地数量
func (r *ShippingDestinationRepo) Count(ctx context.Context, profileID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingDestination{}).
		Where("shipping_profile_id = ?", profileID).
		Count(&count).Error
	return count, err
}

// ShippingUpgradeRepo 加急配送选项
type ShippingUpgradeRepo struct {
	db *gorm.DB
}

func NewShippingUpgradeRepo(db *gorm.DB) *ShippingUpgradeRepo {
	return &ShippingUpgradeRepo{db: db}
}

// Create 创建加急配送选项
func (r *ShippingUpgradeRepo) Create(ctx context.Context, upgrade *model.ShippingUpgrade) error {
	return r.db.WithContext(ctx).Create(upgrade).Error
}

// GetByID 根据ID获取加急配送选项
func (r *ShippingUpgradeRepo) GetByID(ctx context.Context, id int64) (*model.ShippingUpgrade, error) {
	var upgrade model.ShippingUpgrade
	err := r.db.WithContext(ctx).First(&upgrade, id).Error
	if err != nil {
		return nil, err
	}
	return &upgrade, nil
}

// GetByProfileID 根据运费模板ID获取所有加急配送选项
func (r *ShippingUpgradeRepo) GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingUpgrade, error) {
	var list []model.ShippingUpgrade
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// GetByEtsyUpgradeID 根据Etsy升级选项ID获取
func (r *ShippingUpgradeRepo) GetByEtsyUpgradeID(ctx context.Context, profileID int64, etsyUpgradeID int64) (*model.ShippingUpgrade, error) {
	var upgrade model.ShippingUpgrade
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ? AND etsy_upgrade_id = ?", profileID, etsyUpgradeID).
		First(&upgrade).Error
	if err != nil {
		return nil, err
	}
	return &upgrade, nil
}

// Update 更新加急配送选项
func (r *ShippingUpgradeRepo) Update(ctx context.Context, upgrade *model.ShippingUpgrade) error {
	return r.db.WithContext(ctx).Save(upgrade).Error
}

// UpdateFields 更新指定字段
func (r *ShippingUpgradeRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingUpgrade{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 删除加急配送选项
func (r *ShippingUpgradeRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingUpgrade{}, id).Error
}

// DeleteByProfileID 根据运费模板ID删除所有加急配送选项
func (r *ShippingUpgradeRepo) DeleteByProfileID(ctx context.Context, profileID int64) error {
	return r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Delete(&model.ShippingUpgrade{}).Error
}

// BatchCreate 批量创建加急配送选项
func (r *ShippingUpgradeRepo) BatchCreate(ctx context.Context, upgrades []model.ShippingUpgrade) error {
	if len(upgrades) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&upgrades).Error
}

// BatchUpsert 批量更新或创建（用于同步）
func (r *ShippingUpgradeRepo) BatchUpsert(ctx context.Context, profileID int64, upgrades []model.ShippingUpgrade) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, upgrade := range upgrades {
			upgrade.ShippingProfileID = profileID
			err := tx.Where("shipping_profile_id = ? AND etsy_upgrade_id = ?", profileID, upgrade.EtsyUpgradeID).
				Assign(upgrade).
				FirstOrCreate(&upgrade).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// Count 统计运费模板加急配送选项数量
func (r *ShippingUpgradeRepo) Count(ctx context.Context, profileID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingUpgrade{}).
		Where("shipping_profile_id = ?", profileID).
		Count(&count).Error
	return count, err
}

// ReturnPolicyRepo 退货政策
type ReturnPolicyRepo struct {
	db *gorm.DB
}

func NewReturnPolicyRepo(db *gorm.DB) *ReturnPolicyRepo {
	return &ReturnPolicyRepo{db: db}
}

// Create 创建退货政策
func (r *ReturnPolicyRepo) Create(ctx context.Context, policy *model.ReturnPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

// GetByID 根据ID获取退货政策
func (r *ReturnPolicyRepo) GetByID(ctx context.Context, id int64) (*model.ReturnPolicy, error) {
	var policy model.ReturnPolicy
	err := r.db.WithContext(ctx).First(&policy, id).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// GetByShopID 根据店铺ID获取所有退货政策
func (r *ReturnPolicyRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ReturnPolicy, error) {
	var list []model.ReturnPolicy
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

// GetByEtsyPolicyID 根据Etsy退货政策ID获取
func (r *ReturnPolicyRepo) GetByEtsyPolicyID(ctx context.Context, shopID int64, etsyPolicyID int64) (*model.ReturnPolicy, error) {
	var policy model.ReturnPolicy
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_policy_id = ?", shopID, etsyPolicyID).
		First(&policy).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// Update 更新退货政策
func (r *ReturnPolicyRepo) Update(ctx context.Context, policy *model.ReturnPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

// UpdateFields 更新指定字段
func (r *ReturnPolicyRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("id = ?", id).
		Updates(fields).Error
}

// Delete 删除退货政策
func (r *ReturnPolicyRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ReturnPolicy{}, id).Error
}

// DeleteByShopID 根据店铺ID删除所有退货政策
func (r *ReturnPolicyRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ReturnPolicy{}).Error
}

// BatchCreate 批量创建退货政策
func (r *ReturnPolicyRepo) BatchCreate(ctx context.Context, policies []model.ReturnPolicy) error {
	if len(policies) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&policies).Error
}

// BatchUpsert 批量更新或创建（用于同步）
func (r *ReturnPolicyRepo) BatchUpsert(ctx context.Context, shopID int64, policies []model.ReturnPolicy) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, policy := range policies {
			policy.ShopID = shopID
			err := tx.Where("shop_id = ? AND etsy_policy_id = ?", shopID, policy.EtsyPolicyID).
				Assign(policy).
				FirstOrCreate(&policy).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// UpdateEtsySyncedAt 更新Etsy同步时间
func (r *ReturnPolicyRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}

// Count 统计店铺退货政策数量
func (r *ReturnPolicyRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}
