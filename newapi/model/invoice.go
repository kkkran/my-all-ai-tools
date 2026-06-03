package model

import (
	"time"

	"gorm.io/gorm"
)

// Invoice status constants
const (
	InvoiceStatusPending = "pending"
	InvoiceStatusPaid    = "paid"
	InvoiceStatusVoid    = "void"
	InvoiceStatusRefund  = "refund"
)

// Invoice represents a billing invoice
type Invoice struct {
	Id            int            `json:"id"`
	UserId        int            `json:"user_id" gorm:"index;not null"`
	WorkspaceId   int            `json:"workspace_id" gorm:"index;default:0"`
	InvoiceNo     string         `json:"invoice_no" gorm:"type:varchar(64);uniqueIndex;not null"`
	Amount        float64        `json:"amount" gorm:"type:decimal(12,2);not null"`
	Currency      string         `json:"currency" gorm:"type:varchar(8);default:'USD'"`
	TaxAmount     float64        `json:"tax_amount" gorm:"type:decimal(12,2);default:0"`
	TotalAmount   float64        `json:"total_amount" gorm:"type:decimal(12,2);not null"`
	Status        string         `json:"status" gorm:"type:varchar(16);default:'pending';index"`
	Items         string         `json:"items" gorm:"type:text"` // JSON: invoice line items
	PaymentMethod string         `json:"payment_method" gorm:"type:varchar(64)"`
	PaidAt        int64          `json:"paid_at" gorm:"bigint;default:0"`
	DueAt         int64          `json:"due_at" gorm:"bigint;default:0"`
	Notes         string         `json:"notes" gorm:"type:text"`
	CreatedAt     int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt     int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

// InvoiceItem represents a single line item in an invoice
type InvoiceItem struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
}

// --- Invoice CRUD ---

func (inv *Invoice) Insert() error {
	inv.CreatedAt = time.Now().Unix()
	inv.UpdatedAt = time.Now().Unix()
	return DB.Create(inv).Error
}

func (inv *Invoice) Update() error {
	inv.UpdatedAt = time.Now().Unix()
	return DB.Model(inv).Select("*").Updates(inv).Error
}

func GetInvoiceById(id int) (*Invoice, error) {
	inv := &Invoice{Id: id}
	err := DB.First(inv, "id = ?", id).Error
	return inv, err
}

func GetInvoiceByNo(invoiceNo string) (*Invoice, error) {
	inv := &Invoice{}
	err := DB.Where("invoice_no = ?", invoiceNo).First(inv).Error
	return inv, err
}

func GetUserInvoices(userId int, offset, limit int) ([]Invoice, int64, error) {
	var invoices []Invoice
	var total int64

	tx := DB.Begin()
	if err := tx.Model(&Invoice{}).Where("user_id = ?", userId).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Where("user_id = ?", userId).Order("id desc").Offset(offset).Limit(limit).Find(&invoices).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	return invoices, total, tx.Commit().Error
}

func GetWorkspaceInvoices(workspaceId int, offset, limit int) ([]Invoice, int64, error) {
	var invoices []Invoice
	var total int64

	tx := DB.Begin()
	if err := tx.Model(&Invoice{}).Where("workspace_id = ?", workspaceId).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Where("workspace_id = ?", workspaceId).Order("id desc").Offset(offset).Limit(limit).Find(&invoices).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	return invoices, total, tx.Commit().Error
}

// GenerateInvoiceNo generates a sequential invoice number
func GenerateInvoiceNo() string {
	return "INV-" + time.Now().Format("20060102") + "-" + common.GetRandomString(6)
}
