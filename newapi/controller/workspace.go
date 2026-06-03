package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// --- Workspace CRUD ---

// ListWorkspaces returns all workspaces for the current user
func ListWorkspaces(c *gin.Context) {
	userId := c.GetInt("id")

	workspaces, err := model.GetUserWorkspaces(userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": workspaces})
}

// GetWorkspace returns a single workspace by ID
func GetWorkspace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	ws, err := model.GetWorkspaceById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "workspace not found"})
		return
	}

	// Get member count
	count, _ := model.GetWorkspaceMemberCount(id)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"workspace":    ws,
			"member_count": count,
		},
	})
}

// CreateWorkspace creates a new workspace
func CreateWorkspace(c *gin.Context) {
	userId := c.GetInt("id")

	var req struct {
		Name        string `json:"name" binding:"required"`
		Slug        string `json:"slug"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	ws, err := service.CreateWorkspace(req.Name, req.Slug, req.Description, userId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	// Audit log
	detailBytes, _ := common.Marshal(gin.H{"name": ws.Name, "slug": ws.Slug})
	model.RecordAuditLog(c, userId, "", model.AuditActionWorkspaceCreate, "workspace", ws.Id, string(detailBytes))

	c.JSON(http.StatusOK, gin.H{"success": true, "data": ws})
}

// UpdateWorkspace updates workspace info
func UpdateWorkspace(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	// Check membership and role
	role := model.GetUserWorkspaceRole(id, userId)
	if role != model.WorkspaceRoleOwner && role != model.WorkspaceRoleAdmin {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "insufficient permissions"})
		return
	}

	ws, err := model.GetWorkspaceById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "workspace not found"})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Logo        *string `json:"logo"`
		Plan        *string `json:"plan"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.Name != nil {
		ws.Name = *req.Name
	}
	if req.Description != nil {
		ws.Description = *req.Description
	}
	if req.Logo != nil {
		ws.Logo = *req.Logo
	}
	if req.Plan != nil {
		ws.Plan = *req.Plan
	}

	if err := ws.Update(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	model.RecordAuditLog(c, userId, "", model.AuditActionWorkspaceUpdate, "workspace", ws.Id, "updated workspace info")

	c.JSON(http.StatusOK, gin.H{"success": true, "data": ws})
}

// DeleteWorkspace deletes a workspace (owner only)
func DeleteWorkspace(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	if !model.IsWorkspaceOwner(id, userId) {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "only the owner can delete a workspace"})
		return
	}

	ws, err := model.GetWorkspaceById(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "workspace not found"})
		return
	}

	if err := ws.Delete(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	model.RecordAuditLog(c, userId, "", model.AuditActionWorkspaceDelete, "workspace", id, "deleted workspace")

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "workspace deleted"})
}

// --- Member Management ---

// GetWorkspaceMembers returns members of a workspace
func GetWorkspaceMembers(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	role := model.GetUserWorkspaceRole(id, userId)
	if role == "" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "not a workspace member"})
		return
	}

	pageInfo := common.GetPageInfo(c)
	members, total, err := model.GetWorkspaceMembers(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    members,
		"total":   total,
	})
}

// InviteWorkspaceMember invites a user to a workspace
func InviteWorkspaceMember(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	workspaceId, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required"`
		Role  string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.Role == "" {
		req.Role = model.WorkspaceRoleMember
	}

	inv, err := service.InviteMember(workspaceId, userId, req.Email, req.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	model.RecordAuditLog(c, userId, "", model.AuditActionMemberInvite, "workspace", workspaceId,
		"invited "+req.Email+" as "+req.Role)

	c.JSON(http.StatusOK, gin.H{"success": true, "data": inv})
}

// AcceptWorkspaceInvitation accepts an invitation
func AcceptWorkspaceInvitation(c *gin.Context) {
	userId := c.GetInt("id")

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	member, err := service.AcceptInvitation(req.Token, userId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": member})
}

// RemoveWorkspaceMember removes a member
func RemoveWorkspaceMember(c *gin.Context) {
	userId := c.GetInt("id")
	wsIdStr := c.Param("id")
	memberIdStr := c.Param("memberId")

	workspaceId, err := strconv.Atoi(wsIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	targetUserId, err := strconv.Atoi(memberIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid member id"})
		return
	}

	if err := service.RemoveMember(workspaceId, userId, targetUserId); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	model.RecordAuditLog(c, userId, "", model.AuditActionMemberRemove, "workspace", workspaceId,
		"removed member "+memberIdStr)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "member removed"})
}

// UpdateMemberRole updates a member's role
func UpdateMemberRole(c *gin.Context) {
	userId := c.GetInt("id")
	wsIdStr := c.Param("id")
	memberIdStr := c.Param("memberId")

	workspaceId, err := strconv.Atoi(wsIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	targetUserId, err := strconv.Atoi(memberIdStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid member id"})
		return
	}

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := service.UpdateMemberRole(workspaceId, userId, targetUserId, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "role updated"})
}

// GetWorkspaceStats returns workspace usage statistics
func GetWorkspaceStats(c *gin.Context) {
	userId := c.GetInt("id")
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	role := model.GetUserWorkspaceRole(id, userId)
	if role == "" {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "not a workspace member"})
		return
	}

	stats, err := service.GetWorkspaceStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": stats})
}

// GetWorkspaceInvitations returns pending invitations for a workspace
func GetWorkspaceInvitations(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid workspace id"})
		return
	}

	invitations, err := model.GetPendingInvitationsByWorkspace(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": invitations})
}
