package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// ModelCatalog represents a model listing in the model marketplace
type ModelCatalog struct {
	Id              int            `json:"id"`
	ModelName       string         `json:"model_name" gorm:"type:varchar(128);uniqueIndex;not null"`
	DisplayName     string         `json:"display_name" gorm:"type:varchar(128);not null"`
	Provider        string         `json:"provider" gorm:"type:varchar(64);index"`  // OpenAI, Anthropic, Google, DeepSeek, etc.
	Description     string         `json:"description" gorm:"type:text"`
	LongDescription string         `json:"long_description" gorm:"type:text"`       // Markdown detailed description
	Icon            string         `json:"icon" gorm:"type:varchar(512);default:''"` // Icon URL
	Category        string         `json:"category" gorm:"type:varchar(32);index;default:'chat'"` // chat, image, audio, embedding, video, rerank
	Tags            string         `json:"tags" gorm:"type:varchar(512);default:''"` // Comma-separated tags
	PricingInfo     string         `json:"pricing_info" gorm:"type:text"`            // JSON: pricing details for display
	Capabilities    string         `json:"capabilities" gorm:"type:text"`            // JSON: [context_window, max_output_tokens, modalities...]
	DocURL          string         `json:"doc_url" gorm:"type:varchar(512);default:''"`
	Status          int            `json:"status" gorm:"type:int;default:1"` // 1=published, 0=draft
	Featured        bool           `json:"featured" gorm:"default:0"`
	SortOrder       int            `json:"sort_order" gorm:"default:0"`
	CreatedAt       int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt       int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

// ModelTag for model tagging (standalone table for search/aggregation)
type ModelTag struct {
	Id        int    `json:"id"`
	Name      string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
	Color     string `json:"color" gorm:"type:varchar(16);default:'#6366f1'"`
	Usage     int    `json:"usage" gorm:"default:0"` // number of models using this tag
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

// ModelCategory defines categories for model organization
type ModelCategory struct {
	Id          int    `json:"id"`
	Name        string `json:"name" gorm:"type:varchar(64);uniqueIndex;not null"`
	DisplayName string `json:"display_name" gorm:"type:varchar(128);not null"`
	Description string `json:"description" gorm:"type:text"`
	Icon        string `json:"icon" gorm:"type:varchar(512);default:''"`
	SortOrder   int    `json:"sort_order" gorm:"default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

// --- ModelCatalog CRUD ---

func (mc *ModelCatalog) Insert() error {
	mc.CreatedAt = time.Now().Unix()
	mc.UpdatedAt = time.Now().Unix()
	return DB.Create(mc).Error
}

func (mc *ModelCatalog) Update() error {
	mc.UpdatedAt = time.Now().Unix()
	return DB.Model(mc).Select("*").Updates(mc).Error
}

func (mc *ModelCatalog) Delete() error {
	return DB.Delete(mc).Error
}

func GetModelCatalogById(id int) (*ModelCatalog, error) {
	mc := &ModelCatalog{Id: id}
	err := DB.First(mc, "id = ?", id).Error
	return mc, err
}

func GetModelCatalogByName(modelName string) (*ModelCatalog, error) {
	mc := &ModelCatalog{}
	err := DB.Where("model_name = ?", modelName).First(mc).Error
	return mc, err
}

func GetPublishedModels(category string, offset, limit int) ([]ModelCatalog, int64, error) {
	var models []ModelCatalog
	var total int64

	query := DB.Model(&ModelCatalog{}).Where("status = 1")
	if category != "" {
		query = query.Where("category = ?", category)
	}
	query.Count(&total)
	err := query.Order("featured desc, sort_order asc, id asc").Offset(offset).Limit(limit).Find(&models).Error
	return models, total, err
}

func SearchModelCatalog(keyword string) ([]ModelCatalog, error) {
	var models []ModelCatalog
	pattern := "%" + keyword + "%"
	err := DB.Where("status = 1 AND (model_name LIKE ? OR display_name LIKE ? OR description LIKE ? OR provider LIKE ? OR tags LIKE ?)",
		pattern, pattern, pattern, pattern, pattern).
		Order("featured desc, sort_order asc").Limit(50).Find(&models).Error
	return models, err
}

func GetModelsByProvider(provider string) ([]ModelCatalog, error) {
	var models []ModelCatalog
	err := DB.Where("provider = ? AND status = 1", provider).Order("sort_order asc").Find(&models).Error
	return models, err
}

func GetFeaturedModels() ([]ModelCatalog, error) {
	var models []ModelCatalog
	err := DB.Where("featured = 1 AND status = 1").Order("sort_order asc").Limit(20).Find(&models).Error
	return models, err
}

// GetPricingInfo returns parsed pricing information
func (mc *ModelCatalog) GetPricingInfo() map[string]interface{} {
	info := make(map[string]interface{})
	if mc.PricingInfo != "" {
		common.Unmarshal([]byte(mc.PricingInfo), &info)
	}
	return info
}

// GetCapabilities returns parsed capabilities
func (mc *ModelCatalog) GetCapabilities() map[string]interface{} {
	caps := make(map[string]interface{})
	if mc.Capabilities != "" {
		common.Unmarshal([]byte(mc.Capabilities), &caps)
	}
	return caps
}

// --- ModelCategory CRUD ---

func (cat *ModelCategory) Insert() error {
	cat.CreatedAt = time.Now().Unix()
	return DB.Create(cat).Error
}

func (cat *ModelCategory) Update() error {
	return DB.Model(cat).Select("*").Updates(cat).Error
}

func GetModelCategories() ([]ModelCategory, error) {
	var categories []ModelCategory
	err := DB.Order("sort_order asc").Find(&categories).Error
	return categories, err
}

// --- ModelTag CRUD ---

func (tag *ModelTag) Insert() error {
	tag.CreatedAt = time.Now().Unix()
	return DB.Create(tag).Error
}

func (tag *ModelTag) Update() error {
	return DB.Model(tag).Select("*").Updates(tag).Error
}

func GetAllModelTags() ([]ModelTag, error) {
	var tags []ModelTag
	err := DB.Order("usage desc").Find(&tags).Error
	return tags, err
}

func IncrementTagUsage(tagName string) error {
	return DB.Model(&ModelTag{}).Where("name = ?", tagName).
		Update("usage", gorm.Expr("usage + 1")).Error
}
