package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/repository"
	"etsy_dev_v1_202512/internal/service"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	AppManageURL = "https://www.etsy.com/developers/your-apps"
)

type DeveloperController struct {
	developerService *service.DeveloperService
}

func NewDeveloperController(developerService *service.DeveloperService) *DeveloperController {
	return &DeveloperController{developerService: developerService}
}

// Create 创建开发者账号
// @Summary 创建开发者账号
// @Description 录入 Etsy 开发者账号，自动生成防关联 CallbackURL
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param request body dto.CreateDeveloperReq true "创建参数"
// @Success 200 {object} map[string]interface{} "callback_url + 管理页地址"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/developers [post]
func (d *DeveloperController) Create(c *gin.Context) {
	var req dto.CreateDeveloperReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url, err := d.developerService.CreateDeveloper(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "success",
		"callback_url": url,
		"manager_url":  AppManageURL,
	})
}

// GetList 获取开发者账号列表
// @Summary 获取开发者账号列表
// @Description 分页查询开发者账号，支持按名称和状态筛选
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param page query int false "页码 (默认1)"
// @Param page_size query int false "每页数量 (默认20)"
// @Param name query string false "名称模糊搜索"
// @Param status query int false "状态 (0:未配置 1:正常 2:封禁，-1或不传表示全部)"
// @Success 200 {object} map[string]interface{} "data + total + page"
// @Failure 500 {object} map[string]string "服务器错误"
// @Router /api/developers [get]
func (d *DeveloperController) GetList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status, _ := strconv.Atoi(c.DefaultQuery("status", "-1"))

	filter := repository.DeveloperListFilter{
		Page:     page,
		PageSize: pageSize,
		Name:     c.Query("name"),
		Status:   status,
	}

	list, total, err := d.developerService.GetDeveloperList(c.Request.Context(), filter)
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

// GetDetail 获取开发者账号详情
// @Summary 获取开发者账号详情
// @Description 根据 ID 获取单个开发者账号详细信息
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param id path int true "开发者账号 ID"
// @Success 200 {object} map[string]interface{} "data: DeveloperResp"
// @Failure 400 {object} map[string]string "ID 格式错误"
// @Failure 500 {object} map[string]string "查询失败"
// @Router /api/developers/{id} [get]
func (d *DeveloperController) GetDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	data, err := d.developerService.GetDeveloperDetail(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": data})
}

// Update 更新开发者账号信息
// @Summary 更新开发者账号信息
// @Description 更新开发者账号的备注名称、登录密码或 SharedSecret
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param id path int true "开发者账号 ID"
// @Param request body dto.UpdateDeveloperReq true "更新参数"
// @Success 200 {object} map[string]string "message: success"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/developers/{id} [put]
func (d *DeveloperController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.UpdateDeveloperReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := d.developerService.UpdateDeveloper(c.Request.Context(), id, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// UpdateStatus 更新开发者账号状态
// @Summary 更新开发者账号状态
// @Description 变更开发者账号状态（启用/停用/封禁）
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param id path int true "开发者账号 ID"
// @Param request body dto.UpdateDevStatusReq true "状态参数"
// @Success 200 {object} map[string]string "message: success"
// @Failure 400 {object} map[string]string "参数错误"
// @Failure 500 {object} map[string]string "更新失败"
// @Router /api/developers/{id}/status [patch]
func (d *DeveloperController) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req dto.UpdateDevStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := d.developerService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Delete 删除开发者账号
// @Summary 删除开发者账号
// @Description 软删除开发者账号，同时解绑所有关联店铺
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param id path int true "开发者账号 ID"
// @Success 200 {object} map[string]string "message: success"
// @Failure 400 {object} map[string]string "ID 格式错误"
// @Failure 500 {object} map[string]string "删除失败"
// @Router /api/developers/{id} [delete]
func (d *DeveloperController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := d.developerService.DeleteDeveloper(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// TestConnectivity 测试 API Key 连通性
// @Summary 测试 API Key 连通性
// @Description 通过代理请求 Etsy Ping 接口，验证 API Key 是否有效
// @Tags Developer (开发者账号)
// @Accept json
// @Produce json
// @Param id path int true "开发者账号 ID"
// @Success 200 {object} map[string]interface{} "success + latency_ms"
// @Failure 400 {object} map[string]string "ID 格式错误"
// @Failure 500 {object} map[string]string "测试失败"
// @Router /api/developers/{id}/ping [post]
func (d *DeveloperController) TestConnectivity(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	success, latency, err := d.developerService.TestConnectivity(c.Request.Context(), id)

	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"latency_ms": latency,
			"error":      err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    success,
		"latency_ms": latency,
		"message":    "API Key 验证成功",
	})
}
