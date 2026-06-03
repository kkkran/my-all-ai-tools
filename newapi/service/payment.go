package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// PaymentGatewayProvider abstracts a payment provider implementation.
// Implementations: Stripe, Alipay, WeChatPay, PayPal, Epay, Creem.
type PaymentGatewayProvider interface {
	// GetType returns the gateway type identifier
	GetType() string

	// CreateOrder creates a payment order and returns payment URL/QR code
	CreateOrder(c *gin.Context, userId int, amount float64, currency string, description string) (orderNo string, payURL string, qrCode string, err error)

	// HandleWebhook processes payment gateway callback notifications
	HandleWebhook(c *gin.Context, rawBody []byte) (tradeNo string, orderNo string, amount float64, currency string, status string, err error)

	// VerifyOrder verifies order status with the gateway
	VerifyOrder(orderNo string) (status string, err error)

	// RefundOrder processes a refund
	RefundOrder(orderNo string, amount float64, reason string) (refundNo string, err error)

	// GetName returns the display name for this gateway
	GetName() string
}

// --- Gateway Registry ---

var gatewayRegistry = make(map[string]func(config map[string]interface{}) (PaymentGatewayProvider, error))

// RegisterGateway registers a payment gateway factory
func RegisterGateway(gwType string, factory func(config map[string]interface{}) (PaymentGatewayProvider, error)) {
	gatewayRegistry[gwType] = factory
}

// GetGateway returns a configured payment gateway instance
func GetGateway(gatewayId int) (PaymentGatewayProvider, error) {
	gw, err := model.GetPaymentGatewayById(gatewayId)
	if err != nil {
		return nil, fmt.Errorf("gateway not found: %w", err)
	}
	if !gw.Enabled {
		return nil, fmt.Errorf("gateway %s is disabled", gw.Name)
	}

	factory, ok := gatewayRegistry[gw.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported gateway type: %s", gw.Type)
	}

	return factory(gw.GetConfig())
}

// GetDefaultGateway returns the default gateway for a type
func GetDefaultGateway(gwType string) (PaymentGatewayProvider, error) {
	gw, err := model.GetDefaultPaymentGateway(gwType)
	if err != nil {
		return nil, fmt.Errorf("no default gateway for type %s: %w", gwType, err)
	}
	return GetGateway(gw.Id)
}

// --- Unified Payment API ---

// CreatePaymentOrder creates a payment order using the specified gateway
func CreatePaymentOrder(c *gin.Context, userId int, gatewayId int, amount float64, currency string, description string) (*model.PaymentTransaction, string, error) {
	gw, err := GetGateway(gatewayId)
	if err != nil {
		return nil, "", err
	}

	orderNo, payURL, qrCode, err := gw.CreateOrder(c, userId, amount, currency, description)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create order: %w", err)
	}

	transaction := &model.PaymentTransaction{
		UserId:      userId,
		GatewayId:   gatewayId,
		GatewayType: gw.GetType(),
		OrderNo:     orderNo,
		Amount:      amount,
		Currency:    currency,
		Description: description,
		Status:      "pending",
	}
	if err := transaction.Insert(); err != nil {
		return nil, "", fmt.Errorf("failed to save transaction: %w", err)
	}

	return transaction, payURL + qrCode, nil
}

// ProcessPaymentWebhook processes a webhook notification for a specific gateway
func ProcessPaymentWebhook(c *gin.Context, gatewayId int, rawBody []byte) (*model.PaymentTransaction, error) {
	gw, err := GetGateway(gatewayId)
	if err != nil {
		return nil, err
	}

	tradeNo, orderNo, amount, currency, status, err := gw.HandleWebhook(c, rawBody)
	if err != nil {
		return nil, fmt.Errorf("webhook processing failed: %w", err)
	}

	// Find transaction
	transaction, err := model.GetPaymentTransactionByOrderNo(orderNo)
	if err != nil {
		return nil, fmt.Errorf("transaction not found for order %s: %w", orderNo, err)
	}

	// Update transaction
	transaction.TransactionNo = tradeNo
	transaction.Status = status
	transaction.RawNotification = string(rawBody)
	if status == "completed" {
		transaction.CompletedAt = time.Now().Unix()
	}
	if err := transaction.Update(); err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}

	// If payment completed, credit user quota
	if status == "completed" {
		if err := creditUserQuota(transaction.UserId, transaction.QuotaAmount); err != nil {
			common.SysLog(fmt.Sprintf("failed to credit quota for user %d: %v", transaction.UserId, err))
		}
	}

	return transaction, nil
}

// creditUserQuota credits quota to user after successful payment
func creditUserQuota(userId int, quotaAmount int64) error {
	if quotaAmount == 0 {
		// Calculate from amount using the system's QuotaPerUnit
		return nil
	}
	// Compensate quota
	err := model.IncreaseUserQuota(userId, int(quotaAmount), true)
	if err != nil {
		return err
	}
	model.RecordLog(userId, model.LogTypeTopup,
		fmt.Sprintf("支付成功，充值额度 %s", logger.LogQuota(int(quotaAmount))))
	return nil
}

// GetEnabledPaymentMethods returns all enabled payment methods with their gateways
func GetEnabledPaymentMethods() ([]map[string]interface{}, error) {
	gateways, err := model.GetEnabledPaymentGateways()
	if err != nil {
		return nil, err
	}

	methods := make([]map[string]interface{}, 0)
	for _, gw := range gateways {
		methods = append(methods, map[string]interface{}{
			"id":      gw.Id,
			"name":    gw.Name,
			"type":    gw.Type,
			"default": gw.IsDefault,
		})
	}
	return methods, nil
}
