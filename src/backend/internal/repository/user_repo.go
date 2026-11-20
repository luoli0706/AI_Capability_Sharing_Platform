package repository

import (
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	CreateWithRole(user *model.SysUser, roleCode string) error
	FindByID(id uint64) (*model.SysUser, error)
	FindByUsername(username string) (*model.SysUser, error)
	FindByEmail(email string) (*model.SysUser, error)
	GetUserRoles(userID uint64) ([]string, error)
	UpdateProfile(userID uint64, nickname string, email *string, avatar, bio string) error
	UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error
	IncrementTokenVersion(userID uint64) error
	UpdateLastLogin(userID uint64, t time.Time) error
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) CreateWithRole(user *model.SysUser, roleCode string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		var role model.SysRole
		if err := tx.Where("role_code = ?", roleCode).First(&role).Error; err != nil {
			return err
		}

		userRole := model.SysUserRole{
			UserID: user.ID,
			RoleID: role.ID,
		}
		return tx.Create(&userRole).Error
	})
}

func (r *userRepository) FindByID(id uint64) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByUsername(username string) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(email string) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetUserRoles(userID uint64) ([]string, error) {
	var roles []string
	err := r.db.Table("sys_role").
		Select("sys_role.role_code").
		Joins("INNER JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Where("sys_user_role.user_id = ? AND sys_role.deleted_at IS NULL", userID).
		Pluck("sys_role.role_code", &roles).Error
	if err != nil {
		return nil, err
	}
	if roles == nil {
		roles = []string{}
	}
	return roles, nil
}

func (r *userRepository) UpdateProfile(userID uint64, nickname string, email *string, avatar, bio string) error {
	updates := map[string]interface{}{
		"nickname": nickname,
		"email":    email,
		"avatar":   avatar,
		"bio":      bio,
	}
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *userRepository) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"password_hash": passwordHash,
		"token_version": gorm.Expr("token_version + 1"),
	}).Error
}

func (r *userRepository) IncrementTokenVersion(userID uint64) error {
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).
		UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error
}

func (r *userRepository) UpdateLastLogin(userID uint64, t time.Time) error {
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).
		UpdateColumn("last_login_at", t).Error
}
