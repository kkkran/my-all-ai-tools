package controller

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// --- Admin: Payment Gateway Management ---

// ListPaymentGateways returns all configured payment gateways
func ListPaymentGateways(c *gin.Context) {
	gateways, err := model.GetEnabledPaymentGateways()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// Also get disabled ones
	allGateways, err := getAllPaymentGateways()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    allGateways,
	})
}

func getAllPaymentGateways() ([]model.PaymentGateway, error) {
	var gateways []model.PaymentGateway
	err := model.DB.Order("sort_order asc, id asc").Find(&gateways).Error
	return gateways, err
}

// CreatePaymentGateway creates a new payment gateway configuration
func CreatePaymentGateway(c *gin.Context) {
	var req struct {
		Name      string                 `json:"name" binding:"required"`
		Type      string                 `json:"type" binding:"required"`
		Enabled   bool                   `json:"enabled"`
		IsDefault bool                   `json:"is_default"`
		Config    map[string]interface{} `json:"config"`
		SortOrder int                    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	gw := &model.PaymentGateway{
		Name:      req.Name,
		Type:      req.Type,
		Enabled:   req.Enabled,
		IsDefault: req.IsDefault,
		SortOrder: req.SortOrder,
	}
	gw.SetConfig(req.Config)

	if err := gw.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gw})
}

// UpdatePaymentGateway updates an existing payment gateway
func UpdatePaymentGateway(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	gw, err := model.GetPaymentGatewayById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "gateway not found"})
		return
	}

	var req struct {
		Name      *string                 `json:"name"`
		Type      *string                 `json:"type"`
		Enabled   *bool                   `json:"enabled"`
		IsDefault *bool                   `json:"is_default"`
		Config    map[string]interface{}  `json:"config"`
		SortOrder *int                    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.Name != nil {
		gw.Name = *req.Name
	}
	if req.Type != nil {
		gw.Type = *req.Type
	}
	if req.Enabled != nil {
		gw.Enabled = *req.Enabled
	}
	if req.IsDefault != nil {
		gw.IsDefault = *req.IsDefault
	}
	if req.Config != nil {
		gw.SetConfig(req.Config)
	}
	if req.SortOrder != nil {
		gw.SortOrder = *req.SortOrder
	}

	if err := gw.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gw})
}

// DeletePaymentGateway deletes a payment gateway
func DeletePaymentGateway(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid id"})
		return
	}

	gw, err := model.GetPaymentGatewayById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "gateway not found"})
		return
	}

	if err := gw.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "gateway deleted"})
}

// --- User-facing: Payment API ---

// GetPaymentMethods returns available payment methods for users
func GetPaymentMethods(c *gin.Context) {
	methods, err := service.GetEnabledPaymentMethods()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    methods,
	})
}

// CreatePayment creates a payment order
func CreatePayment(c *gin.Context) {
	userId := c.GetInt("id")

	var req struct {
		GatewayId   int     `json:"gateway_id" binding:"required"`
		Amount      float64 `json:"amount" binding:"required"`
		Currency    string  `json:"currency"`
		Description string  `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Description == "" {
		req.Description = fmt.Sprintf("Top-up for user %d", userId)
	}

	transaction, payInfo, err := service.CreatePaymentOrder(c, userId, req.GatewayId, req.Amount, req.Currency, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"transaction": transaction,
			"pay_url":     payInfo,
		},
	})
}

// PaymentWebhook handles payment gateway callbacks
func PaymentWebhook(c *gin.Context) {
	gatewayIdStr := c.Param("gatewayId")
	gatewayId, err := strconv.Atoi(gatewayIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid gateway id"})
		return
	}

	bodySeeker, err := common.GetRequestBody(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "failed to read body"})
		return
	}
	rawBody, err := io.ReadAll(bodySeeker)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "failed to read body"})
		return
	}

	transaction, err := service.ProcessPaymentWebhook(c, gatewayId, rawBody)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"status":      transaction.Status,
			"order_no":    transaction.OrderNo,
			"trade_no":    transaction.TransactionNo,
		},
	})
}

// GetPaymentTransactions returns user's payment transactions
func GetPaymentTransactions(c *gin.Context) {
	userId := c.GetInt("id")
	pageInfo := common.GetPageInfo(c)

	transactions, total, err := model.GetUserPaymentTransactions(userId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    transactions,
		"total":   total,
	})
}

// --- Payment Gateway Types list ---

func ListPaymentGatewayTypes(c *gin.Context) {
	types := []gin.H{
		{"type": model.GatewayTypeStripe, "name": "Stripe", "description": "International credit card payments"},
		{"type": model.GatewayTypePayPal, "name": "PayPal", "description": "PayPal digital wallet"},
		{"type": model.GatewayTypeAlipay, "name": "Alipay", "description": "支付宝"},
		{"type": model.GatewayTypeWeChatPay, "name": "WeChat Pay", "description": "微信支付"},
		{"type": model.GatewayTypeEpay, "name": "Epay", "description": "Chinese payment aggregator"},
		{"type": model.GatewayTypeCreem, "name": "Creem", "description": "Subscription payment provider"},
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": types})
}
