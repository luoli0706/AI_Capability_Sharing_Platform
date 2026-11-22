package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	OrgRoleOwner  = "owner"
	OrgRoleAdmin  = "admin"
	OrgRoleMember = "member"

	OrgVisibilityPublic  = "public"
	OrgVisibilityPrivate = "private"
)

type BizOrganization struct {
	ID          uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	OrgCode     string         `gorm:"type:varchar(64);uniqueIndex:uk_org_code;not null" json:"org_code"`
	OrgName     string         `gorm:"type:varchar(128);not null" json:"org_name"`
	Description string         `gorm:"type:varchar(512);not null;default:''" json:"description"`
	Visibility  string         `gorm:"size:20;not null;default:'public';index" json:"visibility"`
	OwnerID     uint64         `gorm:"not null" json:"owner_id"`
	Status      int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (BizOrganization) TableName() string {
	return "biz_organization"
}

type OrganizationMember struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	OrgID     uint64    `gorm:"uniqueIndex:uk_org_user;not null" json:"org_id"`
	UserID    uint64    `gorm:"uniqueIndex:uk_org_user;not null" json:"user_id"`
	OrgRole   string    `gorm:"type:varchar(32);not null;default:'member'" json:"org_role"`
	JoinedAt  time.Time `gorm:"autoCreateTime" json:"joined_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (OrganizationMember) TableName() string {
	return "organization_member"
}

type OrgListItem struct {
	ID          uint64    `json:"id"`
	OrgCode     string    `json:"org_code"`
	OrgName     string    `json:"org_name"`
	Description string    `json:"description"`
	Visibility  string    `json:"visibility"`
	OwnerID     uint64    `json:"owner_id"`
	Status      int8      `json:"status"`
	MyRole      string    `json:"my_role"`
	MemberCount int64     `json:"member_count"`
	DomainCount int64     `json:"domain_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type OrgDetail struct {
	OrgListItem
	PublicDomains []DomainListItem `json:"public_domains"`
}

type OrgMemberItem struct {
	UserID   uint64    `json:"user_id"`
	Username string    `json:"username"`
	Nickname string    `json:"nickname"`
	Email    string    `json:"email"`
	OrgRole  string    `json:"org_role"`
	JoinedAt time.Time `json:"joined_at"`
}

type OrgQuery struct {
	Tab          string
	Keyword      string
	IsSuperAdmin bool
	Page         int
	PageSize     int
}

type JoinOrgRequest struct {
	Reason string `json:"reason" validate:"max=512"`
}

type CreateOrgRequest struct {
	OrgCode     string `json:"org_code" validate:"required,min=2,max=64"`
	OrgName     string `json:"org_name" validate:"required,min=2,max=128"`
	Description string `json:"description" validate:"max=512"`
	Visibility  string `json:"visibility" validate:"omitempty,oneof=public private"`
}

type UpdateOrgRequest struct {
	OrgName     string  `json:"org_name" validate:"omitempty,min=2,max=128"`
	Description *string `json:"description" validate:"omitempty,max=512"`
	Visibility  string  `json:"visibility" validate:"omitempty,oneof=public private"`
}

type AddOrgMemberRequest struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	OrgRole  string `json:"org_role" validate:"required,oneof=admin member"`
}
