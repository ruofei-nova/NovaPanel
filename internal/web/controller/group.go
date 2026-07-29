package controller

import (
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"

	"github.com/gin-gonic/gin"
)

type GroupController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
}

func NewGroupController(g *gin.RouterGroup) *GroupController {
	a := &GroupController{}
	a.initRouter(g)
	return a
}

func (a *GroupController) initRouter(g *gin.RouterGroup) {
	g.GET("/groups", a.list)
	g.GET("/groups/:name/emails", a.emails)
	g.POST("/groups/create", a.create)
	g.POST("/groups/rename", a.rename)
	g.POST("/groups/delete", a.delete)
	g.POST("/groups/resetTraffic", a.resetTraffic)
	g.POST("/groups/bulkAdd", a.bulkAdd)
	g.POST("/groups/bulkRemove", a.bulkRemove)
}

func (a *GroupController) list(c *gin.Context) {
	var (
		rows []service.GroupSummary
		err  error
	)
	if user, customer := customerUser(c); customer {
		rows, err = a.clientService.ListGroupsForUser(user.Id)
	} else {
		rows, err = a.clientService.ListGroups()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *GroupController) emails(c *gin.Context) {
	name := c.Param("name")
	var (
		emails []string
		err    error
	)
	if user, customer := customerUser(c); customer {
		emails, err = a.clientService.EmailsByGroupForUser(user.Id, name)
	} else {
		emails, err = a.clientService.EmailsByGroup(name)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, emails, nil)
}

type groupCreateBody struct {
	Name string `json:"name"`
}

func (a *GroupController) create(c *gin.Context) {
	var body groupCreateBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var err error
	if user, customer := customerUser(c); customer {
		err = a.clientService.CreateGroupForUser(user.Id, body.Name)
	} else {
		err = a.clientService.CreateGroup(body.Name)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"name": body.Name}, nil)
	notifyClientsChanged()
}

type groupRenameBody struct {
	OldName string `json:"oldName"`
	NewName string `json:"newName"`
}

func (a *GroupController) rename(c *gin.Context) {
	var body groupRenameBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var (
		affected int
		err      error
	)
	if user, customer := customerUser(c); customer {
		emails, listErr := a.clientService.EmailsByGroupForUser(user.Id, body.OldName)
		if listErr != nil {
			err = listErr
		} else {
			stored, storedErr := a.clientService.HasStoredGroupForUser(user.Id, body.OldName)
			if storedErr != nil {
				err = storedErr
			} else if len(emails) == 0 && !stored {
				tenantNotFound(c, nil)
				return
			} else {
				if len(emails) > 0 {
					affected, err = a.clientService.AddToGroupForUser(user.Id, emails, body.NewName)
					if err == nil && stored && body.OldName != body.NewName {
						err = a.clientService.DeleteStoredGroupForUser(user.Id, body.OldName)
					}
				} else if stored {
					err = a.clientService.RenameStoredGroupForUser(user.Id, body.OldName, body.NewName)
				}
			}
		}
	} else {
		affected, err = a.clientService.RenameGroup(body.OldName, body.NewName)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	markCentralXrayRestart(c, &a.xrayService)
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

type groupDeleteBody struct {
	Name string `json:"name"`
}

func (a *GroupController) delete(c *gin.Context) {
	var body groupDeleteBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var (
		affected int
		err      error
	)
	if user, customer := customerUser(c); customer {
		emails, listErr := a.clientService.EmailsByGroupForUser(user.Id, body.Name)
		if listErr != nil {
			err = listErr
		} else {
			stored, storedErr := a.clientService.HasStoredGroupForUser(user.Id, body.Name)
			if storedErr != nil {
				err = storedErr
			} else if len(emails) == 0 && !stored {
				tenantNotFound(c, nil)
				return
			} else {
				if len(emails) > 0 {
					affected, err = a.clientService.AddToGroupForUser(user.Id, emails, "")
				}
				if err == nil && stored {
					err = a.clientService.DeleteStoredGroupForUser(user.Id, body.Name)
				}
			}
		}
	} else {
		affected, err = a.clientService.DeleteGroup(body.Name)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	markCentralXrayRestart(c, &a.xrayService)
	jsonObj(c, gin.H{"affected": affected}, nil)
	notifyClientsChanged()
}

type groupResetTrafficBody struct {
	Name string `json:"name"`
}

func (a *GroupController) resetTraffic(c *gin.Context) {
	var body groupResetTrafficBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var err error
	if user, customer := customerUser(c); customer {
		emails, listErr := a.clientService.EmailsByGroupForUser(user.Id, body.Name)
		if listErr != nil {
			err = listErr
		} else if len(emails) == 0 {
			tenantNotFound(c, nil)
			return
		} else {
			_, err = a.clientService.BulkResetTraffic(&a.inboundService, emails)
		}
	} else {
		err = a.clientService.ResetGroupTraffic(body.Name)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"name": body.Name}, nil)
	notifyClientsChanged()
}

type bulkAddToGroupRequest struct {
	Emails []string `json:"emails"`
	Group  string   `json:"group"`
}

func (a *GroupController) bulkAdd(c *gin.Context) {
	var req bulkAddToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if strings.TrimSpace(req.Group) == "" {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), common.NewError("group name is required"))
		return
	}
	var (
		affected int
		err      error
	)
	if user, customer := customerUser(c); customer {
		affected, err = a.clientService.AddToGroupForUser(user.Id, req.Emails, req.Group)
	} else {
		affected, err = a.clientService.AddToGroup(req.Emails, req.Group)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	markCentralXrayRestart(c, &a.xrayService)
	notifyClientsChanged()
}

type bulkRemoveFromGroupRequest struct {
	Emails []string `json:"emails"`
}

func (a *GroupController) bulkRemove(c *gin.Context) {
	var req bulkRemoveFromGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var (
		affected int
		err      error
	)
	if user, customer := customerUser(c); customer {
		affected, err = a.clientService.AddToGroupForUser(user.Id, req.Emails, "")
	} else {
		affected, err = a.clientService.RemoveFromGroup(req.Emails)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	markCentralXrayRestart(c, &a.xrayService)
	notifyClientsChanged()
}
