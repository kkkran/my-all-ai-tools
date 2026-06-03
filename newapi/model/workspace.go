package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Workspace roles
const (
	WorkspaceRoleOwner  = "owner"
	WorkspaceRoleAdmin  = "admin"
	WorkspaceRoleMember = "member"
	WorkspaceRoleViewer = "viewer"
)

// Workspace represents an enterprise/organization workspace
type Workspace struct {
	Id          int            `json:"id"`
	Name        string         `json:"name" gorm:"type:varchar(128);not null;index"`
	Slug        string         `json:"slug" gorm:"type:varchar(64);uniqueIndex;not null"`
	Description string         `json:"description" gorm:"type:text"`
	Logo        string         `json:"logo" gorm:"type:varchar(512);default:''"`
	OwnerId     int            `json:"owner_id" gorm:"index;not null"`
	Status      int            `json:"status" gorm:"type:int;default:1"` // 1=active, 0=suspended
	Plan        string         `json:"plan" gorm:"type:varchar(64);default:'free'"` // free, pro, enterprise
	Settings    string         `json:"settings" gorm:"type:text"`                  // JSON encoded workspace settings
	CreatedAt   int64          `json:"created_at" gorm:"autoCreateTime;column:created_at"`
	UpdatedAt   int64          `json:"updated_at" gorm:"autoUpdateTime;column:updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

// WorkspaceMember represents a member of a workspace
type WorkspaceMember struct {
	Id          int    `json:"id"`
	WorkspaceId int    `json:"workspace_id" gorm:"uniqueIndex:idx_ws_user;not null"`
	UserId      int    `json:"user_id" gorm:"uniqueIndex:idx_ws_user;not null"`
	Role        string `json:"role" gorm:"type:varchar(16);default:'member'"`
	DisplayName string `json:"display_name" gorm:"type:varchar(64);default:''"`
	JoinedAt    int64  `json:"joined_at" gorm:"autoCreateTime;column:joined_at"`
}

// WorkspaceInvitation represents a pending invitation to a workspace
type WorkspaceInvitation struct {
	Id          int    `json:"id"`
	WorkspaceId int    `json:"workspace_id" gorm:"index;not null"`
	InviterId   int    `json:"inviter_id" gorm:"not null"`
	InviteeId   int    `json:"invitee_id" gorm:"index"`
	Email       string `json:"email" gorm:"type:varchar(128);index"`
	Role        string `json:"role" gorm:"type:varchar(16);default:'member'"`
	Token       string `json:"token" gorm:"type:varchar(64);uniqueIndex"`
	Status      string `json:"status" gorm:"type:varchar(16);default:'pending'"` // pending, accepted, declined, expired
	ExpiresAt   int64  `json:"expires_at" gorm:"bigint"`
	CreatedAt   int64  `json:"created_at" gorm:"autoCreateTime;column:created_at"`
}

// --- Workspace CRUD ---

func (ws *Workspace) Insert() error {
	if ws.Slug == "" {
		ws.Slug = fmt.Sprintf("ws-%s", common.GetRandomString(8))
	}
	ws.Status = 1
	ws.CreatedAt = time.Now().Unix()
	ws.UpdatedAt = time.Now().Unix()
	return DB.Create(ws).Error
}

func (ws *Workspace) Update() error {
	ws.UpdatedAt = time.Now().Unix()
	return DB.Model(ws).Select("*").Updates(ws).Error
}

func (ws *Workspace) Delete() error {
	// Soft delete workspace and all related records
	tx := DB.Begin()
	if err := tx.Where("workspace_id = ?", ws.Id).Delete(&WorkspaceMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Where("workspace_id = ?", ws.Id).Delete(&WorkspaceInvitation{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(ws).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func GetWorkspaceById(id int) (*Workspace, error) {
	if id == 0 {
		return nil, errors.New("workspace id is empty")
	}
	ws := &Workspace{Id: id}
	err := DB.First(ws, "id = ?", id).Error
	return ws, err
}

func GetWorkspaceBySlug(slug string) (*Workspace, error) {
	if slug == "" {
		return nil, errors.New("workspace slug is empty")
	}
	ws := &Workspace{}
	err := DB.Where("slug = ?", slug).First(ws).Error
	return ws, err
}

func GetUserWorkspaces(userId int) ([]Workspace, error) {
	var workspaces []Workspace
	err := DB.Table("workspaces").
		Joins("JOIN workspace_members ON workspace_members.workspace_id = workspaces.id").
		Where("workspace_members.user_id = ? AND workspaces.deleted_at IS NULL", userId).
		Find(&workspaces).Error
	return workspaces, err
}

func IsWorkspaceOwner(workspaceId, userId int) bool {
	ws, err := GetWorkspaceById(workspaceId)
	if err != nil {
		return false
	}
	return ws.OwnerId == userId
}

// --- WorkspaceMember CRUD ---

func (m *WorkspaceMember) Insert() error {
	m.JoinedAt = time.Now().Unix()
	return DB.Create(m).Error
}

func (m *WorkspaceMember) Update() error {
	return DB.Model(m).Select("role", "display_name").Updates(m).Error
}

func (m *WorkspaceMember) Delete() error {
	return DB.Delete(m).Error
}

func GetWorkspaceMember(workspaceId, userId int) (*WorkspaceMember, error) {
	member := &WorkspaceMember{}
	err := DB.Where("workspace_id = ? AND user_id = ?", workspaceId, userId).First(member).Error
	return member, err
}

func GetWorkspaceMembers(workspaceId int, offset, limit int) ([]WorkspaceMember, int64, error) {
	var members []WorkspaceMember
	var total int64

	tx := DB.Begin()
	if err := tx.Model(&WorkspaceMember{}).Where("workspace_id = ?", workspaceId).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	if err := tx.Where("workspace_id = ?", workspaceId).Order("joined_at asc").Offset(offset).Limit(limit).Find(&members).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}
	return members, total, tx.Commit().Error
}

func GetUserWorkspaceRole(workspaceId, userId int) string {
	member, err := GetWorkspaceMember(workspaceId, userId)
	if err != nil {
		return ""
	}
	return member.Role
}

func GetWorkspaceMemberCount(workspaceId int) (int64, error) {
	var count int64
	err := DB.Model(&WorkspaceMember{}).Where("workspace_id = ?", workspaceId).Count(&count).Error
	return count, err
}

// --- WorkspaceInvitation CRUD ---

func (inv *WorkspaceInvitation) Insert() error {
	if inv.Token == "" {
		inv.Token = common.GetUUID()
	}
	inv.CreatedAt = time.Now().Unix()
	if inv.ExpiresAt == 0 {
		inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour).Unix() // 7 days default
	}
	return DB.Create(inv).Error
}

func (inv *WorkspaceInvitation) Update() error {
	return DB.Model(inv).Select("status").Updates(inv).Error
}

func GetInvitationByToken(token string) (*WorkspaceInvitation, error) {
	inv := &WorkspaceInvitation{}
	err := DB.Where("token = ?", token).First(inv).Error
	return inv, err
}

func GetPendingInvitationsByWorkspace(workspaceId int) ([]WorkspaceInvitation, error) {
	var invitations []WorkspaceInvitation
	err := DB.Where("workspace_id = ? AND status = 'pending'", workspaceId).
		Order("created_at desc").Find(&invitations).Error
	return invitations, err
}

func GetPendingInvitationsByEmail(email string) ([]WorkspaceInvitation, error) {
	var invitations []WorkspaceInvitation
	err := DB.Where("email = ? AND status = 'pending'", email).
		Order("created_at desc").Find(&invitations).Error
	return invitations, err
}

// --- Workspace Settings ---

func (ws *Workspace) GetSettings() map[string]interface{} {
	settings := make(map[string]interface{})
	if ws.Settings != "" {
		common.Unmarshal([]byte(ws.Settings), &settings)
	}
	return settings
}

func (ws *Workspace) SetSettings(settings map[string]interface{}) {
	data, err := common.Marshal(settings)
	if err != nil {
		common.SysLog("failed to marshal workspace settings: " + err.Error())
		return
	}
	ws.Settings = string(data)
}
