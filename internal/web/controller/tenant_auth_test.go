package controller

import (
	"encoding/json"
	"testing"
)

func TestCustomerAllowedAPIPathDefaultDeny(t *testing.T) {
	allowed := []string{
		"/panel/api/account/me",
		"/secret-base/panel/api/account/me",
		"/panel/api/network/map",
		"/panel/api/inbounds/list",
		"/panel/api/clients/list/paged",
		"/panel/api/clients/groups",
		"/panel/api/hosts/list",
		"/panel/api/nodes/list",
		"/panel/api/server/getNewUUID",
	}
	for _, path := range allowed {
		if !customerAllowedAPIPath(path) {
			t.Fatalf("expected customer path %q to be allowed", path)
		}
	}

	denied := []string{
		"/panel/api/nodes/add",
		"/panel/api/server/status",
		"/panel/api/setting/all",
		"/panel/api/customers/list",
		"/panel/api/inbounds/pushClientTraffics",
		"/panel/api/clients/resetAllTraffics",
		"/panel/api/network/map/other",
		"/panel/api/account/me/other",
	}
	for _, path := range denied {
		if customerAllowedAPIPath(path) {
			t.Fatalf("expected customer path %q to be denied", path)
		}
	}
}

func TestCustomerInboundPayloadNeverWritesEmbeddedClients(t *testing.T) {
	stripped, err := stripEmbeddedInboundClients(`{"decryption":"none","clients":[{"email":"victim","id":"secret"}]}`)
	if err != nil {
		t.Fatalf("strip clients: %v", err)
	}
	var strippedValues map[string]any
	if err := json.Unmarshal([]byte(stripped), &strippedValues); err != nil {
		t.Fatalf("decode stripped settings: %v", err)
	}
	clients, ok := strippedValues["clients"].([]any)
	if !ok || len(clients) != 0 {
		t.Fatalf("stripped clients = %#v, want empty array", strippedValues["clients"])
	}

	preserved, err := preserveEmbeddedInboundClients(
		`{"decryption":"new","clients":[{"email":"attacker"}]}`,
		`{"decryption":"old","clients":[{"email":"owned","id":"keep-me"}]}`,
	)
	if err != nil {
		t.Fatalf("preserve clients: %v", err)
	}
	var preservedValues map[string]any
	if err := json.Unmarshal([]byte(preserved), &preservedValues); err != nil {
		t.Fatalf("decode preserved settings: %v", err)
	}
	preservedClients, ok := preservedValues["clients"].([]any)
	if !ok || len(preservedClients) != 1 {
		t.Fatalf("preserved clients = %#v, want one stored client", preservedValues["clients"])
	}
	client, _ := preservedClients[0].(map[string]any)
	if client["email"] != "owned" || client["id"] != "keep-me" {
		t.Fatalf("incoming clients replaced stored identity: %#v", client)
	}
	if preservedValues["decryption"] != "new" {
		t.Fatalf("non-client setting was not updated: %#v", preservedValues)
	}
}
