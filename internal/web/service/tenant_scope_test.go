package service

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestNodeAssignmentPersistsAndMigratesInboundOwner(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	admin := &model.User{}
	customer := &model.User{Username: "tenant-customer", Role: "customer", Enabled: true}
	if err := db.Where("role = ?", "admin").Order("id asc").First(admin).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}

	token := "write-only-node-token"
	node, err := (&NodeService{}).CreateFromRequest(&NodeMutationRequest{
		Name:     "tenant-node",
		Scheme:   "https",
		Address:  "node.example.test",
		Port:     2053,
		BasePath: "/panel/",
		ApiToken: &token,
		Enable:   true,
	})
	if err != nil {
		t.Fatalf("create node: %v", err)
	}
	inbound := &model.Inbound{
		UserId:         admin.Id,
		NodeID:         &node.Id,
		Tag:            "tenant-node-inbound",
		Remark:         "remote inbound",
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{}`,
		Sniffing:       `{}`,
		Enable:         true,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}

	ownerID := customer.Id
	if err := (&NodeService{}).UpdateFromRequest(node.Id, &NodeMutationRequest{
		Name:          "tenant-node-renamed",
		Scheme:        "https",
		Address:       "node.example.test",
		Port:          2053,
		BasePath:      "/panel/",
		Enable:        true,
		OwnerUserID:   &ownerID,
		Country:       "Malaysia",
		City:          "Kuala Lumpur",
		Latitude:      3.139,
		Longitude:     101.6869,
		TlsVerifyMode: "verify",
	}); err != nil {
		t.Fatalf("assign node: %v", err)
	}

	var storedNode model.Node
	if err := db.First(&storedNode, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if storedNode.OwnerUserID == nil || *storedNode.OwnerUserID != customer.Id {
		t.Fatalf("owner_user_id = %v, want %d", storedNode.OwnerUserID, customer.Id)
	}
	if storedNode.Country != "Malaysia" || storedNode.City != "Kuala Lumpur" ||
		storedNode.Latitude != 3.139 || storedNode.Longitude != 101.6869 {
		t.Fatalf("node geodata was not persisted: %#v", storedNode)
	}
	var storedInbound model.Inbound
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if storedInbound.UserId != customer.Id {
		t.Fatalf("inbound user_id = %d, want customer %d", storedInbound.UserId, customer.Id)
	}

	if err := (&NodeService{}).UpdateFromRequest(node.Id, &NodeMutationRequest{
		Name:           "tenant-node-renamed",
		Scheme:         "https",
		Address:        "node.example.test",
		Port:           2053,
		BasePath:       "/panel/",
		Enable:         true,
		TlsVerifyMode:  "verify",
		OwnerUserIDSet: true,
	}); err != nil {
		t.Fatalf("unassign node: %v", err)
	}
	if err := db.First(&storedInbound, inbound.Id).Error; err != nil {
		t.Fatalf("reload unassigned inbound: %v", err)
	}
	if storedInbound.UserId != admin.Id {
		t.Fatalf("unassigned inbound user_id = %d, want admin %d", storedInbound.UserId, admin.Id)
	}
}

func TestNodeUpdateOmittedOwnerPreservesAssignment(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	customer := &model.User{Username: "preserved-owner", Role: "customer", Enabled: true}
	if err := db.Create(customer).Error; err != nil {
		t.Fatalf("create customer: %v", err)
	}
	node := &model.Node{
		Name:          "preserved-node",
		Address:       "preserved.example.test",
		Port:          2053,
		Scheme:        "https",
		BasePath:      "/",
		TlsVerifyMode: "verify",
		ApiToken:      "stored-token",
		Enable:        true,
		OwnerUserID:   &customer.Id,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := (&NodeService{}).UpdateFromRequest(node.Id, &NodeMutationRequest{
		Name:          "preserved-node-renamed",
		Address:       node.Address,
		Port:          node.Port,
		Scheme:        node.Scheme,
		BasePath:      node.BasePath,
		TlsVerifyMode: node.TlsVerifyMode,
		Enable:        true,
	}); err != nil {
		t.Fatalf("partial update: %v", err)
	}
	var stored model.Node
	if err := db.First(&stored, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if stored.OwnerUserID == nil || *stored.OwnerUserID != customer.Id {
		t.Fatalf("omitted owner cleared assignment: %v", stored.OwnerUserID)
	}

	var decoded NodeMutationRequest
	if err := json.Unmarshal([]byte(`{"name":"x","address":"x","port":1,"ownerUserId":null}`), &decoded); err != nil {
		t.Fatalf("decode explicit null owner: %v", err)
	}
	if !decoded.OwnerUserIDSet || decoded.OwnerUserID != nil {
		t.Fatalf("explicit null owner not detected: set=%v owner=%v", decoded.OwnerUserIDSet, decoded.OwnerUserID)
	}
}

func TestNodeTransferWithClientsRequiresCleanup(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	customerA := &model.User{Username: "transfer-a", Role: "customer", Enabled: true}
	customerB := &model.User{Username: "transfer-b", Role: "customer", Enabled: true}
	if err := db.Create(customerA).Error; err != nil {
		t.Fatalf("create customer A: %v", err)
	}
	if err := db.Create(customerB).Error; err != nil {
		t.Fatalf("create customer B: %v", err)
	}
	node := &model.Node{
		Name: "transfer-node", Address: "transfer.example.test", Port: 2053,
		Scheme: "https", BasePath: "/", TlsVerifyMode: "verify",
		ApiToken: "stored-token", Enable: true, OwnerUserID: &customerA.Id,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	inbound := &model.Inbound{
		UserId: customerA.Id, NodeID: &node.Id, Tag: "transfer-inbound",
		Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`,
	}
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	client := &model.ClientRecord{Email: "transfer-client@example.test", SubID: "transfer-sub", Enable: true}
	if err := db.Create(client).Error; err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := db.Create(&model.ClientInbound{ClientId: client.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("attach client: %v", err)
	}

	if err := (&NodeService{}).UpdateFromRequest(node.Id, &NodeMutationRequest{
		Name: node.Name, Address: node.Address, Port: node.Port, Scheme: node.Scheme,
		BasePath: node.BasePath, TlsVerifyMode: node.TlsVerifyMode, Enable: true,
		OwnerUserID: &customerB.Id,
	}); err == nil {
		t.Fatal("populated node transferred directly between customers")
	}
	var stored model.Node
	if err := db.First(&stored, node.Id).Error; err != nil {
		t.Fatalf("reload node: %v", err)
	}
	if stored.OwnerUserID == nil || *stored.OwnerUserID != customerA.Id {
		t.Fatalf("failed transfer changed owner: %v", stored.OwnerUserID)
	}
}

func TestTenantScopeRejectsCrossTenantAndMixedClientAttachments(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	customerA := &model.User{Username: "scope-a", Role: "customer", Enabled: true}
	customerB := &model.User{Username: "scope-b", Role: "customer", Enabled: true}
	if err := db.Create(customerA).Error; err != nil {
		t.Fatalf("create customer A: %v", err)
	}
	if err := db.Create(customerB).Error; err != nil {
		t.Fatalf("create customer B: %v", err)
	}
	nodeA := &model.Node{Name: "scope-node-a", Address: "a.example.test", Port: 2053, OwnerUserID: &customerA.Id}
	nodeB := &model.Node{Name: "scope-node-b", Address: "b.example.test", Port: 2053, OwnerUserID: &customerB.Id}
	if err := db.Create(nodeA).Error; err != nil {
		t.Fatalf("create node A: %v", err)
	}
	if err := db.Create(nodeB).Error; err != nil {
		t.Fatalf("create node B: %v", err)
	}
	inboundA := &model.Inbound{UserId: customerA.Id, NodeID: &nodeA.Id, Tag: "scope-in-a", Port: 10001, Protocol: model.VLESS}
	inboundB := &model.Inbound{UserId: customerB.Id, NodeID: &nodeB.Id, Tag: "scope-in-b", Port: 10002, Protocol: model.VLESS}
	if err := db.Create(inboundA).Error; err != nil {
		t.Fatalf("create inbound A: %v", err)
	}
	if err := db.Create(inboundB).Error; err != nil {
		t.Fatalf("create inbound B: %v", err)
	}

	clientA := &model.ClientRecord{Email: "only-a@example.test", SubID: "sub-a", Enable: true}
	clientB := &model.ClientRecord{Email: "only-b@example.test", SubID: "sub-b", Password: "victim-secret", Enable: true}
	mixed := &model.ClientRecord{Email: "mixed@example.test", SubID: "sub-mixed", Enable: true}
	for _, client := range []*model.ClientRecord{clientA, clientB, mixed} {
		if err := db.Create(client).Error; err != nil {
			t.Fatalf("create client %q: %v", client.Email, err)
		}
	}
	links := []model.ClientInbound{
		{ClientId: clientA.Id, InboundId: inboundA.Id},
		{ClientId: clientB.Id, InboundId: inboundB.Id},
		{ClientId: mixed.Id, InboundId: inboundA.Id},
		{ClientId: mixed.Id, InboundId: inboundB.Id},
	}
	if err := db.Create(&links).Error; err != nil {
		t.Fatalf("create client attachments: %v", err)
	}

	scope := &TenantScopeService{}
	if owned, err := scope.OwnsNode(customerA.Id, nodeA.Id); err != nil || !owned {
		t.Fatalf("customer A should own node A: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsNode(customerA.Id, nodeB.Id); err != nil || owned {
		t.Fatalf("customer A must not own node B: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsInbound(customerA.Id, inboundB.Id); err != nil || owned {
		t.Fatalf("customer A must not own inbound B: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsInboundIDs(customerA.Id, []int{inboundA.Id, inboundB.Id}); err != nil || owned {
		t.Fatalf("mixed inbound set must be denied: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsClientEmail(customerA.Id, clientA.Email); err != nil || !owned {
		t.Fatalf("customer A should own client A: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsClientEmail(customerA.Id, clientB.Email); err != nil || owned {
		t.Fatalf("customer A must not own client B: owned=%v err=%v", owned, err)
	}
	if owned, err := scope.OwnsClientEmail(customerA.Id, mixed.Email); err != nil || owned {
		t.Fatalf("mixed-tenant client must be hidden: owned=%v err=%v", owned, err)
	}

	emailsA, err := scope.OwnedClientEmails(customerA.Id)
	if err != nil {
		t.Fatalf("owned client emails for A: %v", err)
	}
	if !slices.Equal(emailsA, []string{clientA.Email}) {
		t.Fatalf("customer A emails = %v, want only %q", emailsA, clientA.Email)
	}
	if email, owned, err := scope.ClientEmailBySubID(customerA.Id, clientB.SubID); err != nil || owned || email != clientB.Email {
		t.Fatalf("cross-tenant subId lookup: email=%q owned=%v err=%v", email, owned, err)
	}
	if allowed, err := scope.CanCreateClientIdentity(customerA.Id, clientA.Email, clientA.SubID); err != nil || !allowed {
		t.Fatalf("customer A should be able to reuse its identity: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := scope.CanCreateClientIdentity(customerA.Id, clientB.Email, clientB.SubID); err != nil || allowed {
		t.Fatalf("customer A must not reuse customer B identity: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := scope.CanCreateClientIdentity(customerA.Id, "new@example.test", clientB.SubID); err != nil || allowed {
		t.Fatalf("customer A must not reuse customer B subId: allowed=%v err=%v", allowed, err)
	}
	if allowed, err := scope.CanCreateClientIdentity(customerA.Id, "new@example.test", "fresh-sub"); err != nil || !allowed {
		t.Fatalf("fresh identity should be available: allowed=%v err=%v", allowed, err)
	}

	if err := (&ClientService{}).SyncInbound(db, inboundA.Id, []model.Client{{
		Email: clientB.Email, SubID: clientB.SubID, Password: "hijacked-secret", Enable: true,
	}}); err == nil {
		t.Fatal("SyncInbound accepted a client identity owned by another tenant")
	}
	var preserved model.ClientRecord
	if err := db.First(&preserved, clientB.Id).Error; err != nil {
		t.Fatalf("reload protected client: %v", err)
	}
	if preserved.Password != "victim-secret" {
		t.Fatalf("cross-tenant SyncInbound changed victim credentials to %q", preserved.Password)
	}
	var inboundALinks int64
	if err := db.Model(&model.ClientInbound{}).Where("inbound_id = ?", inboundA.Id).Count(&inboundALinks).Error; err != nil {
		t.Fatalf("count inbound A links: %v", err)
	}
	if inboundALinks != 2 {
		t.Fatalf("failed SyncInbound changed inbound A links: got %d want 2", inboundALinks)
	}

	responseInbound := &model.Inbound{
		Id: inboundA.Id,
		Settings: `{"clients":[
			{"email":"only-a@example.test","id":"visible"},
			{"email":"only-b@example.test","id":"foreign-secret"},
			{"email":"mixed@example.test","id":"mixed-secret"}
		]}`,
		ClientStats: []xray.ClientTraffic{
			{Email: clientA.Email},
			{Email: clientB.Email},
			{Email: mixed.Email},
		},
		FallbackParent: &model.FallbackParentInfo{MasterId: inboundB.Id},
	}
	if err := scope.FilterInboundClientData(customerA.Id, []*model.Inbound{responseInbound}); err != nil {
		t.Fatalf("filter inbound client data: %v", err)
	}
	var filteredSettings struct {
		Clients []model.Client `json:"clients"`
	}
	if err := json.Unmarshal([]byte(responseInbound.Settings), &filteredSettings); err != nil {
		t.Fatalf("decode filtered inbound settings: %v", err)
	}
	if len(filteredSettings.Clients) != 1 || filteredSettings.Clients[0].Email != clientA.Email {
		t.Fatalf("filtered inbound clients = %+v, want only %q", filteredSettings.Clients, clientA.Email)
	}
	if len(responseInbound.ClientStats) != 1 || responseInbound.ClientStats[0].Email != clientA.Email {
		t.Fatalf("filtered inbound stats = %+v, want only %q", responseInbound.ClientStats, clientA.Email)
	}
	if responseInbound.FallbackParent != nil {
		t.Fatalf("foreign fallback parent was exposed: %+v", responseInbound.FallbackParent)
	}

	listed, err := (&ClientService{}).ListForUser(customerA.Id)
	if err != nil {
		t.Fatalf("list clients for A: %v", err)
	}
	if len(listed) != 1 || listed[0].Email != clientA.Email {
		t.Fatalf("customer A client list = %+v, want only %q", listed, clientA.Email)
	}
}
