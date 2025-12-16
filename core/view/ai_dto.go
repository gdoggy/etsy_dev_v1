package view

// AIGenResult 最终返回给业务层的聚合结果
type AIGenResult struct {
	TextSets []TextSetResult `json:"text_sets"`
	Images   []string        `json:"images"` // URL 列表
	Video    string          `json:"video"`  // URL
}

// TextSetResult 一套文案
type TextSetResult struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}
