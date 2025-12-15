package view

import (
	"etsy_dev_v1_202512/core/model"
	"strconv"
)

// ProductResp 是专门给前端页面展示用的
type ProductResp struct {
	ID        uint   `json:"id"`
	ListingID string `json:"listing_id"` // 这里的 ID 转字符串防止前端大数精度丢失
	Title     string `json:"title"`
	ImgUrl    string `json:"img_url"` // 暂时留空，后续可以加
	State     string `json:"state"`

	// 组合字段：直接给前端一个浮点数价格
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`

	Quantity int      `json:"quantity"`
	Views    int      `json:"views"`
	Tags     []string `json:"tags"`

	LastUpdated string `json:"last_updated"`
}

// ProductListResp 专门用于 Swagger 文档和 API 响应的列表结构
type ProductListResp struct {
	Code     int           `json:"code" example:"0"`
	Message  string        `json:"message" example:"success"`
	Data     []ProductResp `json:"data"`
	Total    int64         `json:"total" example:"100"`
	Page     int           `json:"page" example:"1"`
	PageSize int           `json:"page_size" example:"20"`
}

// ToProductVO 将数据库 Model 转换为前端 VO (Assembler)
func ToProductVO(p *model.Product) ProductResp {
	// 计算真实价格
	realPrice := 0.0
	if p.PriceDivisor > 0 {
		realPrice = float64(p.PriceAmount) / float64(p.PriceDivisor)
	}

	return ProductResp{
		ID:          p.ID,
		ListingID:   strconv.FormatInt(p.ListingID, 10),
		Title:       p.Title,
		State:       p.State,
		Price:       realPrice,
		Currency:    p.CurrencyCode,
		Quantity:    p.Quantity,
		Views:       p.Views,
		Tags:        p.Tags, // GORM 已经把 JSON 字符串转回 []string 了
		LastUpdated: p.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
}
