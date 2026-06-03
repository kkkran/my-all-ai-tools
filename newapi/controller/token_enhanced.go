package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// --- Enhanced Token Management ---

// UpdateTokenScopes updates a token's permission scopes
func UpdateTokenScopes(c *gin.Context) {
	userId := c.GetInt("id")
	tokenIdStr := c.Param("id")
	tokenId, err := strconv.Atoi(tokenIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token id"})
		return
	}

	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "token not found"})
		return
	}

	var req struct {
		Scopes []string `json:"scopes" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Validate scopes
	for _, s := range req.Scopes {
		if s != model.TokenScopeRead && s != model.TokenScopeWrite && s != model.TokenScopeAdmin {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid scope: " + s})
			return
		}
	}

	token.SetScopes(req.Scopes)
	if err := token.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":     token.Id,
		"scopes": token.GetScopes(),
	}})
}

// UpdateTokenRateLimit updates a token's rate limit (requests per minute)
func UpdateTokenRateLimit(c *gin.Context) {
	userId := c.GetInt("id")
	tokenIdStr := c.Param("id")
	tokenId, err := strconv.Atoi(tokenIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token id"})
		return
	}

	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "token not found"})
		return
	}

	var req struct {
		RateLimitRPM int `json:"rate_limit_rpm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.RateLimitRPM < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "rate_limit_rpm must be >= 0"})
		return
	}

	token.RateLimitRPM = req.RateLimitRPM
	if err := token.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"id":             token.Id,
		"rate_limit_rpm": token.RateLimitRPM,
	}})
}

// RotateTokenKey performs key rotation for a token
func RotateTokenKey(c *gin.Context) {
	userId := c.GetInt("id")
	tokenIdStr := c.Param("id")
	tokenId, err := strconv.Atoi(tokenIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token id"})
		return
	}

	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "token not found"})
		return
	}

	newKey, err := token.RotateKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":                     token.Id,
			"new_key":                newKey,
			"rotation_expires_at":    token.RotationExpiresAt,
			"rotation_expires_in":    token.RotationExpiresAt - common.GetTimestamp(),
		},
	})
}

// GetTokenUsageStats returns usage statistics for a specific token
func GetTokenUsageStats(c *gin.Context) {
	userId := c.GetInt("id")
	tokenIdStr := c.Param("id")
	tokenId, err := strconv.Atoi(tokenIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid token id"})
		return
	}

	// Verify ownership
	token, err := model.GetTokenByIds(tokenId, userId)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "token not found"})
		return
	}

	// Default: last 30 days
	now := common.GetTimestamp()
	startTime := now - 86400*30

	stats, err := model.GetTokenUsageStats(tokenId, startTime, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"token_id": token.Id,
			"name":     token.Name,
			"stats":    stats,
			"period":   "30d",
		},
	})
}

// GetTokensUsageSummary returns usage summary for all user tokens
func GetTokensUsageSummary(c *gin.Context) {
	userId := c.GetInt("id")

	now := common.GetTimestamp()
	startTime := now - 86400*30 // last 30 days

	summary, err := model.GetUserTokensUsageSummary(userId, startTime, now)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}
