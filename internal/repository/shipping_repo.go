package repository

import (
	"context"

	"gorm.io/gorm"

	"etsy_dev_v1_202512/internal/model"
)

// ==================== ShippingProfile 接口定义 ====================

// ShippingProfileRepository 运费模板仓储接口
type ShippingProfileRepository interface {
	// 基础 CRUD
	Create(ctx context.Context, profile *model.ShippingProfile) error
	GetByID(ctx context.Context, id int64) (*model.ShippingProfile, error)
	GetByIDWithRelations(ctx context.Context, id int64) (*model.ShippingProfile, error)
	Update(ctx context.Context, profile *model.ShippingProfile) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error

	// 查询
	GetByShopID(ctx context.Context, shopID int64) ([]model.ShippingProfile, error)
	GetByShopIDWithRelations(ctx context.Context, shopID int64) ([]model.ShippingProfile, error)
	GetByEtsyProfileID(ctx context.Context, shopID int64, etsyProfileID int64) (*model.ShippingProfile, error)
	Count(ctx context.Context, shopID int64) (int64, error)

	// 批量操作
	BatchUpsert(ctx context.Context, shopID int64, profiles []model.ShippingProfile) error
	DeleteByShopID(ctx context.Context, shopID int64) error

	// 同步
	UpdateEtsySyncedAt(ctx context.Context, id int64) error
}

// ==================== ShippingProfile 实现 ====================

type shippingProfileRepo struct {
	db *gorm.DB
}

// NewShippingProfileRepository 创建运费模板仓储
func NewShippingProfileRepository(db *gorm.DB) ShippingProfileRepository {
	return &shippingProfileRepo{db: db}
}

func (r *shippingProfileRepo) Create(ctx context.Context, profile *model.ShippingProfile) error {
	return r.db.WithContext(ctx).Create(profile).Error
}

func (r *shippingProfileRepo) GetByID(ctx context.Context, id int64) (*model.ShippingProfile, error) {
	var profile model.ShippingProfile
	err := r.db.WithContext(ctx).First(&profile, id).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *shippingProfileRepo) GetByIDWithRelations(ctx context.Context, id int64) (*model.ShippingProfile, error) {
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

func (r *shippingProfileRepo) Update(ctx context.Context, profile *model.ShippingProfile) error {
	return r.db.WithContext(ctx).Save(profile).Error
}

func (r *shippingProfileRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *shippingProfileRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingProfile{}, id).Error
}

func (r *shippingProfileRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ShippingProfile, error) {
	var list []model.ShippingProfile
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *shippingProfileRepo) GetByShopIDWithRelations(ctx context.Context, shopID int64) ([]model.ShippingProfile, error) {
	var list []model.ShippingProfile
	err := r.db.WithContext(ctx).
		Preload("Destinations").
		Preload("Upgrades").
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *shippingProfileRepo) GetByEtsyProfileID(ctx context.Context, shopID int64, etsyProfileID int64) (*model.ShippingProfile, error) {
	var profile model.ShippingProfile
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_profile_id = ?", shopID, etsyProfileID).
		First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *shippingProfileRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}

func (r *shippingProfileRepo) BatchUpsert(ctx context.Context, shopID int64, profiles []model.ShippingProfile) error {
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

func (r *shippingProfileRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ShippingProfile{}).Error
}

func (r *shippingProfileRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingProfile{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}

// ==================== ShippingDestination 接口定义 ====================

// ShippingDestinationRepository 运费目的地仓储接口
type ShippingDestinationRepository interface {
	Create(ctx context.Context, dest *model.ShippingDestination) error
	GetByID(ctx context.Context, id int64) (*model.ShippingDestination, error)
	Update(ctx context.Context, dest *model.ShippingDestination) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error

	GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingDestination, error)
	GetByEtsyDestinationID(ctx context.Context, profileID int64, etsyDestinationID int64) (*model.ShippingDestination, error)
	Count(ctx context.Context, profileID int64) (int64, error)

	BatchCreate(ctx context.Context, destinations []model.ShippingDestination) error
	BatchUpsert(ctx context.Context, profileID int64, destinations []model.ShippingDestination) error
	DeleteByProfileID(ctx context.Context, profileID int64) error
}

// ==================== ShippingDestination 实现 ====================

type shippingDestinationRepo struct {
	db *gorm.DB
}

// NewShippingDestinationRepository 创建运费目的地仓储
func NewShippingDestinationRepository(db *gorm.DB) ShippingDestinationRepository {
	return &shippingDestinationRepo{db: db}
}

func (r *shippingDestinationRepo) Create(ctx context.Context, dest *model.ShippingDestination) error {
	return r.db.WithContext(ctx).Create(dest).Error
}

func (r *shippingDestinationRepo) GetByID(ctx context.Context, id int64) (*model.ShippingDestination, error) {
	var dest model.ShippingDestination
	err := r.db.WithContext(ctx).First(&dest, id).Error
	if err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *shippingDestinationRepo) Update(ctx context.Context, dest *model.ShippingDestination) error {
	return r.db.WithContext(ctx).Save(dest).Error
}

func (r *shippingDestinationRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingDestination{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *shippingDestinationRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingDestination{}, id).Error
}

func (r *shippingDestinationRepo) GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingDestination, error) {
	var list []model.ShippingDestination
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *shippingDestinationRepo) GetByEtsyDestinationID(ctx context.Context, profileID int64, etsyDestinationID int64) (*model.ShippingDestination, error) {
	var dest model.ShippingDestination
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ? AND etsy_destination_id = ?", profileID, etsyDestinationID).
		First(&dest).Error
	if err != nil {
		return nil, err
	}
	return &dest, nil
}

func (r *shippingDestinationRepo) Count(ctx context.Context, profileID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingDestination{}).
		Where("shipping_profile_id = ?", profileID).
		Count(&count).Error
	return count, err
}

func (r *shippingDestinationRepo) BatchCreate(ctx context.Context, destinations []model.ShippingDestination) error {
	if len(destinations) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&destinations).Error
}

func (r *shippingDestinationRepo) BatchUpsert(ctx context.Context, profileID int64, destinations []model.ShippingDestination) error {
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

func (r *shippingDestinationRepo) DeleteByProfileID(ctx context.Context, profileID int64) error {
	return r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Delete(&model.ShippingDestination{}).Error
}

// ==================== ShippingUpgrade 接口定义 ====================

// ShippingUpgradeRepository 加急配送选项仓储接口
type ShippingUpgradeRepository interface {
	Create(ctx context.Context, upgrade *model.ShippingUpgrade) error
	GetByID(ctx context.Context, id int64) (*model.ShippingUpgrade, error)
	Update(ctx context.Context, upgrade *model.ShippingUpgrade) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error

	GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingUpgrade, error)
	GetByEtsyUpgradeID(ctx context.Context, profileID int64, etsyUpgradeID int64) (*model.ShippingUpgrade, error)
	Count(ctx context.Context, profileID int64) (int64, error)

	BatchCreate(ctx context.Context, upgrades []model.ShippingUpgrade) error
	BatchUpsert(ctx context.Context, profileID int64, upgrades []model.ShippingUpgrade) error
	DeleteByProfileID(ctx context.Context, profileID int64) error
}

// ==================== ShippingUpgrade 实现 ====================

type shippingUpgradeRepo struct {
	db *gorm.DB
}

// NewShippingUpgradeRepository 创建加急配送选项仓储
func NewShippingUpgradeRepository(db *gorm.DB) ShippingUpgradeRepository {
	return &shippingUpgradeRepo{db: db}
}

func (r *shippingUpgradeRepo) Create(ctx context.Context, upgrade *model.ShippingUpgrade) error {
	return r.db.WithContext(ctx).Create(upgrade).Error
}

func (r *shippingUpgradeRepo) GetByID(ctx context.Context, id int64) (*model.ShippingUpgrade, error) {
	var upgrade model.ShippingUpgrade
	err := r.db.WithContext(ctx).First(&upgrade, id).Error
	if err != nil {
		return nil, err
	}
	return &upgrade, nil
}

func (r *shippingUpgradeRepo) Update(ctx context.Context, upgrade *model.ShippingUpgrade) error {
	return r.db.WithContext(ctx).Save(upgrade).Error
}

func (r *shippingUpgradeRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ShippingUpgrade{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *shippingUpgradeRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ShippingUpgrade{}, id).Error
}

func (r *shippingUpgradeRepo) GetByProfileID(ctx context.Context, profileID int64) ([]model.ShippingUpgrade, error) {
	var list []model.ShippingUpgrade
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *shippingUpgradeRepo) GetByEtsyUpgradeID(ctx context.Context, profileID int64, etsyUpgradeID int64) (*model.ShippingUpgrade, error) {
	var upgrade model.ShippingUpgrade
	err := r.db.WithContext(ctx).
		Where("shipping_profile_id = ? AND etsy_upgrade_id = ?", profileID, etsyUpgradeID).
		First(&upgrade).Error
	if err != nil {
		return nil, err
	}
	return &upgrade, nil
}

func (r *shippingUpgradeRepo) Count(ctx context.Context, profileID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ShippingUpgrade{}).
		Where("shipping_profile_id = ?", profileID).
		Count(&count).Error
	return count, err
}

func (r *shippingUpgradeRepo) BatchCreate(ctx context.Context, upgrades []model.ShippingUpgrade) error {
	if len(upgrades) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&upgrades).Error
}

func (r *shippingUpgradeRepo) BatchUpsert(ctx context.Context, profileID int64, upgrades []model.ShippingUpgrade) error {
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

func (r *shippingUpgradeRepo) DeleteByProfileID(ctx context.Context, profileID int64) error {
	return r.db.WithContext(ctx).
		Where("shipping_profile_id = ?", profileID).
		Delete(&model.ShippingUpgrade{}).Error
}

// ==================== ReturnPolicy 接口定义 ====================

// ReturnPolicyRepository 退货政策仓储接口
type ReturnPolicyRepository interface {
	Create(ctx context.Context, policy *model.ReturnPolicy) error
	GetByID(ctx context.Context, id int64) (*model.ReturnPolicy, error)
	Update(ctx context.Context, policy *model.ReturnPolicy) error
	UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error
	Delete(ctx context.Context, id int64) error

	GetByShopID(ctx context.Context, shopID int64) ([]model.ReturnPolicy, error)
	GetByEtsyPolicyID(ctx context.Context, shopID int64, etsyPolicyID int64) (*model.ReturnPolicy, error)
	Count(ctx context.Context, shopID int64) (int64, error)

	BatchCreate(ctx context.Context, policies []model.ReturnPolicy) error
	BatchUpsert(ctx context.Context, shopID int64, policies []model.ReturnPolicy) error
	DeleteByShopID(ctx context.Context, shopID int64) error

	UpdateEtsySyncedAt(ctx context.Context, id int64) error
}

// ==================== ReturnPolicy 实现 ====================

type returnPolicyRepo struct {
	db *gorm.DB
}

// NewReturnPolicyRepository 创建退货政策仓储
func NewReturnPolicyRepository(db *gorm.DB) ReturnPolicyRepository {
	return &returnPolicyRepo{db: db}
}

func (r *returnPolicyRepo) Create(ctx context.Context, policy *model.ReturnPolicy) error {
	return r.db.WithContext(ctx).Create(policy).Error
}

func (r *returnPolicyRepo) GetByID(ctx context.Context, id int64) (*model.ReturnPolicy, error) {
	var policy model.ReturnPolicy
	err := r.db.WithContext(ctx).First(&policy, id).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *returnPolicyRepo) Update(ctx context.Context, policy *model.ReturnPolicy) error {
	return r.db.WithContext(ctx).Save(policy).Error
}

func (r *returnPolicyRepo) UpdateFields(ctx context.Context, id int64, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("id = ?", id).
		Updates(fields).Error
}

func (r *returnPolicyRepo) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ReturnPolicy{}, id).Error
}

func (r *returnPolicyRepo) GetByShopID(ctx context.Context, shopID int64) ([]model.ReturnPolicy, error) {
	var list []model.ReturnPolicy
	err := r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Order("id ASC").
		Find(&list).Error
	return list, err
}

func (r *returnPolicyRepo) GetByEtsyPolicyID(ctx context.Context, shopID int64, etsyPolicyID int64) (*model.ReturnPolicy, error) {
	var policy model.ReturnPolicy
	err := r.db.WithContext(ctx).
		Where("shop_id = ? AND etsy_policy_id = ?", shopID, etsyPolicyID).
		First(&policy).Error
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (r *returnPolicyRepo) Count(ctx context.Context, shopID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("shop_id = ?", shopID).
		Count(&count).Error
	return count, err
}

func (r *returnPolicyRepo) BatchCreate(ctx context.Context, policies []model.ReturnPolicy) error {
	if len(policies) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&policies).Error
}

func (r *returnPolicyRepo) BatchUpsert(ctx context.Context, shopID int64, policies []model.ReturnPolicy) error {
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

func (r *returnPolicyRepo) DeleteByShopID(ctx context.Context, shopID int64) error {
	return r.db.WithContext(ctx).
		Where("shop_id = ?", shopID).
		Delete(&model.ReturnPolicy{}).Error
}

func (r *returnPolicyRepo) UpdateEtsySyncedAt(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.ReturnPolicy{}).
		Where("id = ?", id).
		Update("etsy_synced_at", gorm.Expr("NOW()")).Error
}
