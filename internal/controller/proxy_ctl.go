package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ProxyController struct {
	proxyService *service.ProxyService
}

func NewProxyController(proxyService *service.ProxyService) *ProxyController {
	return &ProxyController{proxyService: proxyService}
}

// ==========================================
// 1. 写操作 (Create / Update)
// ==========================================

// Create 创建代理
// @Summary 创建代理 IP
// @Description 录入新的代理 IP，需保证 IP+Port 唯一
// @Tags Proxy
// @Accept json
// @Produce json
// @Param request body dto.CreateProxyReq true "创建参数"
// @Success 200 {object} map[string]string "{"message": "success"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/proxies [post]
func (h *ProxyController) Create(c *gin.Context) {
	var req dto.CreateProxyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	operatorID := h.getUserID(c)

	if err := h.proxyService.CreateProxy(c.Request.Context(), req, operatorID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Update 更新代理
// @Summary 更新代理信息
// @Description 更新代理配置或状态 (如修改容量、停用代理)
// @Tags Proxy
// @Accept json
// @Produce json
// @Param request body dto.UpdateProxyReq true "更新参数"
// @Success 200 {object} map[string]string "{"message": "success"}"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器内部错误"
// @Router /api/proxies [put]
func (h *ProxyController) Update(c *gin.Context) {
	req := dto.UpdateProxyReq{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 获取审计字段
	operatorID := h.getUserID(c)

	if err := h.proxyService.UpdateProxy(c.Request.Context(), req, operatorID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// ==========================================
// 2. 读操作 (List / Detail)
// ==========================================

// GetList 获取代理列表
// @Summary 获取代理分页列表
// @Description 获取代理列表，支持按 IP、地区、状态过滤。列表项只包含关联数量，不包含详情。
// @Tags Proxy
// @Accept json
// @Produce json
// @Param page query int false "页码 (默认1)"
// @Param page_size query int false "每页数量 (默认20)"
// @Param ip query string false "IP 模糊搜索"
// @Param region query string false "地区代码 (如 US)"
// @Param status query int false "状态 (1:正常 2:过期...)"
// @Param capacity query int false "容量 (1:独享 2:共享)"
// @Success 200 {object} map[string]interface{} "{"data": [dto.ProxyResp], "total": 100, "page": 1}"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/proxies [get]
func (h *ProxyController) GetList(c *gin.Context) {
	// 解析 Query 参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.Query("status"))
	capacity, _ := strconv.Atoi(c.Query("capacity"))

	filter := repository.ProxyFilter{
		Page:     page,
		PageSize: pageSize,
		IP:       c.Query("ip"),
		Region:   c.Query("region"),
		Status:   status,
		Capacity: capacity,
	}

	list, total, err := h.proxyService.GetProxyList(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  list,
		"total": total,
		"page":  page,
	})
}

// GetDetail 获取代理详情
// @Summary 获取代理详情 (含绑定关系)
// @Description 根据 ID 获取代理详细信息，包含已绑定的店铺列表和开发者账号列表
// @Tags Proxy
// @Accept json
// @Produce json
// @Param id path int true "代理 ID"
// @Success 200 {object} map[string]dto.ProxyResp "{"data": dto.ProxyResp}"
// @Failure 400 {object} map[string]string "ID 格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/proxies/{id} [get]
func (h *ProxyController) GetDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	data, err := h.proxyService.GetProxyDetail(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// getUserID 辅助方法
func (h *ProxyController) getUserID(c *gin.Context) int64 {
	if v, exists := c.Get("userID"); exists {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

type apiRes struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Area     string `json:"area"`
	City     string `json:"city"`
	State    string `json:"state"`
	Session  string `json:"session"`
	Life     string `json:"life"`
}

// Callback 通过 API 接收第三方数据 批量获取代理口令
func (h *ProxyController) Callback(c *gin.Context) {
	req := apiRes{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
	fmt.Printf("proxy api callback req:%v", req)

}
