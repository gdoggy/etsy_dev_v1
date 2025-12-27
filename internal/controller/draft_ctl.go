package controller

import (
	"encoding/json"
	"etsy_dev_v1_202512/internal/api/dto"
	"net/http"
	"strconv"
	"time"

	"etsy_dev_v1_202512/internal/service"

	"github.com/gin-gonic/gin"
)

// ==================== 控制器 ====================

// DraftController 草稿控制器
type DraftController struct {
	draftService *service.DraftService
}

func NewDraftController(draftService *service.DraftService) *DraftController {
	return &DraftController{draftService: draftService}
}

// ==================== API 方法 ====================

// CreateDraft 创建草稿任务
// @Summary 提交URL创建草稿任务
// @Tags Draft
// @Accept json
// @Produce json
// @Param body body dto.CreateDraftRequest true "创建请求"
// @Success 201 {object} dto.CreateDraftResult
// @Router /api/drafts [post]
func (ctrl *DraftController) CreateDraft(c *gin.Context) {
	var req dto.CreateDraftRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// TODO: 从JWT中获取UserID
	if req.UserID == 0 {
		req.UserID = 1 // 临时默认值
	}

	ctx := c.Request.Context()
	result, err := ctrl.draftService.CreateDraft(ctx, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// GetDraftDetail 获取草稿详情
// @Summary 获取草稿任务详情
// @Tags Draft
// @Param task_id path int true "任务ID"
// @Success 200 {object} dto.DraftDetailResponse
// @Router /api/drafts/{task_id} [get]
func (ctrl *DraftController) GetDraftDetail(c *gin.Context) {
	taskIDStr := c.Param("task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的任务ID",
		})
		return
	}

	ctx := c.Request.Context()
	result, err := ctrl.draftService.GetTaskDetail(ctx, taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// UpdateDraftProduct 更新草稿商品
// @Summary 更新草稿商品信息
// @Tags Draft
// @Accept json
// @Param product_id path int true "草稿商品ID"
// @Param body body dto.UpdateDraftProductRequest true "更新内容"
// @Success 200 {object} map[string]interface{}
// @Router /api/drafts/products/{product_id} [patch]
func (ctrl *DraftController) UpdateDraftProduct(c *gin.Context) {
	productIDStr := c.Param("product_id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的商品ID",
		})
		return
	}

	var req dto.UpdateDraftProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.draftService.UpdateDraftProduct(ctx, productID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "更新失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// ConfirmDraftProduct 确认单个草稿商品
// @Summary 确认草稿商品，加入提交队列
// @Tags Draft
// @Param product_id path int true "草稿商品ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/drafts/products/{product_id}/confirm [post]
func (ctrl *DraftController) ConfirmDraftProduct(c *gin.Context) {
	productIDStr := c.Param("product_id")
	productID, err := strconv.ParseInt(productIDStr, 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的商品ID",
		})
		return
	}

	ctx := c.Request.Context()
	if err := ctrl.draftService.ConfirmDraft(ctx, productID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "确认成功，已加入提交队列",
	})
}

// ConfirmAllDrafts 确认任务下所有草稿
// @Summary 确认任务下所有草稿商品
// @Tags Draft
// @Param task_id path int true "任务ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/drafts/{task_id}/confirm-all [post]
func (ctrl *DraftController) ConfirmAllDrafts(c *gin.Context) {
	taskIDStr := c.Param("task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的任务ID",
		})
		return
	}

	ctx := c.Request.Context()
	affected, err := ctrl.draftService.ConfirmAllDrafts(ctx, taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "确认失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "全部确认成功",
		"data": gin.H{
			"confirmed_count": affected,
		},
	})
}

// RegenerateImages 重新生成图片
// @Summary 重新生成指定组的图片
// @Tags Draft
// @Accept json
// @Param task_id path int true "任务ID"
// @Param body body dto.RegenerateImagesRequest true "生成参数"
// @Success 200 {object} map[string]interface{}
// @Router /api/drafts/{task_id}/regenerate-images [post]
func (ctrl *DraftController) RegenerateImages(c *gin.Context) {
	taskIDStr := c.Param("task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的任务ID",
		})
		return
	}

	var req dto.RegenerateImagesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	// TODO: 实现重新生成图片逻辑
	_ = taskID
	_ = req

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "图片重新生成已启动",
	})
}

// StreamProgress SSE 订阅任务进度
// @Summary SSE 实时推送任务进度
// @Tags Draft
// @Param task_id path int true "任务ID"
// @Produce text/event-stream
// @Router /api/drafts/{task_id}/stream [get]
func (ctrl *DraftController) StreamProgress(c *gin.Context) {
	taskIDStr := c.Param("task_id")
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的任务ID",
		})
		return
	}

	// 设置 SSE 响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// 订阅进度
	progressCh := ctrl.draftService.Subscribe(taskID)
	defer ctrl.draftService.Unsubscribe(taskID, progressCh)

	// 发送心跳和进度
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			// 心跳
			c.SSEvent("heartbeat", gin.H{"time": time.Now().Unix()})
			c.Writer.Flush()
		case event, ok := <-progressCh:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			c.SSEvent("progress", string(data))
			c.Writer.Flush()

			// 如果任务完成或失败，关闭连接
			if event.Stage == "done" || event.Stage == "failed" {
				return
			}
		}
	}
}

// GetSupportedPlatforms 获取支持的平台列表
// @Summary 获取支持的来源平台
// @Tags Draft
// @Success 200 {object} dto.SupportedPlatformsResponse
// @Router /api/drafts/platforms [get]
func (ctrl *DraftController) GetSupportedPlatforms(c *gin.Context) {
	result := ctrl.draftService.GetSupportedPlatforms()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result.Platforms,
	})
}

// ListDraftTasks 获取草稿任务列表
// @Summary 获取用户的草稿任务列表
// @Tags Draft
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "状态筛选"
// @Success 200 {object} map[string]interface{}
// @Router /api/drafts [get]
func (ctrl *DraftController) ListDraftTasks(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	// TODO: 从JWT获取UserID
	userID := int64(1)

	ctx := c.Request.Context()
	req := &dto.ListDraftTasksRequest{
		UserID:   userID,
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	}

	tasks, total, err := ctrl.draftService.ListTasks(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "查询失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     0,
		"message":  "success",
		"data":     tasks,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}
