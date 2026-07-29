package controller

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/websocket"

	"github.com/gin-gonic/gin"
)

func notifyClientsChanged() {
	websocket.BroadcastInvalidate(websocket.MessageTypeClients)
}

func parseInboundIdsQuery(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		if id, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

type ClientController struct {
	clientService  service.ClientService
	inboundService service.InboundService
	xrayService    service.XrayService
	settingService service.SettingService
}

func NewClientController(g *gin.RouterGroup) *ClientController {
	a := &ClientController{}
	a.initRouter(g)
	return a
}

func (a *ClientController) initRouter(g *gin.RouterGroup) {
	g.GET("/list", a.list)
	g.GET("/list/paged", a.listPaged)
	g.GET("/get/:email", a.get)
	g.GET("/traffic/:email", a.getTrafficByEmail)
	g.GET("/subLinks/:subId", a.getSubLinks)
	g.GET("/links/:email", a.getClientLinks)

	g.POST("/add", a.create)
	g.POST("/update/:email", a.update)
	g.POST("/del/:email", a.delete)
	g.POST("/:email/attach", a.attach)
	g.POST("/:email/detach", a.detach)
	g.POST("/:email/externalLinks", a.setExternalLinks)
	g.GET("/export", a.export)
	g.POST("/import", a.importClients)
	g.POST("/delOrphans", a.delOrphans)
	g.POST("/resetAllTraffics", a.resetAllTraffics)
	g.POST("/delDepleted", a.delDepleted)
	g.POST("/bulkAdjust", a.bulkAdjust)
	g.POST("/bulkEnable", a.bulkEnable)
	g.POST("/bulkDisable", a.bulkDisable)
	g.POST("/bulkDel", a.bulkDelete)
	g.POST("/bulkCreate", a.bulkCreate)
	g.POST("/bulkAttach", a.bulkAttach)
	g.POST("/bulkDetach", a.bulkDetach)
	g.POST("/bulkResetTraffic", a.bulkResetTraffic)
	g.POST("/resetTraffic/:email", a.resetTrafficByEmail)
	g.POST("/updateTraffic/:email", a.updateTrafficByEmail)
	g.POST("/ips/:email", a.getIps)
	g.POST("/clearIps/:email", a.clearIps)
	g.POST("/onlines", a.onlines)
	g.POST("/onlinesByGuid", a.onlinesByGuid)
	g.POST("/clientIpsByGuid", a.clientIpsByGuid)
	g.POST("/activeInbounds", a.activeInbounds)
	g.POST("/lastOnline", a.lastOnline)
}

func (a *ClientController) list(c *gin.Context) {
	var (
		rows []service.ClientWithAttachments
		err  error
	)
	if user, customer := customerUser(c); customer {
		rows, err = a.clientService.ListForUser(user.Id)
	} else {
		rows, err = a.clientService.List()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, rows, nil)
}

func (a *ClientController) listPaged(c *gin.Context) {
	var params service.ClientPageParams
	if err := c.ShouldBindQuery(&params); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	var (
		resp *service.ClientPageResponse
		err  error
	)
	if user, customer := customerUser(c); customer {
		resp, err = a.clientService.ListPagedForUser(&a.inboundService, &a.settingService, user.Id, params)
	} else {
		resp, err = a.clientService.ListPaged(&a.inboundService, &a.settingService, params)
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, resp, nil)
}

func (a *ClientController) get(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	rec, err := a.clientService.GetRecordByEmail(nil, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	inboundIds, err := a.clientService.GetInboundIdsForRecord(rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	externalLinks, err := a.clientService.GetExternalLinksForRecord(rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	flow, err := a.clientService.EffectiveFlow(nil, rec.Id)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	rec.Flow = flow
	// Consumed bytes (up+down, including cross-node global overlay) so API
	// consumers can pair usage with the client's totalGB quota (#4973).
	// Best-effort: a traffic lookup failure must not break the client fetch.
	var usedTraffic int64
	if t, tErr := a.inboundService.GetClientTrafficByEmail(email); tErr == nil && t != nil {
		usedTraffic = t.Up + t.Down
	}
	jsonObj(c, gin.H{"client": rec, "inboundIds": inboundIds, "externalLinks": externalLinks, "usedTraffic": usedTraffic}, nil)
}

func (a *ClientController) create(c *gin.Context) {
	var payload service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, customer := customerUser(c); customer {
		if len(payload.InboundIds) == 0 || !requireOwnedInboundIDs(c, payload.InboundIds) {
			return
		}
		if !requireCreatableClientIdentity(c, payload.Client.Email, payload.Client.SubID) {
			return
		}
	}
	needRestart, err := a.clientService.Create(&a.inboundService, &payload)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(payload.InboundIds)), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) update(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	var updated model.Client
	if err := c.ShouldBindJSON(&updated); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, customer := customerUser(c); customer && strings.TrimSpace(updated.Email) != "" &&
		!requireCreatableClientIdentity(c, updated.Email, updated.SubID) {
		return
	}
	inboundFilter := parseInboundIdsQuery(c.Query("inboundIds"))
	if _, customer := customerUser(c); customer && len(inboundFilter) > 0 && !requireOwnedInboundIDs(c, inboundFilter) {
		return
	}
	needRestart, err := a.clientService.UpdateByEmail(&a.inboundService, email, updated, inboundFilter...)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), pendingNodeObj(a.clientService.HasPendingNode(&a.inboundService, email)), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) delete(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	keepTraffic := c.Query("keepTraffic") == "1"
	needRestart, err := a.clientService.DeleteByEmail(&a.inboundService, email, keepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type attachDetachBody struct {
	InboundIds []int `json:"inboundIds"`
}

type externalLinksBody struct {
	ExternalLinks []service.ExternalLinkInput `json:"externalLinks"`
}

func (a *ClientController) attach(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedInboundIDs(c, body.InboundIds) {
		return
	}
	needRestart, err := a.clientService.AttachByEmail(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientAddSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) setExternalLinks(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	var body externalLinksBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.clientService.SetExternalLinksByEmail(email, body.ExternalLinks); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
}

func (a *ClientController) resetAllTraffics(c *gin.Context) {
	if _, customer := customerUser(c); customer {
		c.AbortWithStatusJSON(403, gin.H{"success": false, "msg": "global traffic reset requires an administrator"})
		return
	}
	needRestart, err := a.clientService.ResetAllTraffics()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetAllClientTrafficSuccess"), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type bulkAdjustRequest struct {
	Emails   []string `json:"emails"`
	AddDays  int      `json:"addDays"`
	AddBytes int64    `json:"addBytes"`
	Flow     string   `json:"flow"`
}

func (a *ClientController) bulkAdjust(c *gin.Context) {
	var req bulkAdjustRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) {
		return
	}
	result, needRestart, err := a.clientService.BulkAdjust(&a.inboundService, req.Emails, req.AddDays, req.AddBytes, req.Flow)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type bulkDeleteRequest struct {
	Emails      []string `json:"emails"`
	KeepTraffic bool     `json:"keepTraffic"`
}

type bulkAttachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkAttach(c *gin.Context) {
	var req bulkAttachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) || !requireOwnedInboundIDs(c, req.InboundIds) {
		return
	}
	result, needRestart, err := a.clientService.BulkAttach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type bulkDetachRequest struct {
	Emails     []string `json:"emails"`
	InboundIds []int    `json:"inboundIds"`
}

func (a *ClientController) bulkDetach(c *gin.Context) {
	var req bulkDetachRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) || !requireOwnedInboundIDs(c, req.InboundIds) {
		return
	}
	result, needRestart, err := a.clientService.BulkDetach(&a.inboundService, req.Emails, req.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkDelete(c *gin.Context) {
	var req bulkDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) {
		return
	}
	result, needRestart, err := a.clientService.BulkDelete(&a.inboundService, req.Emails, req.KeepTraffic)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type bulkEnableRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkEnable(c *gin.Context) {
	a.bulkSetEnable(c, true)
}

func (a *ClientController) bulkDisable(c *gin.Context) {
	a.bulkSetEnable(c, false)
}

func (a *ClientController) bulkSetEnable(c *gin.Context, enable bool) {
	var req bulkEnableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) {
		return
	}
	result, needRestart, err := a.clientService.BulkSetEnable(&a.inboundService, req.Emails, enable)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) bulkCreate(c *gin.Context) {
	var payloads []service.ClientCreatePayload
	if err := c.ShouldBindJSON(&payloads); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, customer := customerUser(c); customer {
		if len(payloads) == 0 {
			tenantNotFound(c, nil)
			return
		}
		for i := range payloads {
			if len(payloads[i].InboundIds) == 0 || !requireOwnedInboundIDs(c, payloads[i].InboundIds) {
				return
			}
			if !requireCreatableClientIdentity(c, payloads[i].Client.Email, payloads[i].Client.SubID) {
				return
			}
		}
	}
	result, needRestart, err := a.clientService.BulkCreate(&a.inboundService, payloads)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) delDepleted(c *gin.Context) {
	if _, customer := customerUser(c); customer {
		c.AbortWithStatusJSON(403, gin.H{"success": false, "msg": "global cleanup requires an administrator"})
		return
	}
	deleted, needRestart, err := a.clientService.DelDepleted(&a.inboundService)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"deleted": deleted}, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

// export returns every client as a {client, inboundIds} list in the standard
// envelope. The frontend renders it in a read-only CodeMirror viewer (Copy /
// Download), so this hands back data rather than streaming a file attachment.
func (a *ClientController) export(c *gin.Context) {
	var (
		items []service.ClientCreatePayload
		err   error
	)
	if user, customer := customerUser(c); customer {
		items, err = a.clientService.ExportForUser(user.Id)
	} else {
		items, err = a.clientService.ExportAll()
	}
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, items, nil)
}

type importClientsRequest struct {
	Data string `json:"data"`
}

// importClients accepts the pasted export text as a JSON body { "data": "..." },
// mirroring the inbound import flow. The data string is itself a JSON-encoded
// []ClientCreatePayload, so it is unmarshalled in a second step.
func (a *ClientController) importClients(c *gin.Context) {
	var req importClientsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	var items []service.ClientCreatePayload
	if err := json.Unmarshal([]byte(req.Data), &items); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if _, customer := customerUser(c); customer {
		if len(items) == 0 {
			tenantNotFound(c, nil)
			return
		}
		for i := range items {
			if len(items[i].InboundIds) == 0 || !requireOwnedInboundIDs(c, items[i].InboundIds) {
				return
			}
			if !requireCreatableClientIdentity(c, items[i].Client.Email, items[i].Client.SubID) {
				return
			}
		}
	}
	result, needRestart, err := a.clientService.ImportClients(&a.inboundService, items)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, result, nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

func (a *ClientController) delOrphans(c *gin.Context) {
	if _, customer := customerUser(c); customer {
		c.AbortWithStatusJSON(403, gin.H{"success": false, "msg": "global cleanup requires an administrator"})
		return
	}
	deleted, err := a.clientService.DeleteOrphans()
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"deleted": deleted}, nil)
	notifyClientsChanged()
}

func (a *ClientController) resetTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	needRestart, err := a.clientService.ResetTrafficByEmail(&a.inboundService, email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.resetInboundClientTrafficSuccess"), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type trafficUpdateRequest struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func (a *ClientController) updateTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	var req trafficUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if err := a.inboundService.UpdateClientTrafficByEmail(email, req.Upload, req.Download); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientUpdateSuccess"), nil)
	notifyClientsChanged()
}

func (a *ClientController) getIps(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	infos, err := a.inboundService.GetClientIpsWithNodes(email)
	jsonObj(c, infos, err)
}

func (a *ClientController) clientIpsByGuid(c *gin.Context) {
	data, err := a.inboundService.GetClientIpsByGuid()
	jsonObj(c, data, err)
}

func (a *ClientController) clearIps(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	if err := a.inboundService.ClearClientIps(email); err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.updateSuccess"), err)
		return
	}
	jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.logCleanSuccess"), nil)
}

func (a *ClientController) onlines(c *gin.Context) {
	values := a.inboundService.GetOnlineClients()
	if allowed, customer := customerOwnedEmailSet(c); customer {
		if c.IsAborted() {
			return
		}
		values = filterOwnedEmails(values, allowed)
	}
	jsonObj(c, values, nil)
}

func (a *ClientController) onlinesByGuid(c *gin.Context) {
	values := a.inboundService.GetOnlineClientsByGuid()
	if allowed, customer := customerOwnedEmailSet(c); customer {
		if c.IsAborted() {
			return
		}
		filtered := make(map[string][]string)
		for guid, emails := range values {
			if owned := filterOwnedEmails(emails, allowed); len(owned) > 0 {
				filtered[guid] = owned
			}
		}
		values = filtered
	}
	jsonObj(c, values, nil)
}

func (a *ClientController) activeInbounds(c *gin.Context) {
	if _, customer := customerUser(c); customer {
		jsonObj(c, map[string][]string{}, nil)
		return
	}
	jsonObj(c, a.inboundService.GetActiveInboundsByGuid(), nil)
}

func (a *ClientController) lastOnline(c *gin.Context) {
	data, err := a.inboundService.GetClientsLastOnline()
	if err == nil {
		if allowed, customer := customerOwnedEmailSet(c); customer {
			if c.IsAborted() {
				return
			}
			filtered := make(map[string]int64)
			for email, last := range data {
				if _, ok := allowed[email]; ok {
					filtered[email] = last
				}
			}
			data = filtered
		}
	}
	jsonObj(c, data, err)
}

func (a *ClientController) getTrafficByEmail(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	traffic, err := a.inboundService.GetClientTrafficByEmail(email)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.trafficGetError"), err)
		return
	}
	jsonObj(c, traffic, nil)
}

func (a *ClientController) getSubLinks(c *gin.Context) {
	if !requireOwnedSubID(c, c.Param("subId")) {
		return
	}
	links, err := a.inboundService.GetSubLinks(resolveHost(c), c.Param("subId"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) getClientLinks(c *gin.Context) {
	if !requireOwnedClient(c, c.Param("email")) {
		return
	}
	links, err := a.inboundService.GetAllClientLinks(resolveHost(c), c.Param("email"))
	if err != nil {
		jsonMsg(c, I18nWeb(c, "pages.inbounds.toasts.obtain"), err)
		return
	}
	jsonObj(c, links, nil)
}

func (a *ClientController) detach(c *gin.Context) {
	email := c.Param("email")
	if !requireOwnedClient(c, email) {
		return
	}
	var body attachDetachBody
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedInboundIDs(c, body.InboundIds) {
		return
	}
	needRestart, err := a.clientService.DetachByEmailMany(&a.inboundService, email, body.InboundIds)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonMsgObj(c, I18nWeb(c, "pages.inbounds.toasts.inboundClientDeleteSuccess"), pendingNodeObj(a.inboundService.AnyNodePending(body.InboundIds)), nil)
	if needRestart {
		markCentralXrayRestart(c, &a.xrayService)
	}
	notifyClientsChanged()
}

type bulkResetRequest struct {
	Emails []string `json:"emails"`
}

func (a *ClientController) bulkResetTraffic(c *gin.Context) {
	var req bulkResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	if !requireOwnedClients(c, req.Emails) {
		return
	}
	affected, err := a.clientService.BulkResetTraffic(&a.inboundService, req.Emails)
	if err != nil {
		jsonMsg(c, I18nWeb(c, "somethingWentWrong"), err)
		return
	}
	jsonObj(c, gin.H{"affected": affected}, nil)
	markCentralXrayRestart(c, &a.xrayService)
	notifyClientsChanged()
}
