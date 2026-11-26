package repository

import (
	"strings"
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
	UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error
	UpdateEmail(userID uint64, email string, verified bool) error
	UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error
	IncrementTokenVersion(userID uint64) error
	UpdateLastLogin(userID uint64, t time.Time) error
	SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error)
	FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error)
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

func (r *userRepository) UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error {
	updates := map[string]interface{}{
		"nickname":         nickname,
		"avatar":           avatar,
		"bio":              bio,
		"favorites_public": favoritesPublic,
	}
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *userRepository) UpdateEmail(userID uint64, email string, verified bool) error {
	return r.db.Model(&model.SysUser{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"email":          email,
		"email_verified": verified,
	}).Error
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

func (r *userRepository) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	tx := r.db.Model(&model.SysUser{}).
		Select("id, username, nickname, avatar").
		Where("status = ? AND username <> ?", model.UserStatusActive, "system")
	if excludeUserID > 0 {
		tx = tx.Where("id <> ?", excludeUserID)
	}
	kw := strings.TrimSpace(keyword)
	if kw != "" {
		like := "%" + kw + "%"
		tx = tx.Where("(username LIKE ? OR nickname LIKE ?)", like, like)
	}
	var users []model.UserBrief
	if err := tx.Order("id ASC").Limit(limit).Scan(&users).Error; err != nil {
		return nil, err
	}
	if users == nil {
		users = []model.UserBrief{}
	}
	return users, nil
}

func (r *userRepository) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	result := make(map[uint64]model.UserBrief)
	if len(ids) == 0 {
		return result, nil
	}
	var users []model.UserBrief
	if err := r.db.Model(&model.SysUser{}).
		Select("id, username, nickname, avatar").
		Where("id IN ?", ids).
		Scan(&users).Error; err != nil {
		return nil, err
	}
	for _, u := range users {
		result[u.ID] = u
	}
	return result, nil
}
