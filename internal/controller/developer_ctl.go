package controller

import (
	"etsy_dev_v1_202512/internal/api/dto"
	"etsy_dev_v1_202512/internal/service"
	"net/http"

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
		"message":     "success",
		"url":         url,
		"manager_url": AppManageURL,
	})
}
