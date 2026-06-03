package model

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// MonitorMetricType defines metric categories
const (
	MetricTypeQPS          = "qps"
	MetricTypeTPS          = "tps"
	MetricTypeTokenThrough = "token_throughput"
	MetricTypeLatency      = "latency"
	MetricTypeErrorRate    = "error_rate"
	MetricTypeChannelSuccess = "channel_success_rate"
)

// MonitorMetric stores aggregated monitoring metrics per time window
type MonitorMetric struct {
	Id          int64   `json:"id"`
	MetricType  string  `json:"metric_type" gorm:"type:varchar(32);index:idx_metric_time;not null"`
	ChannelId   int     `json:"channel_id" gorm:"index;default:0"`
	ChannelType int     `json:"channel_type" gorm:"index;default:0"`
	ModelName   string  `json:"model_name" gorm:"type:varchar(128);index;default:''"`
	GroupName   string  `json:"group_name" gorm:"type:varchar(64);default:''"`
	Value       float64 `json:"value" gorm:"type:double;not null"`
	Count       int64   `json:"count" gorm:"default:0"`       // number of requests in this window
	TotalTokens int64   `json:"total_tokens" gorm:"default:0"` // total tokens consumed
	AvgLatency  float64 `json:"avg_latency" gorm:"default:0"`  // average latency in ms
	ErrorCount  int64   `json:"error_count" gorm:"default:0"`  // error count in this window
	WindowStart int64   `json:"window_start" gorm:"bigint;index:idx_metric_time;not null"` // unix timestamp
	WindowEnd   int64   `json:"window_end" gorm:"bigint;index;not null"`                    // unix timestamp
	CreatedAt   int64   `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

// MonitorAlertRule defines alerting rules
type MonitorAlertRule struct {
	Id             int    `json:"id"`
	Name           string `json:"name" gorm:"type:varchar(128);not null"`
	Description    string `json:"description" gorm:"type:text"`
	MetricType     string `json:"metric_type" gorm:"type:varchar(32);not null"`
	Condition      string `json:"condition" gorm:"type:varchar(32);not null"` // gt, lt, gte, lte, eq
	Threshold      float64 `json:"threshold" gorm:"type:double;not null"`
	Duration       int    `json:"duration" gorm:"default:300"` // evaluation window in seconds
	Enabled        bool   `json:"enabled" gorm:"default:1"`
	NotifyChannels string `json:"notify_channels" gorm:"type:text"` // JSON: notification channel configs
	LastTriggered  int64  `json:"last_triggered" gorm:"bigint;default:0"`
	CreatedAt      int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt      int64  `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
}

// MonitorAlertEvent records triggered alerts
type MonitorAlertEvent struct {
	Id          int     `json:"id"`
	RuleId      int     `json:"rule_id" gorm:"index;not null"`
	RuleName    string  `json:"rule_name" gorm:"type:varchar(128)"`
	MetricType  string  `json:"metric_type" gorm:"type:varchar(32)"`
	Value       float64 `json:"value" gorm:"type:double"`
	Threshold   float64 `json:"threshold" gorm:"type:double"`
	Message     string  `json:"message" gorm:"type:text"`
	Acknowledged bool   `json:"acknowledged" gorm:"default:0"`
	AcknowledgedBy int  `json:"acknowledged_by" gorm:"default:0"`
	CreatedAt   int64   `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
}

// --- MonitorMetric CRUD ---

func (m *MonitorMetric) Insert() error {
	m.CreatedAt = time.Now().Unix()
	return DB.Create(m).Error
}

func BatchInsertMetrics(metrics []MonitorMetric) error {
	if len(metrics) == 0 {
		return nil
	}
	return DB.CreateInBatches(metrics, 100).Error
}

func GetMetricsByType(metricType string, windowStart, windowEnd int64, channelId int, modelName string) ([]MonitorMetric, error) {
	var metrics []MonitorMetric
	query := DB.Where("metric_type = ? AND window_start >= ? AND window_end <= ?", metricType, windowStart, windowEnd)
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	if modelName != "" {
		query = query.Where("model_name = ?", modelName)
	}
	err := query.Order("window_start asc").Find(&metrics).Error
	return metrics, err
}

func GetAggregatedMetrics(metricType string, windowStart, windowEnd int64) ([]MonitorMetric, error) {
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, err
	}

	var metrics []MonitorMetric
	// Use DB-agnostic aggregation
	if common.UsingPostgreSQL {
		err = DB.Raw(`SELECT metric_type, channel_type, model_name,
			SUM(value) as value, SUM(count) as count, SUM(total_tokens) as total_tokens,
			AVG(avg_latency) as avg_latency, SUM(error_count) as error_count,
			MIN(window_start) as window_start, MAX(window_end) as window_end
			FROM monitor_metrics
			WHERE metric_type = $1 AND window_start >= $2 AND window_end <= $3
			GROUP BY metric_type, channel_type, model_name
			ORDER BY window_start ASC`, metricType, windowStart, windowEnd).Scan(&metrics).Error
	} else {
		err = DB.Raw("SELECT metric_type, channel_type, model_name, "+
			"SUM(value) as value, SUM(count) as count, SUM(total_tokens) as total_tokens, "+
			"AVG(avg_latency) as avg_latency, SUM(error_count) as error_count, "+
			"MIN(window_start) as window_start, MAX(window_end) as window_end "+
			"FROM monitor_metrics "+
			"WHERE metric_type = ? AND window_start >= ? AND window_end <= ? "+
			"GROUP BY metric_type, channel_type, model_name "+
			"ORDER BY window_start ASC", metricType, windowStart, windowEnd).Scan(&metrics).Error
	}
	// suppress unused variable warning
	_ = sqlDB
	return metrics, err
}

func CleanupOldMetrics(beforeTime int64) error {
	return DB.Where("window_end < ?", beforeTime).Delete(&MonitorMetric{}).Error
}

// --- MonitorAlertRule CRUD ---

func (r *MonitorAlertRule) Insert() error {
	r.CreatedAt = time.Now().Unix()
	r.UpdatedAt = time.Now().Unix()
	return DB.Create(r).Error
}

func (r *MonitorAlertRule) Update() error {
	r.UpdatedAt = time.Now().Unix()
	return DB.Model(r).Select("*").Updates(r).Error
}

func (r *MonitorAlertRule) Delete() error {
	return DB.Delete(r).Error
}

func GetEnabledAlertRules(metricType string) ([]MonitorAlertRule, error) {
	var rules []MonitorAlertRule
	query := DB.Where("enabled = 1")
	if metricType != "" {
		query = query.Where("metric_type = ?", metricType)
	}
	err := query.Find(&rules).Error
	return rules, err
}

func GetAllAlertRules() ([]MonitorAlertRule, error) {
	var rules []MonitorAlertRule
	err := DB.Order("id desc").Find(&rules).Error
	return rules, err
}

// --- MonitorAlertEvent CRUD ---

func (e *MonitorAlertEvent) Insert() error {
	e.CreatedAt = time.Now().Unix()
	return DB.Create(e).Error
}

func GetAlertEvents(ruleId int, acknowledged bool, offset, limit int) ([]MonitorAlertEvent, int64, error) {
	var events []MonitorAlertEvent
	var total int64

	query := DB.Model(&MonitorAlertEvent{})
	if ruleId > 0 {
		query = query.Where("rule_id = ?", ruleId)
	}
	if !acknowledged {
		query = query.Where("acknowledged = 0")
	}
	query.Count(&total)
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&events).Error
	return events, total, err
}

func AcknowledgeAlertEvent(eventId int, userId int) error {
	return DB.Model(&MonitorAlertEvent{}).Where("id = ?", eventId).
		Updates(map[string]interface{}{
			"acknowledged":    true,
			"acknowledged_by": userId,
		}).Error
}
