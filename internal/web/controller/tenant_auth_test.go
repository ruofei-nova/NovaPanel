package controller

import "testing"

func TestCustomerAllowedAPIPathDefaultDeny(t *testing.T) {
	allowed := []string{
		"/panel/api/account/me",
		"/secret-base/panel/api/account/me",
		"/panel/api/network/map",
	}
	for _, path := range allowed {
		if !customerAllowedAPIPath(path) {
			t.Fatalf("expected customer path %q to be allowed", path)
		}
	}

	denied := []string{
		"/panel/api/nodes/list",
		"/panel/api/server/status",
		"/panel/api/setting/all",
		"/panel/api/customers/list",
		"/panel/api/network/map/other",
		"/panel/api/account/me/other",
	}
	for _, path := range denied {
		if customerAllowedAPIPath(path) {
			t.Fatalf("expected customer path %q to be denied", path)
		}
	}
}
