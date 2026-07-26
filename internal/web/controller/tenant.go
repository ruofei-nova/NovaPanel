package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service/panel"
	"github.com/mhsanaei/3x-ui/v3/internal/web/session"
)

type TenantController struct {
	customers panel.CustomerService
}

func NewTenantController(api *gin.RouterGroup) *TenantController {
	controller := &TenantController{}

	account := api.Group("/account")
	account.GET("/me", controller.me)

	network := api.Group("/network")
	network.GET("/map", controller.networkMap)

	customers := api.Group("/customers")
	customers.Use(controller.adminOnly)
	customers.GET("/list", controller.listCustomers)
	customers.POST("/create", controller.createCustomer)
	customers.POST("/:id/enabled", controller.setCustomerEnabled)
	customers.POST("/:id/reset-password", controller.resetCustomerPassword)
	customers.POST("/:id/delete", controller.deleteCustomer)
	return controller
}

func (a *TenantController) adminOnly(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil || user.Role != panel.UserRoleAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"success": false,
			"msg":     "administrator access required",
		})
		return
	}
	c.Next()
}

func (a *TenantController) me(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	jsonObj(c, gin.H{
		"id": user.Id, "username": user.Username, "role": user.Role,
	}, nil)
}

func (a *TenantController) networkMap(c *gin.Context) {
	user := session.GetLoginUser(c)
	if user == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	var ownerID *int
	if user.Role == panel.UserRoleCustomer {
		ownerID = &user.Id
	}
	payload, err := service.GetNetworkMap(ownerID)
	jsonObj(c, payload, err)
}

func (a *TenantController) listCustomers(c *gin.Context) {
	users, err := a.customers.List()
	jsonObj(c, users, err)
}

func (a *TenantController) createCustomer(c *gin.Context) {
	var req struct {
		Username string `json:"username" form:"username"`
		Password string `json:"password" form:"password"`
	}
	if err := c.ShouldBind(&req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	user, password, err := a.customers.Create(req.Username, req.Password)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, gin.H{"user": user, "password": password}, nil)
}

func customerID(c *gin.Context) (int, error) {
	return strconv.Atoi(c.Param("id"))
}

func (a *TenantController) setCustomerEnabled(c *gin.Context) {
	id, err := customerID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	var req struct {
		Enabled bool `json:"enabled" form:"enabled"`
	}
	if err := c.ShouldBind(&req); err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, gin.H{"enabled": req.Enabled}, a.customers.SetEnabled(id, req.Enabled))
}

func (a *TenantController) resetCustomerPassword(c *gin.Context) {
	id, err := customerID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	password, err := a.customers.ResetPassword(id)
	jsonObj(c, gin.H{"password": password}, err)
}

func (a *TenantController) deleteCustomer(c *gin.Context) {
	id, err := customerID(c)
	if err != nil {
		jsonObj(c, nil, err)
		return
	}
	jsonObj(c, gin.H{"deleted": id}, a.customers.Delete(id))
}
