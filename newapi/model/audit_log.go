package model

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Audit action constants
const (
	AuditActionUserCreate    = "user.create"
	AuditActionUserUpdate    = "user.update"
	AuditActionUserDelete    = "user.delete"
	AuditActionUserDisable   = "user.disable"
	AuditActionTokenCreate   = "token.create"
	AuditActionTokenDelete   = "token.delete"
	AuditActionChannelCreate = "channel.create"
	AuditActionChannelUpdate = "channel.update"
	AuditActionChannelDelete = "channel.delete"
	AuditActionChannelEnable = "channel.enable"
	AuditActionChannelDisable = "channel.disable"
	AuditActionModelUpdate   = "model.update"
	AuditActionPaymentCreate = "payment.create"
	AuditActionPaymentRefund = "payment.refund"
	AuditActionSystemConfig  = "system.config"
	AuditActionWorkspaceCreate = "workspace.create"
	AuditActionWorkspaceUpdate = "workspace.update"
	AuditActionWorkspaceDelete = "workspace.delete"
	AuditActionMemberInvite    = "workspace.member.invite"
	AuditActionMemberRemove    = "workspace.member.remove"
)

// AuditLog records administrator and sensitive operations
type AuditLog struct {
	Id          int    `json:"id"`
	UserId      int    `json:"user_id" gorm:"index"`          // who performed the action
	Username    string `json:"username" gorm:"type:varchar(64);index;default:''"`
	Action      string `json:"action" gorm:"type:varchar(64);index;not null"` // audit action type
	Resource    string `json:"resource" gorm:"type:varchar(64);index"`        // affected resource type: user, token, channel, workspace
	ResourceId  int    `json:"resource_id" gorm:"default:0"`                  // affected resource id
	Detail      string `json:"detail" gorm:"type:text"`                       // JSON: before/after values or description
	IpAddress   string `json:"ip_address" gorm:"type:varchar(64);default:''"`
	UserAgent   string `json:"user_agent" gorm:"type:varchar(512);default:''"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;column:created_at;index"`
}

// --- AuditLog CRUD ---

func (al *AuditLog) Insert() error {
	al.CreatedAt = time.Now().Unix()
	return DB.Create(al).Error
}

// RecordAuditLog records an audit log entry. Non-blocking — logs errors but doesn't return them.
func RecordAuditLog(c *gin.Context, userId int, username string, action string, resource string, resourceId int, detail string) {
	entry := &AuditLog{
		UserId:     userId,
		Username:   username,
		Action:     action,
		Resource:   resource,
		ResourceId: resourceId,
		Detail:     detail,
	}
	if c != nil {
		entry.IpAddress = c.ClientIP()
		entry.UserAgent = c.Request.UserAgent()
	}
	if err := entry.Insert(); err != nil {
		// use fmt instead of logger to avoid circular imports
		common.SysLog("failed to record audit log: " + err.Error())
	}
}

func GetAuditLogs(userId int, action string, resource string, offset, limit int) ([]AuditLog, int64, error) {
	var logs []AuditLog
	var total int64

	query := DB.Model(&AuditLog{})
	if userId > 0 {
		query = query.Where("user_id = ?", userId)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	query.Count(&total)
	err := query.Order("id desc").Offset(offset).Limit(limit).Find(&logs).Error
	return logs, total, err
}

func GetAuditLogsByDateRange(startTime, endTime int64) ([]AuditLog, error) {
	var logs []AuditLog
	err := DB.Where("created_at BETWEEN ? AND ?", startTime, endTime).
		Order("id desc").Limit(1000).Find(&logs).Error
	return logs, err
}
