package service

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// CreateWorkspace creates a new workspace and adds the owner as a member
func CreateWorkspace(name, slug, description string, ownerId int) (*model.Workspace, error) {
	if slug != "" {
		// Check slug uniqueness
		existing, _ := model.GetWorkspaceBySlug(slug)
		if existing != nil {
			return nil, fmt.Errorf("workspace slug '%s' already exists", slug)
		}
	}

	ws := &model.Workspace{
		Name:        name,
		Slug:        slug,
		Description: description,
		OwnerId:     ownerId,
	}

	if err := ws.Insert(); err != nil {
		return nil, err
	}

	// Add owner as member with owner role
	member := &model.WorkspaceMember{
		WorkspaceId: ws.Id,
		UserId:      ownerId,
		Role:        model.WorkspaceRoleOwner,
	}
	if err := member.Insert(); err != nil {
		return nil, fmt.Errorf("workspace created but failed to add owner member: %w", err)
	}

	return ws, nil
}

// InviteMember creates an invitation to join a workspace
func InviteMember(workspaceId, inviterId int, email string, role string) (*model.WorkspaceInvitation, error) {
	// Validate inviter is admin or owner
	member, err := model.GetWorkspaceMember(workspaceId, inviterId)
	if err != nil {
		return nil, fmt.Errorf("inviter is not a workspace member")
	}
	if member.Role != model.WorkspaceRoleOwner && member.Role != model.WorkspaceRoleAdmin {
		return nil, fmt.Errorf("only owners and admins can invite members")
	}

	// Check if already a member
	if _, err := model.GetWorkspaceMember(workspaceId, 0); err == nil {
		// We need to check by email for user lookup
	}

	inv := &model.WorkspaceInvitation{
		WorkspaceId: workspaceId,
		InviterId:   inviterId,
		Email:       email,
		Role:        role,
		Status:      "pending",
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	if err := inv.Insert(); err != nil {
		return nil, err
	}

	return inv, nil
}

// AcceptInvitation accepts a workspace invitation
func AcceptInvitation(token string, userId int) (*model.WorkspaceMember, error) {
	inv, err := model.GetInvitationByToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid invitation token")
	}

	if inv.Status != "pending" {
		return nil, fmt.Errorf("invitation is %s", inv.Status)
	}

	if inv.ExpiresAt < time.Now().Unix() {
		inv.Status = "expired"
		inv.Update()
		return nil, fmt.Errorf("invitation has expired")
	}

	// Check if user is already a member
	existing, _ := model.GetWorkspaceMember(inv.WorkspaceId, userId)
	if existing != nil {
		inv.Status = "accepted"
		inv.Update()
		return existing, nil
	}

	// Add member
	member := &model.WorkspaceMember{
		WorkspaceId: inv.WorkspaceId,
		UserId:      userId,
		Role:        inv.Role,
	}
	if err := member.Insert(); err != nil {
		return nil, err
	}

	// Update invitation status
	inv.Status = "accepted"
	inv.Update()

	return member, nil
}

// RemoveMember removes a member from a workspace
func RemoveMember(workspaceId, removerId, targetUserId int) error {
	remover, err := model.GetWorkspaceMember(workspaceId, removerId)
	if err != nil {
		return fmt.Errorf("remover is not a member")
	}

	target, err := model.GetWorkspaceMember(workspaceId, targetUserId)
	if err != nil {
		return fmt.Errorf("target user is not a member")
	}

	// Can't remove the owner
	if target.Role == model.WorkspaceRoleOwner {
		return fmt.Errorf("cannot remove the workspace owner")
	}

	// Only owner and admin can remove members
	if remover.Role != model.WorkspaceRoleOwner && remover.Role != model.WorkspaceRoleAdmin {
		return fmt.Errorf("only owners and admins can remove members")
	}

	return target.Delete()
}

// UpdateMemberRole updates a member's role
func UpdateMemberRole(workspaceId, updaterId, targetUserId int, newRole string) error {
	updater, err := model.GetWorkspaceMember(workspaceId, updaterId)
	if err != nil {
		return fmt.Errorf("updater is not a member")
	}

	if updater.Role != model.WorkspaceRoleOwner {
		return fmt.Errorf("only the workspace owner can change member roles")
	}

	target, err := model.GetWorkspaceMember(workspaceId, targetUserId)
	if err != nil {
		return fmt.Errorf("target user is not a member")
	}

	// Prevent changing owner role
	if target.Role == model.WorkspaceRoleOwner {
		return fmt.Errorf("cannot change the owner's role")
	}

	// Prevent setting someone else as owner
	if newRole == model.WorkspaceRoleOwner {
		return fmt.Errorf("cannot transfer ownership via role update")
	}

	target.Role = newRole
	return target.Update()
}

// GetWorkspaceStats returns usage stats for a workspace
func GetWorkspaceStats(workspaceId int) (map[string]interface{}, error) {
	ws, err := model.GetWorkspaceById(workspaceId)
	if err != nil {
		return nil, err
	}

	memberCount, _ := model.GetWorkspaceMemberCount(workspaceId)

	stats := map[string]interface{}{
		"id":            ws.Id,
		"name":          ws.Name,
		"slug":          ws.Slug,
		"plan":          ws.Plan,
		"member_count":  memberCount,
		"created_at":    ws.CreatedAt,
		"settings":      ws.GetSettings(),
	}

	return stats, nil
}

// TransferOwnership transfers workspace ownership to another member
func TransferOwnership(workspaceId, currentOwnerId, newOwnerId int) error {
	// Verify current owner
	ws, err := model.GetWorkspaceById(workspaceId)
	if err != nil {
		return err
	}
	if ws.OwnerId != currentOwnerId {
		return fmt.Errorf("only the current owner can transfer ownership")
	}

	// Verify new owner is a member
	newOwnerMember, err := model.GetWorkspaceMember(workspaceId, newOwnerId)
	if err != nil {
		return fmt.Errorf("new owner is not a workspace member")
	}

	// Update workspace owner
	ws.OwnerId = newOwnerId
	if err := ws.Update(); err != nil {
		return err
	}

	// Downgrade previous owner to admin
	oldOwnerMember, _ := model.GetWorkspaceMember(workspaceId, currentOwnerId)
	if oldOwnerMember != nil {
		oldOwnerMember.Role = model.WorkspaceRoleAdmin
		oldOwnerMember.Update()
	}

	// Upgrade new owner
	newOwnerMember.Role = model.WorkspaceRoleOwner
	newOwnerMember.Update()

	return nil
}
