package service

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestFallbacksCannotCrossTenantOrNode(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	userA := &model.User{Username: "fallback-a", Role: "customer", Enabled: true}
	userB := &model.User{Username: "fallback-b", Role: "customer", Enabled: true}
	if err := db.Create(userA).Error; err != nil {
		t.Fatalf("create user A: %v", err)
	}
	if err := db.Create(userB).Error; err != nil {
		t.Fatalf("create user B: %v", err)
	}
	nodeA := &model.Node{Name: "fallback-node-a", Address: "a.example.test", Port: 2053}
	nodeB := &model.Node{Name: "fallback-node-b", Address: "b.example.test", Port: 2053}
	if err := db.Create(nodeA).Error; err != nil {
		t.Fatalf("create node A: %v", err)
	}
	if err := db.Create(nodeB).Error; err != nil {
		t.Fatalf("create node B: %v", err)
	}
	master := &model.Inbound{UserId: userA.Id, NodeID: &nodeA.Id, Tag: "fallback-master", Port: 443, Protocol: model.VLESS}
	childA := &model.Inbound{UserId: userA.Id, NodeID: &nodeA.Id, Tag: "fallback-child-a", Port: 8080, Protocol: model.VLESS}
	childOtherNode := &model.Inbound{UserId: userA.Id, NodeID: &nodeB.Id, Tag: "fallback-child-node", Port: 8081, Protocol: model.VLESS}
	childB := &model.Inbound{UserId: userB.Id, NodeID: &nodeB.Id, Tag: "fallback-child-b", Port: 8082, Protocol: model.VLESS}
	for _, inbound := range []*model.Inbound{master, childA, childOtherNode, childB} {
		if err := db.Create(inbound).Error; err != nil {
			t.Fatalf("create inbound %q: %v", inbound.Tag, err)
		}
	}

	svc := &FallbackService{}
	if err := svc.SetByMaster(master.Id, []FallbackInput{{ChildId: childA.Id, Path: "/owned"}}); err != nil {
		t.Fatalf("set same-tenant fallback: %v", err)
	}
	if err := svc.SetByMaster(master.Id, []FallbackInput{{ChildId: childB.Id, Path: "/foreign"}}); err == nil {
		t.Fatal("cross-tenant fallback was accepted")
	}
	if err := svc.SetByMaster(master.Id, []FallbackInput{{ChildId: childOtherNode.Id, Path: "/other-node"}}); err == nil {
		t.Fatal("cross-node fallback was accepted")
	}

	rows, err := svc.GetByMaster(master.Id)
	if err != nil {
		t.Fatalf("reload fallbacks: %v", err)
	}
	if len(rows) != 1 || rows[0].ChildId != childA.Id {
		t.Fatalf("failed replacement changed stored fallback: %+v", rows)
	}
	parent, err := svc.GetParentForChild(childA.Id)
	if err != nil || parent == nil || parent.MasterId != master.Id {
		t.Fatalf("same-tenant parent lookup failed: parent=%+v err=%v", parent, err)
	}

	cross := &model.InboundFallback{MasterId: master.Id, ChildId: childB.Id, Path: "/legacy-cross"}
	if err := db.Create(cross).Error; err != nil {
		t.Fatalf("seed legacy cross-tenant fallback: %v", err)
	}
	parent, err = svc.GetParentForChild(childB.Id)
	if err != nil {
		t.Fatalf("cross-tenant parent lookup: %v", err)
	}
	if parent != nil {
		t.Fatalf("cross-tenant parent was exposed: %+v", parent)
	}
}
