package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// --- Model Catalog: Public API ---

// GetModelCatalog returns published models for the marketplace
func GetModelCatalog(c *gin.Context) {
	category := c.Query("category")
	pageInfo := common.GetPageInfo(c)

	models, total, err := model.GetPublishedModels(category, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
		"total":   total,
	})
}

// GetFeaturedModels returns featured models for the landing page
func GetFeaturedModels(c *gin.Context) {
	models, err := model.GetFeaturedModels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetModelDetail returns a single model's details
func GetModelDetail(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	mc, err := model.GetModelCatalogById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"model":        mc,
			"pricing":      mc.GetPricingInfo(),
			"capabilities": mc.GetCapabilities(),
		},
	})
}

// SearchModels searches the model catalog
func SearchModels(c *gin.Context) {
	keyword := c.Query("q")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "search query required"})
		return
	}

	models, err := model.SearchModelCatalog(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetModelsByProvider returns models filtered by provider
func GetModelsByProvider(c *gin.Context) {
	provider := c.Query("provider")
	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "provider required"})
		return
	}

	models, err := model.GetModelsByProvider(provider)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    models,
	})
}

// GetModelCategories returns all model categories
func GetModelCategories(c *gin.Context) {
	categories, err := model.GetModelCategories()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": categories})
}

// GetModelTags returns all model tags
func GetModelTags(c *gin.Context) {
	tags, err := model.GetAllModelTags()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": tags})
}

// --- Admin: Model Catalog Management ---

// AdminListModelCatalog returns all models (including drafts) for admin
func AdminListModelCatalog(c *gin.Context) {
	var models []model.ModelCatalog
	err := model.DB.Order("sort_order asc, id desc").Find(&models).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": models})
}

// AdminCreateModelCatalog creates a new model catalog entry
func AdminCreateModelCatalog(c *gin.Context) {
	var mc model.ModelCatalog
	if err := c.ShouldBindJSON(&mc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := mc.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": mc})
}

// AdminUpdateModelCatalog updates a model catalog entry
func AdminUpdateModelCatalog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	mc, err := model.GetModelCatalogById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return
	}

	var req model.ModelCatalog
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Merge fields
	if req.DisplayName != "" {
		mc.DisplayName = req.DisplayName
	}
	if req.Description != "" {
		mc.Description = req.Description
	}
	if req.LongDescription != "" {
		mc.LongDescription = req.LongDescription
	}
	if req.Icon != "" {
		mc.Icon = req.Icon
	}
	if req.Category != "" {
		mc.Category = req.Category
	}
	if req.Tags != "" {
		mc.Tags = req.Tags
	}
	if req.PricingInfo != "" {
		mc.PricingInfo = req.PricingInfo
	}
	if req.Capabilities != "" {
		mc.Capabilities = req.Capabilities
	}
	if req.DocURL != "" {
		mc.DocURL = req.DocURL
	}
	if req.Provider != "" {
		mc.Provider = req.Provider
	}
	mc.Status = req.Status
	mc.Featured = req.Featured
	mc.SortOrder = req.SortOrder

	if err := mc.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": mc})
}

// AdminDeleteModelCatalog deletes a model catalog entry
func AdminDeleteModelCatalog(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	mc, err := model.GetModelCatalogById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return
	}

	if err := mc.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

// --- Admin: Category Management ---

// AdminCreateCategory creates a new category
func AdminCreateCategory(c *gin.Context) {
	var cat model.ModelCategory
	if err := c.ShouldBindJSON(&cat); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := cat.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": cat})
}
