package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	DomainRoleOwner  = "owner"
	DomainRoleAdmin  = "admin"
	DomainRoleMember = "member"
)

type BizDomain struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	DomainCode  string         `gorm:"type:varchar(64);uniqueIndex:uk_domain_code;not null" json:"domain_code"`
	DomainName  string         `gorm:"type:varchar(128);not null" json:"domain_name"`
	Description string         `gorm:"type:varchar(512);not null;default:''" json:"description"`
	OwnerID     uint64         `gorm:"not null" json:"owner_id"`
	Status      int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (BizDomain) TableName() string {
	return "biz_domain"
}

type DomainMember struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	DomainID   uint64    `gorm:"uniqueIndex:uk_domain_user;not null" json:"domain_id"`
	UserID     uint64    `gorm:"uniqueIndex:uk_domain_user;not null" json:"user_id"`
	DomainRole string    `gorm:"type:varchar(32);not null;default:'member'" json:"domain_role"`
	JoinedAt   time.Time `gorm:"autoCreateTime" json:"joined_at"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DomainMember) TableName() string {
	return "domain_member"
}

// DomainListItem represents a domain row with current user's role and member count.
type DomainListItem struct {
	ID          uint64    `json:"id"`
	DomainCode  string    `json:"domain_code"`
	DomainName  string    `json:"domain_name"`
	Description string    `json:"description"`
	OwnerID     uint64    `json:"owner_id"`
	Status      int8      `json:"status"`
	MyRole      string    `json:"my_role"`
	MemberCount int64     `json:"member_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// DomainMemberItem represents a member entry in GET /api/v1/domains/:id/members.
type DomainMemberItem struct {
	UserID     uint64    `json:"user_id"`
	Username   string    `json:"username"`
	Nickname   string    `json:"nickname"`
	Email      string    `json:"email"`
	DomainRole string    `json:"domain_role"`
	JoinedAt   time.Time `json:"joined_at"`
}

// CreateDomainRequest represents POST /api/v1/domains payload.
type CreateDomainRequest struct {
	DomainCode  string `json:"domain_code" validate:"required,min=2,max=64"`
	DomainName  string `json:"domain_name" validate:"required,min=2,max=128"`
	Description string `json:"description" validate:"max=512"`
}

// AddDomainMemberRequest represents POST /api/v1/domains/:id/members payload.
type AddDomainMemberRequest struct {
	UserID     uint64 `json:"user_id"`
	Username   string `json:"username"`
	DomainRole string `json:"domain_role" validate:"required,oneof=owner admin member"`
}
