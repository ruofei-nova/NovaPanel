package service

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"

	"gorm.io/gorm"
)

type GroupSummary struct {
	Name        string `json:"name"`
	ClientCount int    `json:"clientCount"`
	TrafficUsed int64  `json:"trafficUsed"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
}

func (s *ClientService) ListGroups() ([]GroupSummary, error) {
	db := database.GetDB()
	// email is unique in both clients and client_traffics, so the LEFT JOIN
	// never double-counts a client's traffic.
	var derived []GroupSummary
	if err := db.Table("clients AS c").
		Select("c.group_name AS name, COUNT(*) AS client_count, COALESCE(SUM(ct.up + ct.down), 0) AS traffic_used, COALESCE(SUM(ct.up), 0) AS up, COALESCE(SUM(ct.down), 0) AS down").
		Joins("LEFT JOIN client_traffics ct ON ct.email = c.email").
		Where("c.group_name <> ''").
		Group("c.group_name").
		Scan(&derived).Error; err != nil {
		return nil, err
	}
	var stored []model.ClientGroup
	if err := db.Find(&stored).Error; err != nil {
		return nil, err
	}
	type groupAgg struct {
		count int
		up    int64
		down  int64
	}
	baseUp := make(map[string]int64, len(stored))
	baseDown := make(map[string]int64, len(stored))
	merged := make(map[string]groupAgg, len(derived)+len(stored))
	for _, g := range stored {
		merged[g.Name] = groupAgg{}
		baseUp[g.Name] = g.ResetUp
		baseDown[g.Name] = g.ResetDown
	}
	for _, g := range derived {
		merged[g.Name] = groupAgg{count: g.ClientCount, up: g.Up, down: g.Down}
	}
	out := make([]GroupSummary, 0, len(merged))
	for name, agg := range merged {
		up := max(agg.up-baseUp[name], 0)
		down := max(agg.down-baseDown[name], 0)
		out = append(out, GroupSummary{Name: name, ClientCount: agg.count, TrafficUsed: up + down, Up: up, Down: down})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// ListGroupsForUser derives groups only from clients whose inbound attachments
// are exclusively owned by the delegated account. Empty global group rows are
// intentionally omitted because the legacy client_groups table has no owner
// column.
func (s *ClientService) ListGroupsForUser(userID int) ([]GroupSummary, error) {
	clients, err := s.ListForUser(userID)
	if err != nil {
		return nil, err
	}
	merged := make(map[string]*GroupSummary)
	for _, client := range clients {
		name := strings.TrimSpace(client.Group)
		if name == "" {
			continue
		}
		group := merged[name]
		if group == nil {
			group = &GroupSummary{Name: name}
			merged[name] = group
		}
		group.ClientCount++
		if client.Traffic != nil {
			group.Up += client.Traffic.Up
			group.Down += client.Traffic.Down
			group.TrafficUsed += client.Traffic.Up + client.Traffic.Down
		}
	}
	var stored []model.ClientGroup
	if err := database.GetDB().Where("owner_user_id = ?", userID).Find(&stored).Error; err != nil {
		return nil, err
	}
	for _, group := range stored {
		if _, ok := merged[group.Name]; !ok {
			merged[group.Name] = &GroupSummary{Name: group.Name}
		}
	}
	out := make([]GroupSummary, 0, len(merged))
	for _, group := range merged {
		out = append(out, *group)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// adjustGroupBaselinesForRemovedTraffic shifts group baselines down by the clients'
// current counters so ListGroups totals survive a traffic reset or client delete (#5675).
func adjustGroupBaselinesForRemovedTraffic(tx *gorm.DB, emails []string) error {
	if len(emails) == 0 {
		return nil
	}
	type groupDelta struct {
		Name string
		Up   int64
		Down int64
	}
	totals := make(map[string]*groupDelta)
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var part []groupDelta
		if err := tx.Table("clients AS c").
			Select("c.group_name AS name, COALESCE(SUM(ct.up), 0) AS up, COALESCE(SUM(ct.down), 0) AS down").
			Joins("JOIN client_traffics ct ON ct.email = c.email").
			Where("c.group_name <> '' AND c.email IN ?", batch).
			Group("c.group_name").
			Scan(&part).Error; err != nil {
			return err
		}
		for i := range part {
			if agg, ok := totals[part[i].Name]; ok {
				agg.Up += part[i].Up
				agg.Down += part[i].Down
			} else {
				totals[part[i].Name] = &part[i]
			}
		}
	}
	for name, d := range totals {
		if d.Up == 0 && d.Down == 0 {
			continue
		}
		res := tx.Model(&model.ClientGroup{}).Where("name = ?", name).Updates(map[string]any{
			"reset_up":   gorm.Expr("reset_up - ?", d.Up),
			"reset_down": gorm.Expr("reset_down - ?", d.Down),
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			if err := tx.Create(&model.ClientGroup{Name: name, ResetUp: -d.Up, ResetDown: -d.Down}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ClientService) EmailsByGroup(name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{}, nil
	}
	db := database.GetDB()
	var emails []string
	if err := db.Model(&model.ClientRecord{}).
		Where("group_name = ?", name).
		Order("email ASC").
		Pluck("email", &emails).Error; err != nil {
		return nil, err
	}
	if emails == nil {
		emails = []string{}
	}
	return emails, nil
}

func (s *ClientService) EmailsByGroupForUser(userID int, name string) ([]string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{}, nil
	}
	owned, err := (&TenantScopeService{}).OwnedClientEmails(userID)
	if err != nil || len(owned) == 0 {
		return []string{}, err
	}
	var emails []string
	err = database.GetDB().Model(&model.ClientRecord{}).
		Where("group_name = ? AND email IN ?", name, owned).
		Order("email ASC").
		Pluck("email", &emails).Error
	if emails == nil {
		emails = []string{}
	}
	return emails, err
}

func (s *ClientService) AddToGroupForUser(userID int, emails []string, group string) (int, error) {
	owned, err := (&TenantScopeService{}).OwnsClientEmails(userID, emails)
	if err != nil {
		return 0, err
	}
	if !owned {
		return 0, common.NewError("client not found")
	}
	group = strings.TrimSpace(group)
	if group != "" {
		var stored model.ClientGroup
		err := database.GetDB().Where("name = ?", group).First(&stored).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			owner := userID
			if err := database.GetDB().Create(&model.ClientGroup{
				OwnerUserID: &owner,
				Name:        group,
			}).Error; err != nil {
				return 0, err
			}
		case err != nil:
			return 0, err
		case stored.OwnerUserID != nil && *stored.OwnerUserID != userID:
			return 0, common.NewError("group already exists")
		}
	}
	return s.AddToGroup(emails, group)
}

func (s *ClientService) ResetGroupTraffic(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return common.NewError("group name is required")
	}
	db := database.GetDB()
	var agg struct {
		Up   int64
		Down int64
	}
	if err := db.Table("clients AS c").
		Select("COALESCE(SUM(ct.up), 0) AS up, COALESCE(SUM(ct.down), 0) AS down").
		Joins("LEFT JOIN client_traffics ct ON ct.email = c.email").
		Where("c.group_name = ?", name).
		Scan(&agg).Error; err != nil {
		return err
	}
	var count int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return db.Create(&model.ClientGroup{Name: name, ResetUp: agg.Up, ResetDown: agg.Down}).Error
	}
	return db.Model(&model.ClientGroup{}).Where("name = ?", name).
		Updates(map[string]any{"reset_up": agg.Up, "reset_down": agg.Down}).Error
}

func (s *ClientService) CreateGroup(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return common.NewError("group name is required")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.ClientGroup{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return common.NewError("group already exists")
	}
	return db.Create(&model.ClientGroup{Name: name}).Error
}

func (s *ClientService) CreateGroupForUser(userID int, name string) error {
	name = strings.TrimSpace(name)
	if userID <= 0 || name == "" {
		return common.NewError("group name is required")
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.ClientGroup{}).
		Where("owner_user_id = ? AND name = ?", userID, name).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return common.NewError("group already exists")
	}
	owner := userID
	return db.Create(&model.ClientGroup{OwnerUserID: &owner, Name: name}).Error
}

func (s *ClientService) DeleteStoredGroupForUser(userID int, name string) error {
	return database.GetDB().Where("owner_user_id = ? AND name = ?", userID, strings.TrimSpace(name)).
		Delete(&model.ClientGroup{}).Error
}

func (s *ClientService) RenameStoredGroupForUser(userID int, oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return common.NewError("group name is required")
	}
	return database.GetDB().Model(&model.ClientGroup{}).
		Where("owner_user_id = ? AND name = ?", userID, oldName).
		Update("name", newName).Error
}

func (s *ClientService) HasStoredGroupForUser(userID int, name string) (bool, error) {
	var count int64
	err := database.GetDB().Model(&model.ClientGroup{}).
		Where("owner_user_id = ? AND name = ?", userID, strings.TrimSpace(name)).
		Count(&count).Error
	return count > 0, err
}

func (s *ClientService) RenameGroup(oldName, newName string) (int, error) {
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" {
		return 0, common.NewError("old group name is required")
	}
	if newName == "" {
		return 0, common.NewError("new group name is required")
	}
	if oldName == newName {
		return 0, nil
	}
	return s.replaceGroupValue(oldName, newName)
}

func (s *ClientService) DeleteGroup(name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, common.NewError("group name is required")
	}
	return s.replaceGroupValue(name, "")
}

func (s *ClientService) RemoveFromGroup(emails []string) (int, error) {
	return s.AddToGroup(emails, "")
}

func (s *ClientService) AddToGroup(emails []string, group string) (int, error) {
	group = strings.TrimSpace(group)
	if len(emails) == 0 {
		return 0, nil
	}
	db := database.GetDB()

	if group != "" {
		var exists int64
		if err := db.Model(&model.ClientGroup{}).Where("name = ?", group).Count(&exists).Error; err != nil {
			return 0, err
		}
		if exists == 0 {
			var derived int64
			if err := db.Model(&model.ClientRecord{}).Where("group_name = ?", group).Count(&derived).Error; err != nil {
				return 0, err
			}
			if derived == 0 {
				if err := db.Create(&model.ClientGroup{Name: group}).Error; err != nil {
					return 0, err
				}
			}
		}
	}

	var records []model.ClientRecord
	for _, batch := range chunkStrings(emails, sqlInChunk) {
		var rows []model.ClientRecord
		if err := db.Where("email IN ?", batch).Find(&rows).Error; err != nil {
			return 0, err
		}
		records = append(records, rows...)
	}
	if len(records) == 0 {
		return 0, nil
	}
	affectedEmails := make([]string, 0, len(records))
	for _, r := range records {
		affectedEmails = append(affectedEmails, r.Email)
	}

	tx := db.Begin()
	for _, batch := range chunkStrings(affectedEmails, sqlInChunk) {
		if err := tx.Model(&model.ClientRecord{}).
			Where("email IN ?", batch).
			UpdateColumn("group_name", group).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	var inboundIDs []int
	inboundIDSeen := make(map[int]struct{})
	for _, batch := range chunkStrings(affectedEmails, sqlInChunk) {
		var ids []int
		if err := tx.Table("client_inbounds").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ?", batch).
			Distinct("client_inbounds.inbound_id").
			Pluck("inbound_id", &ids).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		for _, id := range ids {
			if _, ok := inboundIDSeen[id]; !ok {
				inboundIDSeen[id] = struct{}{}
				inboundIDs = append(inboundIDs, id)
			}
		}
	}

	emailSet := make(map[string]struct{}, len(affectedEmails))
	for _, e := range affectedEmails {
		emailSet[e] = struct{}{}
	}

	for _, ibID := range inboundIDs {
		var ib model.Inbound
		if err := tx.First(&ib, ibID).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		modified := false
		for i := range clients {
			cm, ok := clients[i].(map[string]any)
			if !ok {
				continue
			}
			email, _ := cm["email"].(string)
			if _, hit := emailSet[email]; !hit {
				continue
			}
			if group == "" {
				delete(cm, "group")
			} else {
				cm["group"] = group
			}
			clients[i] = cm
			modified = true
		}
		if modified {
			settings["clients"] = clients
			newSettings, err := json.Marshal(settings)
			if err != nil {
				continue
			}
			ib.Settings = string(newSettings)
			if err := tx.Save(&ib).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return len(records), nil
}

func (s *ClientService) replaceGroupValue(oldName, newName string) (int, error) {
	db := database.GetDB()
	if newName == "" {
		if err := db.Where("name = ?", oldName).Delete(&model.ClientGroup{}).Error; err != nil {
			return 0, err
		}
	} else {
		if err := db.Model(&model.ClientGroup{}).Where("name = ?", oldName).Update("name", newName).Error; err != nil {
			return 0, err
		}
	}
	var records []model.ClientRecord
	if err := db.Where("group_name = ?", oldName).Find(&records).Error; err != nil {
		return 0, err
	}
	if len(records) == 0 {
		return 0, nil
	}
	affectedEmails := make([]string, 0, len(records))
	for _, r := range records {
		affectedEmails = append(affectedEmails, r.Email)
	}

	tx := db.Begin()
	if err := tx.Model(&model.ClientRecord{}).
		Where("group_name = ?", oldName).
		UpdateColumn("group_name", newName).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	var inboundIDs []int
	inboundIDSeen := make(map[int]struct{})
	for _, batch := range chunkStrings(affectedEmails, sqlInChunk) {
		var ids []int
		if err := tx.Table("client_inbounds").
			Joins("JOIN clients ON clients.id = client_inbounds.client_id").
			Where("clients.email IN ?", batch).
			Distinct("client_inbounds.inbound_id").
			Pluck("inbound_id", &ids).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		for _, id := range ids {
			if _, ok := inboundIDSeen[id]; !ok {
				inboundIDSeen[id] = struct{}{}
				inboundIDs = append(inboundIDs, id)
			}
		}
	}

	for _, ibID := range inboundIDs {
		var ib model.Inbound
		if err := tx.First(&ib, ibID).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		var settings map[string]any
		if err := json.Unmarshal([]byte(ib.Settings), &settings); err != nil {
			continue
		}
		clients, ok := settings["clients"].([]any)
		if !ok {
			continue
		}
		modified := false
		for i := range clients {
			cm, ok := clients[i].(map[string]any)
			if !ok {
				continue
			}
			if g, ok := cm["group"].(string); ok && g == oldName {
				if newName == "" {
					delete(cm, "group")
				} else {
					cm["group"] = newName
				}
				clients[i] = cm
				modified = true
			}
		}
		if modified {
			settings["clients"] = clients
			newSettings, err := json.Marshal(settings)
			if err != nil {
				continue
			}
			ib.Settings = string(newSettings)
			if err := tx.Save(&ib).Error; err != nil {
				tx.Rollback()
				return 0, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return 0, err
	}
	return len(records), nil
}
