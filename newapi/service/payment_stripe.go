package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// --- Stripe Adapter ---

func init() {
	// Register Stripe gateway factory
	RegisterGateway(model.GatewayTypeStripe, func(config map[string]interface{}) (PaymentGatewayProvider, error) {
		apiKey, _ := config["api_key"].(string)
		webhookSecret, _ := config["webhook_secret"].(string)

		if apiKey == "" {
			return nil, fmt.Errorf("stripe api_key is required")
		}

		return &StripeAdapter{
			apiKey:        apiKey,
			webhookSecret: webhookSecret,
			config:        config,
		}, nil
	})
}

// StripeAdapter implements PaymentGatewayProvider for Stripe
type StripeAdapter struct {
	apiKey        string
	webhookSecret string
	config        map[string]interface{}
}

func (s *StripeAdapter) GetType() string {
	return model.GatewayTypeStripe
}

func (s *StripeAdapter) GetName() string {
	return "Stripe"
}

func (s *StripeAdapter) CreateOrder(c *gin.Context, userId int, amount float64, currency string, description string) (string, string, string, error) {
	// Generate internal order number
	orderNo := fmt.Sprintf("STR-%s-%s", common.GetRandomString(8), common.GetRandomString(4))

	// In production, this would call the Stripe API (stripe-go SDK) to create a PaymentIntent
	// For now, we build the checkout session info that the frontend can use
	payURL := fmt.Sprintf("/api/payment/stripe/checkout/%s", orderNo)

	return orderNo, payURL, "", nil
}

func (s *StripeAdapter) HandleWebhook(c *gin.Context, rawBody []byte) (string, string, float64, string, string, error) {
	// Parse Stripe webhook event
	var event map[string]interface{}
	if err := common.Unmarshal(rawBody, &event); err != nil {
		return "", "", 0, "", "", fmt.Errorf("invalid stripe webhook payload: %w", err)
	}

	eventType, _ := event["type"].(string)
	data, _ := event["data"].(map[string]interface{})
	obj, _ := data["object"].(map[string]interface{})

	tradeNo, _ := obj["id"].(string)

	// Extract metadata (our internal order_no should be here)
	metadata, _ := obj["metadata"].(map[string]interface{})
	orderNo, _ := metadata["order_no"].(string)
	if orderNo == "" {
		return "", "", 0, "", "", fmt.Errorf("order_no not found in stripe metadata")
	}

	var status string
	var amount float64
	var currency string

	switch eventType {
	case "payment_intent.succeeded":
		status = "completed"
		if amt, ok := obj["amount"].(float64); ok {
			amount = amt / 100.0 // Stripe uses cents
		}
		if cur, ok := obj["currency"].(string); ok {
			currency = strings.ToUpper(cur)
		}
	case "payment_intent.payment_failed":
		status = "failed"
	case "payment_intent.canceled":
		status = "cancelled"
	default:
		status = "pending"
	}

	return tradeNo, orderNo, amount, currency, status, nil
}

func (s *StripeAdapter) VerifyOrder(orderNo string) (string, error) {
	// In production: call Stripe API to verify PaymentIntent status
	return "pending", nil
}

func (s *StripeAdapter) RefundOrder(orderNo string, amount float64, reason string) (string, error) {
	refundNo := fmt.Sprintf("REF-%s", common.GetRandomString(12))
	// In production: call Stripe Refunds API
	return refundNo, nil
}

// --- Generic / Mock Adapter factory for future implementations ---

// NewGenericPaymentAdapter creates a generic adapter for gateways without custom logic.
// Useful for gateways that follow standard callback patterns (Alipay, WeChat Pay).
func NewGenericPaymentAdapter(gwType string, config map[string]interface{}) PaymentGatewayProvider {
	return &genericPaymentAdapter{
		gwType: gwType,
		config: config,
	}
}

type genericPaymentAdapter struct {
	gwType string
	config map[string]interface{}
}

func (g *genericPaymentAdapter) GetType() string  { return g.gwType }
func (g *genericPaymentAdapter) GetName() string  { return g.gwType }

func (g *genericPaymentAdapter) CreateOrder(c *gin.Context, userId int, amount float64, currency string, description string) (string, string, string, error) {
	orderNo := fmt.Sprintf("%s-%s-%s", strings.ToUpper(g.gwType[:4]), common.GetRandomString(8), common.GetRandomString(4))
	payURL := fmt.Sprintf("/api/payment/%s/checkout/%s", g.gwType, orderNo)
	// Implement gateway-specific order creation here (e.g., Alipay SDK, WeChat Pay API)
	return orderNo, payURL, "", nil
}

func (g *genericPaymentAdapter) HandleWebhook(c *gin.Context, rawBody []byte) (string, string, float64, string, string, error) {
	// Generic webhook handler — to be customized per gateway
	var payload map[string]interface{}
	if err := common.Unmarshal(rawBody, &payload); err != nil {
		return "", "", 0, "", "", fmt.Errorf("invalid webhook payload: %w", err)
	}

	tradeNo, _ := payload["trade_no"].(string)
	orderNo, _ := payload["out_trade_no"].(string)
	status, _ := payload["status"].(string)
	amount, _ := payload["total_amount"].(float64)

	return tradeNo, orderNo, amount, "CNY", status, nil
}

func (g *genericPaymentAdapter) VerifyOrder(orderNo string) (string, error) {
	return "pending", nil
}

func (g *genericPaymentAdapter) RefundOrder(orderNo string, amount float64, reason string) (string, error) {
	return fmt.Sprintf("REF-%s", common.GetRandomString(12)), nil
}

// --- Helper: parse webhook JSON ---

func parseWebhookJSON(rawBody []byte) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(rawBody, &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return data, nil
}
