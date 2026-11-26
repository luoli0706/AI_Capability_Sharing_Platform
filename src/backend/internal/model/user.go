package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	RoleSuperAdmin  = "super_admin"
	RoleDomainAdmin = "domain_admin"
	RoleDeveloper   = "developer"

	UserStatusDisabled = 0
	UserStatusActive   = 1
)

type SysUser struct {
	ID           uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string         `gorm:"type:varchar(64);uniqueIndex:uk_username;not null" json:"username"`
	PasswordHash string         `gorm:"type:varchar(255);not null" json:"-"`
	Nickname        string         `gorm:"type:varchar(64);not null;default:''" json:"nickname"`
	Email           *string        `gorm:"type:varchar(128);uniqueIndex:uk_email" json:"email"`
	EmailVerified   bool           `gorm:"type:tinyint(1);not null;default:0" json:"email_verified"`
	Avatar          string         `gorm:"type:mediumtext;not null" json:"avatar"`
	Bio             string         `gorm:"type:varchar(255);not null;default:''" json:"bio"`
	FavoritesPublic bool           `gorm:"default:false;not null" json:"favorites_public"`
	Status          int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
	TokenVersion int            `gorm:"type:int;not null;default:1" json:"-"`
	LastLoginAt  *time.Time     `gorm:"type:datetime" json:"last_login_at,omitempty"`
	CreatedAt    time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SysUser) TableName() string {
	return "sys_user"
}

type SysRole struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleCode    string         `gorm:"type:varchar(32);uniqueIndex:uk_role_code;not null" json:"role_code"`
	RoleName    string         `gorm:"type:varchar(64);not null" json:"role_name"`
	Description string         `gorm:"type:varchar(255);not null;default:''" json:"description"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (SysRole) TableName() string {
	return "sys_role"
}

type SysUserRole struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_user_role;not null" json:"user_id"`
	RoleID    uint64    `gorm:"uniqueIndex:uk_user_role;not null" json:"role_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (SysUserRole) TableName() string {
	return "sys_user_role"
}

// UserInfo is the public user representation returned in auth and profile responses.
type UserInfo struct {
	ID              uint64    `json:"id"`
	Username        string    `json:"username"`
	Nickname        string    `json:"nickname"`
	Email           string    `json:"email"`
	EmailVerified   bool      `json:"email_verified"`
	Avatar          string    `json:"avatar"`
	Bio             string    `json:"bio"`
	FavoritesPublic bool      `json:"favorites_public"`
	Status          int8      `json:"status,omitempty"`
	Roles           []string  `json:"roles"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
}

// UserBrief is a compact user summary for lists, uploaders, peers, and interactions.
type UserBrief struct {
	ID       uint64 `json:"id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

// ToUserBrief converts SysUser to UserBrief.
func ToUserBrief(u *SysUser) UserBrief {
	return UserBrief{
		ID:       u.ID,
		Username: u.Username,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
	}
}

// PublicUserProfile represents GET /api/v1/users/:id response data.
type PublicUserProfile struct {
	ID              uint64    `json:"id"`
	Username        string    `json:"username"`
	Nickname        string    `json:"nickname"`
	Avatar          string    `json:"avatar"`
	Bio             string    `json:"bio"`
	FavoritesPublic bool      `json:"favorites_public"`
	CreatedAt       time.Time `json:"created_at"`
}

// UserFavoriteItem represents a favorited KB document entry.
type UserFavoriteItem struct {
	ID         uint64    `json:"id"`
	KBAssetID  uint64    `json:"kb_asset_id"`
	DocID      uint64    `json:"doc_id"`
	DomainID   uint64    `json:"domain_id"`
	DomainName string    `json:"domain_name"`
	KBName     string    `json:"kb_name"`
	FileName   string    `json:"file_name"`
	FolderPath string    `json:"folder_path"`
	FileExt    string    `json:"file_ext"`
	IsFolder   bool      `json:"is_folder"`
	CreatedAt  time.Time `json:"created_at"`

	DomainVisibility string `json:"-"`
	AssetOwnerID     uint64 `json:"-"`
	AssetVisibility  string `json:"-"`
	AssetStatus      string `json:"-"`
}

// ToUserInfo converts SysUser to UserInfo, turning nil Email into "".
func ToUserInfo(u *SysUser, roles []string) UserInfo {
	email := ""
	if u.Email != nil {
		email = *u.Email
	}
	if roles == nil {
		roles = []string{}
	}
	return UserInfo{
		ID:              u.ID,
		Username:        u.Username,
		Nickname:        u.Nickname,
		Email:           email,
		EmailVerified:   u.EmailVerified,
		Avatar:          u.Avatar,
		Bio:             u.Bio,
		FavoritesPublic: u.FavoritesPublic,
		Status:          u.Status,
		Roles:           roles,
		CreatedAt:       u.CreatedAt,
	}
}

// RegisterRequest represents POST /api/v1/auth/register payload.
type RegisterRequest struct {
	Username  string `json:"username" validate:"required,min=3,max=64"`
	Password  string `json:"password" validate:"required,min=8,max=64"`
	Nickname  string `json:"nickname" validate:"max=64"`
	Email     string `json:"email" validate:"required,email,max=128"`
	EmailCode string `json:"email_code" validate:"required,len=6"`
}

// LoginRequest represents POST /api/v1/auth/login payload.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents POST /api/v1/auth/login response data.
type LoginResponse struct {
	Token     string   `json:"token"`
	ExpiresIn int64    `json:"expires_in"`
	User      UserInfo `json:"user"`
}

// UpdateProfileRequest represents PUT /api/v1/user/profile payload.
type UpdateProfileRequest struct {
	Nickname        string  `json:"nickname" validate:"max=64"`
	Avatar          *string `json:"avatar"`
	Bio             *string `json:"bio" validate:"omitempty,max=255"`
	FavoritesPublic *bool   `json:"favorites_public"`
}

// SendEmailCodeRequest represents POST /api/v1/auth/email/send-code and POST /api/v1/user/email/send-code payload.
type SendEmailCodeRequest struct {
	Email string `json:"email" validate:"required,email,max=128"`
}

// BindEmailRequest represents PUT /api/v1/user/email payload.
type BindEmailRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	Email           string `json:"email" validate:"required,email,max=128"`
	EmailCode       string `json:"email_code" validate:"required,len=6"`
}

// UpdatePasswordRequest represents PUT /api/v1/user/password payload.
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6,max=64"`
}

// SendResetPasswordCodeRequest represents POST /api/v1/auth/password/send-reset-code payload.
type SendResetPasswordCodeRequest struct {
	Account string `json:"account" validate:"required,min=1,max=128"`
}

// ResetPasswordRequest represents POST /api/v1/auth/password/reset payload.
type ResetPasswordRequest struct {
	Account   string `json:"account" validate:"required,min=1,max=128"`
	EmailCode string `json:"email_code" validate:"required,len=6"`
}
