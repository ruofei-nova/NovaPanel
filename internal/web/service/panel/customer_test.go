package panel

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func initCustomerTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dir)
	if err := database.InitDB(filepath.Join(dir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
}

func TestCustomerLifecycleAndAuthentication(t *testing.T) {
	initCustomerTestDB(t)
	customers := &CustomerService{}
	auth := &UserService{}

	user, password, err := customers.Create("tenant-a", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.Role != UserRoleCustomer || !user.Enabled {
		t.Fatalf("unexpected customer: %#v", user)
	}
	if len(password) < 12 {
		t.Fatalf("generated password is too short: %d", len(password))
	}
	if err := auth.settingService.SetTwoFactorEnable(true); err != nil {
		t.Fatalf("enable admin 2FA: %v", err)
	}
	if err := auth.settingService.SetTwoFactorToken("JBSWY3DPEHPK3PXP"); err != nil {
		t.Fatalf("set admin 2FA token: %v", err)
	}
	if loggedIn, err := auth.CheckUser("tenant-a", password, ""); err != nil || loggedIn.Id != user.Id {
		t.Fatalf("customer login failed while admin 2FA is enabled: user=%#v err=%v", loggedIn, err)
	}

	if err := customers.SetEnabled(user.Id, false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	if loggedIn, err := auth.CheckUser("tenant-a", password, ""); err == nil || loggedIn != nil {
		t.Fatal("disabled customer was able to log in")
	}

	if err := customers.SetEnabled(user.Id, true); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	nextPassword, err := customers.ResetPassword(user.Id)
	if err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}
	if loggedIn, err := auth.CheckUser("tenant-a", password, ""); err == nil || loggedIn != nil {
		t.Fatal("old password remained valid after reset")
	}
	if loggedIn, err := auth.CheckUser("tenant-a", nextPassword, ""); err != nil || loggedIn.Id != user.Id {
		t.Fatalf("reset password login failed: user=%#v err=%v", loggedIn, err)
	}
}

func TestDeleteCustomerUnassignsNodesAndReturnsInboundsToAdmin(t *testing.T) {
	initCustomerTestDB(t)
	db := database.GetDB()
	customers := &CustomerService{}

	customer, _, err := customers.Create("tenant-delete", "long-enough-test-password")
	if err != nil {
		t.Fatalf("create customer: %v", err)
	}
	var admin model.User
	if err := db.Where("role = ?", UserRoleAdmin).Order("id asc").First(&admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	node := &model.Node{
		Name: "tenant-delete-node", Address: "node.example.test", Port: 2053,
		OwnerUserID: &customer.Id,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	inbound := &model.Inbound{
		UserId: customer.Id, NodeID: &node.Id, Tag: "tenant-delete-inbound",
		Port: 443, Protocol: model.VLESS,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	standaloneInbound := &model.Inbound{
		UserId: customer.Id, Tag: "tenant-delete-standalone",
		Port: 8443, Protocol: model.VLESS,
	}
	if err := db.Create(standaloneInbound).Error; err != nil {
		t.Fatalf("create standalone inbound: %v", err)
	}
	group := &model.ClientGroup{Name: "tenant-delete-group", OwnerUserID: &customer.Id}
	if err := db.Create(group).Error; err != nil {
		t.Fatalf("create customer group: %v", err)
	}

	if err := customers.Delete(customer.Id); err != nil {
		t.Fatalf("delete customer: %v", err)
	}
	if err := db.First(node, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if node.OwnerUserID != nil {
		t.Fatalf("node owner remained assigned: %v", *node.OwnerUserID)
	}
	if err := db.First(inbound, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if inbound.UserId != admin.Id {
		t.Fatalf("inbound user_id = %d, want admin %d", inbound.UserId, admin.Id)
	}
	if err := db.First(standaloneInbound, standaloneInbound.Id).Error; err != nil {
		t.Fatalf("reload standalone inbound: %v", err)
	}
	if standaloneInbound.UserId != admin.Id {
		t.Fatalf("standalone inbound user_id = %d, want admin %d", standaloneInbound.UserId, admin.Id)
	}
	var groupCount int64
	if err := db.Model(&model.ClientGroup{}).Where("id = ?", group.Id).Count(&groupCount).Error; err != nil {
		t.Fatalf("count deleted customer groups: %v", err)
	}
	if groupCount != 0 {
		t.Fatalf("customer-owned group remained after account deletion")
	}
}
