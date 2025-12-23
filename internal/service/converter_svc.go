package service

import (
	"encoding/json"
	"etsy_dev_v1_202512/internal/model"
	"etsy_dev_v1_202512/pkg/etsy"

	"github.com/lib/pq"
	"gorm.io/datatypes"
)

func ToProductModel(dto etsy.ProductListingDTO) *model.Product {
	return &model.Product{
		// 核心身份
		ListingID: dto.ListingID,
		ShopID:    dto.ShopID,

		// 基础信息
		Title:       dto.Title,
		Description: dto.Description,
		State:       dto.State,
		Url:         dto.URL,
		Language:    dto.Language,

		// 价格打平处理 (Listing 级别的价格，通常用于展示)
		PriceAmount:  dto.Price.Amount,
		PriceDivisor: dto.Price.Divisor,
		CurrencyCode: dto.Price.CurrencyCode,
		Quantity:     dto.Quantity,

		// 数组转换 (Go slice -> Postgres Array)
		Tags:      pq.StringArray(dto.Tags),
		Materials: pq.StringArray(dto.Materials),
		Styles:    pq.StringArray(dto.Style), // 注意 JSON 是 style, DB 是 Styles

		// 时间戳
		EtsyCreationTS:     dto.CreatedTimestamp,
		EtsyEndingTS:       dto.EndingTimestamp,
		EtsyLastModifiedTS: dto.LastModifiedTimestamp,
		EtsyStateTS:        dto.StateTimestamp,

		// 配置项
		ListingType:       dto.ListingType,
		ShopSectionID:     dto.ShopSectionID,
		ShippingProfileID: dto.ShippingProfileID,
		ReturnPolicyID:    dto.ReturnPolicyID,
		IsPersonalizable:  dto.IsPersonalizable,
		ShouldAutoRenew:   dto.ShouldAutoRenew,
		HasVariations:     dto.HasVariations,

		// 默认状态
		SyncStatus: 0,
	}
}

// MergeInventoryToProduct 将库存相关的配置合并到 Product 主表
// 因为 PriceOnProperty 这些字段在 Inventory 接口里，不在 Listing 接口里
func MergeInventoryToProduct(p *model.Product, invDto etsy.InventoryDTO) {
	p.PriceOnProperty = pq.Int64Array(invDto.PriceOnProperty)
	p.QuantityOnProperty = pq.Int64Array(invDto.QuantityOnProperty)
	p.SkuOnProperty = pq.Int64Array(invDto.SkuOnProperty)
}

// ToVariantModels 将 Inventory DTO 转换为变体列表
func ToVariantModels(invDto etsy.InventoryDTO, localProductID int64, shopID int64) ([]model.ProductVariant, error) {
	var variants []model.ProductVariant

	for _, p := range invDto.Products {
		// 1. 提取价格和库存 (取 Offerings 第一个元素)
		var priceAmt, priceDiv int64
		var currCode string
		var qty int
		var isEnabled = true
		var offeringID int64

		if len(p.Offerings) > 0 {
			off := p.Offerings[0]
			offeringID = off.OfferingID
			priceAmt = off.Price.Amount
			priceDiv = off.Price.Divisor
			currCode = off.Price.CurrencyCode
			qty = off.Quantity
			isEnabled = off.IsEnabled
		}

		// 2. 处理属性 (Property Values) -> 转换为 JSONB
		// 目标格式: {"Color": "Red", "Size": "Small"}
		propsMap := make(map[string]interface{})
		for _, pv := range p.PropertyValues {
			if len(pv.Values) > 0 {
				propsMap[pv.PropertyName] = pv.Values[0]
			}
		}
		propsJSON, _ := json.Marshal(propsMap)

		// 3. 保存原始属性用于回写 (Raw Properties)
		rawPropsJSON, _ := json.Marshal(p.PropertyValues)

		// 4. 构建模型
		variant := model.ProductVariant{
			ProductID: localProductID, // 关联主表 ID
			ShopID:    shopID,         // 冗余字段

			EtsyVariantID:  p.ProductID,
			EtsyOfferingID: offeringID,
			EtsySKU:        p.Sku,
			LocalSKU:       p.Sku, // 默认 LocalSKU = EtsySKU

			PriceAmount:  priceAmt,
			PriceDivisor: priceDiv,
			CurrencyCode: currCode,
			Quantity:     qty,
			IsEnabled:    isEnabled,

			Properties:   datatypes.JSON(propsJSON),
			EtsyRawProps: datatypes.JSON(rawPropsJSON),
		}

		variants = append(variants, variant)
	}

	return variants, nil
}
