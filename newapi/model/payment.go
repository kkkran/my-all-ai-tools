package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// PaymentGatewayType constants
const (
	GatewayTypeStripe    = "stripe"
	GatewayTypePayPal    = "paypal"
	GatewayTypeAlipay    = "alipay"
	GatewayTypeWeChatPay = "wechat_pay"
	GatewayTypeEpay      = "epay"
	GatewayTypeCreem     = "creem"
)

// PaymentGateway represents a configured payment gateway
type PaymentGateway struct {
	Id        int    `json:"id"`
	Name      string `json:"name" gorm:"type:varchar(64);not null"`
	Type      string `json:"type" gorm:"type:varchar(32);not null;index"` // stripe, paypal, alipay, wechat_pay
	Enabled   bool   `json:"enabled" gorm:"default:1"`
	IsDefault bool   `json:"is_default" gorm:"default:0"`
	Config    string `json:"config" gorm:"type:text"` // JSON: API keys, secrets, webhook secrets
	SortOrder int    `json:"sort_order" gorm:"default:0"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

// PaymentTransaction represents a payment transaction record
type PaymentTransaction struct {
	Id              int     `json:"id"`
	UserId          int     `json:"user_id" gorm:"index;not null"`
	GatewayId       int     `json:"gateway_id"`
	GatewayType     string  `json:"gateway_type" gorm:"type:varchar(32);index"`
	TransactionNo   string  `json:"transaction_no" gorm:"type:varchar(255);uniqueIndex"` // external transaction ID
	OrderNo         string  `json:"order_no" gorm:"type:varchar(128);index"`              // internal order number
	Amount          float64 `json:"amount" gorm:"type:decimal(12,2);not null"`            // amount in original currency
	Currency        string  `json:"currency" gorm:"type:varchar(8);default:'USD'"`
	QuotaAmount     int64   `json:"quota_amount" gorm:"default:0"` // converted quota
	Description     string  `json:"description" gorm:"type:varchar(512)"`
	Status          string  `json:"status" gorm:"type:varchar(24);default:'pending';index"` // pending, completed, failed, refunded, cancelled
	PaymentMethod   string  `json:"payment_method" gorm:"type:varchar(64)"`                 // card, alipay, wechat, etc.
	RawNotification string  `json:"raw_notification" gorm:"type:text"`                      // raw webhook body for audit
	CompletedAt     int64   `json:"completed_at" gorm:"bigint;default:0"`
	CreatedAt       int64   `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt       int64   `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

// --- PaymentGateway CRUD ---

func (g *PaymentGateway) Insert() error {
	g.CreatedAt = time.Now().Unix()
	g.UpdatedAt = time.Now().Unix()
	// If set as default, unset other defaults of same type
	if g.IsDefault {
		DB.Model(&PaymentGateway{}).Where("type = ? AND is_default = 1", g.Type).Update("is_default", false)
	}
	return DB.Create(g).Error
}

func (g *PaymentGateway) Update() error {
	g.UpdatedAt = time.Now().Unix()
	if g.IsDefault {
		DB.Model(&PaymentGateway{}).Where("type = ? AND is_default = 1 AND id != ?", g.Type, g.Id).Update("is_default", false)
	}
	return DB.Model(g).Select("*").Updates(g).Error
}

func (g *PaymentGateway) Delete() error {
	return DB.Delete(g).Error
}

func GetPaymentGatewayById(id int) (*PaymentGateway, error) {
	gw := &PaymentGateway{Id: id}
	err := DB.First(gw, "id = ?", id).Error
	return gw, err
}

func GetEnabledPaymentGateways() ([]PaymentGateway, error) {
	var gateways []PaymentGateway
	err := DB.Where("enabled = 1").Order("sort_order asc").Find(&gateways).Error
	return gateways, err
}

func GetPaymentGatewaysByType(gwType string) ([]PaymentGateway, error) {
	var gateways []PaymentGateway
	err := DB.Where("type = ? AND enabled = 1", gwType).Order("sort_order asc").Find(&gateways).Error
	return gateways, err
}

func GetDefaultPaymentGateway(gwType string) (*PaymentGateway, error) {
	gw := &PaymentGateway{}
	err := DB.Where("type = ? AND enabled = 1 AND is_default = 1", gwType).First(gw).Error
	return gw, err
}

// --- PaymentTransaction CRUD ---

func (pt *PaymentTransaction) Insert() error {
	pt.CreatedAt = time.Now().Unix()
	pt.UpdatedAt = time.Now().Unix()
	return DB.Create(pt).Error
}

func (pt *PaymentTransaction) Update() error {
	pt.UpdatedAt = time.Now().Unix()
	return DB.Model(pt).Select("*").Updates(pt).Error
}

func GetPaymentTransactionByOrderNo(orderNo string) (*PaymentTransaction, error) {
	pt := &PaymentTransaction{}
	err := DB.Where("order_no = ?", orderNo).First(pt).Error
	return pt, err
}

func GetPaymentTransactionByTradeNo(tradeNo string) (*PaymentTransaction, error) {
	pt := &PaymentTransaction{}
	err := DB.Where("transaction_no = ?", tradeNo).First(pt).Error
	return pt, err
}

func GetUserPaymentTransactions(userId int, offset, limit int) ([]PaymentTransaction, int64, error) {
	var transactions []PaymentTransaction
	var total int64

	tx := DB.Begin()
	if err := tx.Model(&PaymentTransaction{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Where("user_id = ?", userId).Order("id desc").Offset(offset).Limit(limit).Find(&transactions).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	return transactions, total, tx.Commit().Error
}

// GetGatewayConfig returns the parsed config map for this gateway
func (g *PaymentGateway) GetConfig() map[string]interface{} {
	config := make(map[string]interface{})
	if g.Config != "" {
		common.Unmarshal([]byte(g.Config), &config)
	}
	return config
}

// SetConfig sets the config as JSON
func (g *PaymentGateway) SetConfig(config map[string]interface{}) {
	data, _ := common.Marshal(config)
	g.Config = string(data)
}
