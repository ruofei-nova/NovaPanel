package panel

import (
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
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
