package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

type FallbackService struct{}

// FallbackInput is the payload shape POSTed by the inbound form.
type FallbackInput struct {
	ChildId   int    `json:"childId"`
	Name      string `json:"name"`
	Alpn      string `json:"alpn"`
	Path      string `json:"path"`
	Dest      string `json:"dest"`
	Xver      int    `json:"xver"`
	SortOrder int    `json:"sortOrder"`
}

// GetByMaster returns every fallback rule attached to the master inbound.
func (s *FallbackService) GetByMaster(masterId int) ([]model.InboundFallback, error) {
	var rows []model.InboundFallback
	err := database.GetDB().
		Where("master_id = ?", masterId).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// GetParentForChild finds the first fallback rule that points at childId.
// Used by client-link generation: when a child inbound is attached as a
// fallback, its client links should advertise the master's address+port
// and TLS instead of the child's loopback listen.
func (s *FallbackService) GetParentForChild(childId int) (*model.InboundFallback, error) {
	var row model.InboundFallback
	err := database.GetDB().
		Table("inbound_fallbacks AS fallbacks").
		Select("fallbacks.*").
		Joins("JOIN inbounds AS child ON child.id = fallbacks.child_id").
		Joins("JOIN inbounds AS master ON master.id = fallbacks.master_id").
		Where("fallbacks.child_id = ? AND child.user_id = master.user_id", childId).
		Order("fallbacks.sort_order ASC, fallbacks.id ASC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// SetByMaster replaces the master's entire fallback list atomically.
func (s *FallbackService) SetByMaster(masterId int, items []FallbackInput) error {
	db := database.GetDB()
	return db.Transaction(func(tx *gorm.DB) error {
		var master model.Inbound
		if err := tx.Select("id", "user_id", "node_id").First(&master, masterId).Error; err != nil {
			return err
		}
		if err := tx.Where("master_id = ?", masterId).Delete(&model.InboundFallback{}).Error; err != nil {
			return err
		}
		for i, c := range items {
			childId := c.ChildId
			if childId == masterId {
				childId = 0
			}
			if childId <= 0 && strings.TrimSpace(c.Dest) == "" {
				continue
			}
			if childId > 0 {
				var child model.Inbound
				if err := tx.Select("id", "user_id", "node_id").First(&child, childId).Error; err != nil {
					return common.NewError("fallback inbound not found")
				}
				if child.UserId != master.UserId || !sameNodeID(child.NodeID, master.NodeID) {
					return common.NewError("fallback inbound must belong to the same tenant and node")
				}
			}
			row := model.InboundFallback{
				MasterId:  masterId,
				ChildId:   childId,
				Name:      c.Name,
				Alpn:      c.Alpn,
				Path:      c.Path,
				Dest:      c.Dest,
				Xver:      c.Xver,
				SortOrder: c.SortOrder,
			}
			if row.SortOrder == 0 {
				row.SortOrder = i
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func sameNodeID(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (s *FallbackService) BuildFallbacksJSON(tx *gorm.DB, masterId int) ([]map[string]any, error) {
	if tx == nil {
		tx = database.GetDB()
	}
	var rows []model.InboundFallback
	err := tx.Where("master_id = ?", masterId).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	var master model.Inbound
	if err := tx.Select("id", "user_id", "node_id").First(&master, masterId).Error; err != nil {
		return nil, err
	}

	childIds := make([]int, 0, len(rows))
	for i := range rows {
		childIds = append(childIds, rows[i].ChildId)
	}
	var children []model.Inbound
	if err := tx.Where("id IN ? AND user_id = ?", childIds, master.UserId).Find(&children).Error; err != nil {
		return nil, err
	}
	byId := make(map[int]*model.Inbound, len(children))
	for i := range children {
		if sameNodeID(children[i].NodeID, master.NodeID) {
			byId[children[i].Id] = &children[i]
		}
	}

	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		dest := strings.TrimSpace(r.Dest)
		if dest == "" {
			child, ok := byId[r.ChildId]
			if !ok {
				continue
			}
			listen := strings.TrimSpace(child.Listen)
			if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "::0" {
				listen = "127.0.0.1"
			}
			dest = fmt.Sprintf("%s:%d", listen, child.Port)
		}
		entry := map[string]any{
			"dest": dest,
		}
		if r.Name != "" {
			entry["name"] = r.Name
		}
		if r.Alpn != "" {
			entry["alpn"] = r.Alpn
		}
		if r.Path != "" {
			entry["path"] = r.Path
		}
		if r.Xver > 0 {
			entry["xver"] = r.Xver
		}
		out = append(out, entry)
	}
	return out, nil
}
