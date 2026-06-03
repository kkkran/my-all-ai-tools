package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// --- Audit Log API ---

// ListAuditLogs returns audit logs (admin only)
func ListAuditLogs(c *gin.Context) {
	userIdStr := c.Query("user_id")
	action := c.Query("action")
	resource := c.Query("resource")

	var userId int
	if userIdStr != "" {
		userId, _ = strconv.Atoi(userIdStr)
	}

	pageInfo := common.GetPageInfo(c)
	logs, total, err := model.GetAuditLogs(userId, action, resource, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   total,
	})
}

// ExportAuditLogs exports audit logs by date range
func ExportAuditLogs(c *gin.Context) {
	startStr := c.Query("start")
	endStr := c.Query("end")

	startTime, err := strconv.ParseInt(startStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid start time"})
		return
	}
	endTime, err := strconv.ParseInt(endStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid end time"})
		return
	}

	logs, err := model.GetAuditLogsByDateRange(startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

// --- Monitoring Metrics API ---

// GetMetrics returns monitoring metrics
func GetMetrics(c *gin.Context) {
	metricType := c.Query("type") // qps, tps, token_throughput, latency, error_rate, channel_success_rate
	windowStartStr := c.Query("start")
	windowEndStr := c.Query("end")
	channelIdStr := c.Query("channel_id")
	modelName := c.Query("model")

	var windowStart, windowEnd int64
	now := common.GetTimestamp()
	if windowStartStr == "" {
		windowStart = now - 3600 // default: last hour
	} else {
		windowStart, _ = strconv.ParseInt(windowStartStr, 10, 64)
	}
	if windowEndStr == "" {
		windowEnd = now
	} else {
		windowEnd, _ = strconv.ParseInt(windowEndStr, 10, 64)
	}

	var channelId int
	if channelIdStr != "" {
		channelId, _ = strconv.Atoi(channelIdStr)
	}

	metrics, err := model.GetMetricsByType(metricType, windowStart, windowEnd, channelId, modelName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetAggregatedMetrics returns aggregated monitoring metrics
func GetAggregatedMetrics(c *gin.Context) {
	metricType := c.Query("type")
	windowStartStr := c.Query("start")
	windowEndStr := c.Query("end")

	now := common.GetTimestamp()
	windowStart, _ := strconv.ParseInt(windowStartStr, 10, 64)
	windowEnd, _ := strconv.ParseInt(windowEndStr, 10, 64)
	if windowStart == 0 {
		windowStart = now - 3600
	}
	if windowEnd == 0 {
		windowEnd = now
	}

	metrics, err := model.GetAggregatedMetrics(metricType, windowStart, windowEnd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// --- Alert Rules API (Admin) ---

// ListAlertRules returns all alert rules
func ListAlertRules(c *gin.Context) {
	rules, err := model.GetAllAlertRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rules})
}

// CreateAlertRule creates a new alert rule
func CreateAlertRule(c *gin.Context) {
	var rule model.MonitorAlertRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := rule.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": rule})
}

// UpdateAlertRule updates an alert rule
func UpdateAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	// Find existing rule
	var existing model.MonitorAlertRule
	if err := model.DB.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "rule not found"})
		return
	}

	var req model.MonitorAlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	req.Id = id
	if err := req.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": req})
}

// DeleteAlertRule deletes an alert rule
func DeleteAlertRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	rule := &model.MonitorAlertRule{Id: id}
	if err := rule.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

// GetAlertEvents returns alert events
func GetAlertEvents(c *gin.Context) {
	ruleIdStr := c.Query("rule_id")
	acknowledgedStr := c.Query("acknowledged")

	var ruleId int
	if ruleIdStr != "" {
		ruleId, _ = strconv.Atoi(ruleIdStr)
	}
	acknowledged := acknowledgedStr == "true"

	pageInfo := common.GetPageInfo(c)
	events, total, err := model.GetAlertEvents(ruleId, acknowledged, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   total,
	})
}

// AcknowledgeAlert acknowledges an alert event
func AcknowledgeAlert(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	if err := model.AcknowledgeAlertEvent(id, userId); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "acknowledged"})
}
