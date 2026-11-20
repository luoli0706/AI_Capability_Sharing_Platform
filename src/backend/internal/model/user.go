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
	Nickname     string         `gorm:"type:varchar(64);not null;default:''" json:"nickname"`
	Email        string         `gorm:"type:varchar(128);uniqueIndex:uk_email;not null" json:"email"`
	Avatar       string         `gorm:"type:varchar(255);not null;default:''" json:"avatar"`
	Status       int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
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
	ID        uint64    `json:"id"`
	Username  string    `json:"username"`
	Nickname  string    `json:"nickname"`
	Email     string    `json:"email"`
	Avatar    string    `json:"avatar"`
	Status    int8      `json:"status,omitempty"`
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// RegisterRequest represents POST /api/v1/auth/register payload.
type RegisterRequest struct {
	Username string `json:"username" validate:"required,min=3,max=64"`
	Password string `json:"password" validate:"required,min=6,max=64"`
	Nickname string `json:"nickname" validate:"max=64"`
	Email    string `json:"email" validate:"required,email,max=128"`
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
	Nickname string `json:"nickname" validate:"max=64"`
	Email    string `json:"email" validate:"omitempty,email,max=128"`
	Avatar   string `json:"avatar" validate:"max=255"`
}

// UpdatePasswordRequest represents PUT /api/v1/user/password payload.
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6,max=64"`
}
