package panel

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"

	"gorm.io/gorm"
)

const (
	UserRoleAdmin    = "admin"
	UserRoleCustomer = "customer"
)

type CustomerService struct{}

func (s *CustomerService) List() ([]model.User, error) {
	var users []model.User
	err := database.GetDB().Where("role = ?", UserRoleCustomer).Order("id asc").Find(&users).Error
	return users, err
}

func randomPassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *CustomerService) Create(username, password string) (*model.User, string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, "", errors.New("username is required")
	}
	if password == "" {
		var err error
		password, err = randomPassword()
		if err != nil {
			return nil, "", err
		}
	}
	if len(password) < 12 {
		return nil, "", errors.New("password must be at least 12 characters")
	}
	hashed, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return nil, "", err
	}
	user := &model.User{
		Username: username,
		Password: hashed,
		Role:     UserRoleCustomer,
		Enabled:  true,
	}
	if err := database.GetDB().Create(user).Error; err != nil {
		return nil, "", err
	}
	return user, password, nil
}

func (s *CustomerService) SetEnabled(id int, enabled bool) error {
	result := database.GetDB().Model(&model.User{}).
		Where("id = ? AND role = ?", id, UserRoleCustomer).
		Updates(map[string]any{"enabled": enabled, "login_epoch": gorm.Expr("login_epoch + 1")})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *CustomerService) ResetPassword(id int) (string, error) {
	password, err := randomPassword()
	if err != nil {
		return "", err
	}
	hashed, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return "", err
	}
	result := database.GetDB().Model(&model.User{}).
		Where("id = ? AND role = ?", id, UserRoleCustomer).
		Updates(map[string]any{"password": hashed, "login_epoch": gorm.Expr("login_epoch + 1")})
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return "", gorm.ErrRecordNotFound
	}
	return password, nil
}

func (s *CustomerService) Delete(id int) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var admin model.User
		if err := tx.Where("role = ?", UserRoleAdmin).Order("id asc").First(&admin).Error; err != nil {
			if fallbackErr := tx.Order("id asc").First(&admin).Error; fallbackErr != nil {
				return fallbackErr
			}
		}
		// Recover every inbound owned by the departing customer, including
		// legacy or manually-created rows that are not attached to a node.
		if err := tx.Model(&model.Inbound{}).Where("user_id = ?", id).
			Update("user_id", admin.Id).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.Node{}).Where("owner_user_id = ?", id).
			Update("owner_user_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Where("owner_user_id = ?", id).Delete(&model.ClientGroup{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND role = ?", id, UserRoleCustomer).Delete(&model.User{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}
