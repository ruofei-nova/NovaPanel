package service

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"

	"gorm.io/gorm"
)

// TenantScopeService centralises the ownership checks used by delegated
// customer accounts.  The checks deliberately follow Inbound.UserId instead of
// trusting request payloads: a node reassignment migrates all of that node's
// inbounds to the new owner in one transaction.
type TenantScopeService struct{}

func (s *TenantScopeService) OwnsNode(userID, nodeID int) (bool, error) {
	if userID <= 0 || nodeID <= 0 {
		return false, nil
	}
	var count int64
	err := database.GetDB().Model(&model.Node{}).
		Where("id = ? AND owner_user_id = ?", nodeID, userID).
		Count(&count).Error
	return count == 1, err
}

func (s *TenantScopeService) OwnsInbound(userID, inboundID int) (bool, error) {
	if userID <= 0 || inboundID <= 0 {
		return false, nil
	}
	var count int64
	err := database.GetDB().Model(&model.Inbound{}).
		Where("id = ? AND user_id = ?", inboundID, userID).
		Count(&count).Error
	return count == 1, err
}

func (s *TenantScopeService) OwnsInboundIDs(userID int, inboundIDs []int) (bool, error) {
	if userID <= 0 || len(inboundIDs) == 0 {
		return false, nil
	}
	unique := make(map[int]struct{}, len(inboundIDs))
	for _, id := range inboundIDs {
		if id <= 0 {
			return false, nil
		}
		unique[id] = struct{}{}
	}
	ids := make([]int, 0, len(unique))
	for id := range unique {
		ids = append(ids, id)
	}
	var count int64
	err := database.GetDB().Model(&model.Inbound{}).
		Where("user_id = ? AND id IN ?", userID, ids).
		Count(&count).Error
	return count == int64(len(ids)), err
}

// OwnsClientEmail requires every attachment to stay inside one tenant.  A
// legacy client attached to inbounds owned by two different accounts is hidden
// from both customers until an administrator resolves the ambiguous ownership.
func (s *TenantScopeService) OwnsClientEmail(userID int, email string) (bool, error) {
	email = strings.TrimSpace(email)
	if userID <= 0 || email == "" {
		return false, nil
	}
	var total int64
	if err := database.GetDB().Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Where("clients.email = ?", email).
		Count(&total).Error; err != nil {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	var owned int64
	err := database.GetDB().Table("client_inbounds").
		Joins("JOIN clients ON clients.id = client_inbounds.client_id").
		Joins("JOIN inbounds ON inbounds.id = client_inbounds.inbound_id").
		Where("clients.email = ? AND inbounds.user_id = ?", email, userID).
		Count(&owned).Error
	return owned == total, err
}

func (s *TenantScopeService) OwnsClientEmails(userID int, emails []string) (bool, error) {
	if len(emails) == 0 {
		return false, nil
	}
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			return false, nil
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		owned, err := s.OwnsClientEmail(userID, email)
		if err != nil || !owned {
			return false, err
		}
	}
	return len(seen) > 0, nil
}

func (s *TenantScopeService) ClientEmailBySubID(userID int, subID string) (string, bool, error) {
	subID = strings.TrimSpace(subID)
	if userID <= 0 || subID == "" {
		return "", false, nil
	}
	var client model.ClientRecord
	if err := database.GetDB().Where("sub_id = ?", subID).First(&client).Error; err != nil {
		return "", false, err
	}
	owned, err := s.OwnsClientEmail(userID, client.Email)
	return client.Email, owned, err
}

// CanCreateClientIdentity permits a new identity, or reuse of an identity that
// already belongs exclusively to this tenant. ClientService intentionally
// supports attaching one existing identity to several inbounds, so controller
// create/import paths need this check before they reach that merge behavior.
func (s *TenantScopeService) CanCreateClientIdentity(userID int, email, subID string) (bool, error) {
	email = strings.TrimSpace(email)
	subID = strings.TrimSpace(subID)
	if userID <= 0 || email == "" {
		return false, nil
	}

	var byEmail model.ClientRecord
	err := database.GetDB().Where("email = ?", email).First(&byEmail).Error
	switch {
	case err == nil:
		owned, ownErr := s.OwnsClientEmail(userID, email)
		if ownErr != nil || !owned {
			return false, ownErr
		}
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return false, err
	}

	if subID == "" {
		return true, nil
	}
	var rows []model.ClientRecord
	if err := database.GetDB().Where("sub_id = ?", subID).Find(&rows).Error; err != nil {
		return false, err
	}
	for i := range rows {
		if rows[i].Email != email {
			return false, nil
		}
		owned, ownErr := s.OwnsClientEmail(userID, rows[i].Email)
		if ownErr != nil || !owned {
			return false, ownErr
		}
	}
	return true, nil
}

func (s *TenantScopeService) OwnedClientEmails(userID int) ([]string, error) {
	if userID <= 0 {
		return []string{}, nil
	}
	var emails []string
	err := database.GetDB().Table("clients").
		Select("clients.email").
		Joins("JOIN client_inbounds ci ON ci.client_id = clients.id").
		Joins("JOIN inbounds i ON i.id = ci.inbound_id").
		Group("clients.id, clients.email").
		Having("COUNT(*) = SUM(CASE WHEN i.user_id = ? THEN 1 ELSE 0 END)", userID).
		Order("clients.id ASC").
		Pluck("clients.email", &emails).Error
	if emails == nil {
		emails = []string{}
	}
	return emails, err
}

// FilterInboundClientData removes credentials and traffic rows for identities
// that are not exclusively owned by userID. This is the response-side defense
// for legacy/malformed mixed-tenant attachments: the client list APIs already
// hide them, and inbound APIs must do the same.
func (s *TenantScopeService) FilterInboundClientData(userID int, inbounds []*model.Inbound) error {
	emails, err := s.OwnedClientEmails(userID)
	if err != nil {
		return err
	}
	allowedEmails := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		allowedEmails[email] = struct{}{}
	}
	var ownedInboundIDs []int
	if err := database.GetDB().Model(&model.Inbound{}).
		Where("user_id = ?", userID).
		Pluck("id", &ownedInboundIDs).Error; err != nil {
		return err
	}
	allowedInbounds := make(map[int]struct{}, len(ownedInboundIDs))
	for _, id := range ownedInboundIDs {
		allowedInbounds[id] = struct{}{}
	}

	for _, inbound := range inbounds {
		if inbound == nil {
			continue
		}
		if strings.TrimSpace(inbound.Settings) != "" {
			var values map[string]any
			if err := json.Unmarshal([]byte(inbound.Settings), &values); err != nil {
				return err
			}
			if clients, ok := values["clients"].([]any); ok {
				filtered := make([]any, 0, len(clients))
				for _, value := range clients {
					client, ok := value.(map[string]any)
					if !ok {
						continue
					}
					email, _ := client["email"].(string)
					if _, ok := allowedEmails[email]; ok {
						filtered = append(filtered, value)
					}
				}
				values["clients"] = filtered
				encoded, err := json.Marshal(values)
				if err != nil {
					return err
				}
				inbound.Settings = string(encoded)
			}
		}
		filteredStats := inbound.ClientStats[:0:0]
		for _, traffic := range inbound.ClientStats {
			if _, ok := allowedEmails[traffic.Email]; ok {
				filteredStats = append(filteredStats, traffic)
			}
		}
		inbound.ClientStats = filteredStats
		if inbound.FallbackParent != nil {
			if _, ok := allowedInbounds[inbound.FallbackParent.MasterId]; !ok {
				inbound.FallbackParent = nil
			}
		}
	}
	return nil
}

func (s *TenantScopeService) OwnsHostGroup(userID int, groupID string) (bool, error) {
	groupID = strings.TrimSpace(groupID)
	if userID <= 0 || groupID == "" {
		return false, nil
	}
	var total int64
	if err := database.GetDB().Model(&model.Host{}).
		Where("group_id = ?", groupID).
		Count(&total).Error; err != nil {
		return false, err
	}
	if total == 0 {
		return false, nil
	}
	var owned int64
	err := database.GetDB().Table("hosts").
		Joins("JOIN inbounds ON inbounds.id = hosts.inbound_id").
		Where("hosts.group_id = ? AND inbounds.user_id = ?", groupID, userID).
		Count(&owned).Error
	return owned == total, err
}

func (s *TenantScopeService) OwnsHostGroups(userID int, groupIDs []string) (bool, error) {
	if len(groupIDs) == 0 {
		return false, nil
	}
	seen := make(map[string]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			return false, nil
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		owned, err := s.OwnsHostGroup(userID, groupID)
		if err != nil || !owned {
			return false, err
		}
	}
	return len(seen) > 0, nil
}
