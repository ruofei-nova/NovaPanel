package controller

import (
	"net/http"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"

	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var tenantScope service.TenantScopeService

func customerUser(c *gin.Context) (*model.User, bool) {
	// Normal panel routes always install the session middleware. Keeping this
	// helper tolerant of bare Gin contexts preserves controller unit tests and
	// internal callers that exercise handlers without the full router stack.
	if _, apiUser := c.Get("api_auth_user"); !apiUser {
		if _, hasSession := c.Get(ginsessions.DefaultKey); !hasSession {
			return nil, false
		}
	}
	user := session.GetLoginUser(c)
	return user, user != nil && user.Role == panel.UserRoleCustomer
}

func tenantNotFound(c *gin.Context, err error) bool {
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return false
	}
	c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
		"success": false,
		"msg":     "resource not found",
	})
	return false
}

func requireOwnedNode(c *gin.Context, nodeID int) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsNode(user.Id, nodeID)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedInbound(c *gin.Context, inboundID int) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsInbound(user.Id, inboundID)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedInboundIDs(c *gin.Context, inboundIDs []int) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsInboundIDs(user.Id, inboundIDs)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedClient(c *gin.Context, email string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsClientEmail(user.Id, email)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedClients(c *gin.Context, emails []string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsClientEmails(user.Id, emails)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireCreatableClientIdentity(c *gin.Context, email, subID string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	allowed, err := tenantScope.CanCreateClientIdentity(user.Id, email, subID)
	if err != nil || !allowed {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedSubID(c *gin.Context, subID string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	_, owned, err := tenantScope.ClientEmailBySubID(user.Id, subID)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedHostGroup(c *gin.Context, groupID string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsHostGroup(user.Id, groupID)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func requireOwnedHostGroups(c *gin.Context, groupIDs []string) bool {
	user, customer := customerUser(c)
	if !customer {
		return true
	}
	owned, err := tenantScope.OwnsHostGroups(user.Id, groupIDs)
	if err != nil || !owned {
		return tenantNotFound(c, err)
	}
	return true
}

func customerOwnedEmailSet(c *gin.Context) (map[string]struct{}, bool) {
	user, customer := customerUser(c)
	if !customer {
		return nil, false
	}
	emails, err := tenantScope.OwnedClientEmails(user.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return map[string]struct{}{}, true
	}
	set := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		set[email] = struct{}{}
	}
	return set, true
}

func filterOwnedEmails(values []string, allowed map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := allowed[value]; ok {
			out = append(out, value)
		}
	}
	return out
}

// Delegated customers only operate node-bound inbounds. Their mutations may
// need a remote-node reconcile, but must never mark the central panel's Xray
// template for restart.
func markCentralXrayRestart(c *gin.Context, xrayService *service.XrayService) {
	if _, customer := customerUser(c); customer {
		return
	}
	xrayService.SetToNeedRestart()
}
