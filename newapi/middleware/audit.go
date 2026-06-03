package middleware

import (
	"bytes"
	"io"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// AuditMiddleware logs mutation operations to audit log.
// Only intercepts POST/PUT/PATCH/DELETE requests on admin routes.
func AuditMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip non-mutation methods
		method := c.Request.Method
		if method != "POST" && method != "PUT" && method != "PATCH" && method != "DELETE" {
			c.Next()
			return
		}

		// Skip non-admin paths
		path := c.Request.URL.Path
		if !strings.Contains(path, "/admin/") &&
			!strings.Contains(path, "/token/") &&
			!strings.Contains(path, "/workspaces/") &&
			!strings.Contains(path, "/payment/") {
			c.Next()
			return
		}

		// Read request body for audit
		var bodyBytes []byte
		if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
			// Restore the body for downstream handlers
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		// Determine action from path
		action := pathToAuditAction(path, method)

		// Determine resource
		resource := pathToResource(path)

		// Extract resource ID from path
		resourceId := extractResourceId(path)

		userId := c.GetInt("id")
		username, _ := c.Get("username")

		usernameStr := ""
		if username != nil {
			usernameStr = username.(string)
		}

		// Record audit after request completes
		c.Next()

		// Only log if successful (2xx)
		if c.Writer.Status() >= 200 && c.Writer.Status() < 300 && action != "" {
			detail := ""
			if len(bodyBytes) > 0 && len(bodyBytes) < 4096 {
				detail = string(bodyBytes)
			}

			model.RecordAuditLog(c, userId, usernameStr, action, resource, resourceId, detail)
		}
	}
}

// pathToAuditAction maps a URL path + method to an audit action
func pathToAuditAction(path string, method string) string {
	switch {
	case strings.Contains(path, "/admin/payment/gateways") && method == "POST":
		return model.AuditActionPaymentCreate
	case strings.Contains(path, "/admin/payment/gateways") && method == "PUT":
		return model.AuditActionPaymentCreate
	case strings.Contains(path, "/admin/payment/gateways") && method == "DELETE":
		return model.AuditActionPaymentCreate

	case strings.Contains(path, "/workspaces") && strings.Contains(path, "/members") && method == "DELETE":
		return model.AuditActionMemberRemove
	case strings.Contains(path, "/workspaces") && strings.Contains(path, "/invite"):
		return model.AuditActionMemberInvite
	case strings.Contains(path, "/workspaces") && method == "POST":
		return model.AuditActionWorkspaceCreate
	case strings.Contains(path, "/workspaces") && method == "PUT":
		return model.AuditActionWorkspaceUpdate
	case strings.Contains(path, "/workspaces") && method == "DELETE":
		return model.AuditActionWorkspaceDelete

	case strings.Contains(path, "/token/") && strings.Contains(path, "/rotate"):
		return model.AuditActionTokenCreate
	case strings.Contains(path, "/token/") && method == "DELETE":
		return model.AuditActionTokenDelete

	case strings.Contains(path, "/admin/model-catalog") && method == "POST":
		return model.AuditActionModelUpdate
	case strings.Contains(path, "/admin/model-catalog") && method == "PUT":
		return model.AuditActionModelUpdate
	case strings.Contains(path, "/admin/model-catalog") && method == "DELETE":
		return model.AuditActionModelUpdate

	case strings.Contains(path, "/admin/") && strings.Contains(path, "/user"):
		return model.AuditActionUserUpdate
	case strings.Contains(path, "/admin/") && strings.Contains(path, "/channel"):
		return model.AuditActionChannelUpdate

	default:
		return ""
	}
}

// pathToResource extracts the resource type from the path
func pathToResource(path string) string {
	switch {
	case strings.Contains(path, "/payment"):
		return "payment"
	case strings.Contains(path, "/workspaces"):
		return "workspace"
	case strings.Contains(path, "/token"):
		return "token"
	case strings.Contains(path, "/admin/model-catalog"):
		return "model"
	case strings.Contains(path, "/admin/") && strings.Contains(path, "/user"):
		return "user"
	case strings.Contains(path, "/admin/") && strings.Contains(path, "/channel"):
		return "channel"
	default:
		return "system"
	}
}

// extractResourceId tries to extract a numeric ID from the path
func extractResourceId(path string) int {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	// Look for numeric segments
	for _, seg := range segments {
		if len(seg) > 0 && isNumeric(seg) {
			var id int
			for _, c := range seg {
				id = id*10 + int(c-'0')
			}
			return id
		}
	}
	return 0
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
