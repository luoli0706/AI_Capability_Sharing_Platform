package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	DomainRoleOwner  = "owner"
	DomainRoleAdmin  = "admin"
	DomainRoleMember = "member"

	DomainVisibilityPublic     = "public"
	DomainVisibilityPrivate    = "private"
	DomainVisibilityOrgPrivate = "org_private"

	DomainJoinModeApprovalOnly   = "approval_only"
	DomainJoinModeApprovalOrQuiz = "approval_or_quiz"
)

type BizDomain struct {
	ID             uint64         `gorm:"primaryKey;autoIncrement" json:"id"`
	DomainUID      string         `gorm:"column:domain_uid;type:varchar(32);uniqueIndex:uk_domain_uid;not null;default:''" json:"domain_uid"`
	DomainCode     string         `gorm:"type:varchar(64);uniqueIndex:uk_domain_code;not null" json:"domain_code"`
	DomainName     string         `gorm:"type:varchar(128);not null" json:"domain_name"`
	Description    string         `gorm:"type:varchar(512);not null;default:''" json:"description"`
	Visibility     string         `gorm:"type:varchar(16);not null;default:'public'" json:"visibility"`
	JoinMode       string         `gorm:"type:varchar(24);not null;default:'approval_only'" json:"join_mode"`
	JoinQuestion   string         `gorm:"type:varchar(512);not null;default:''" json:"join_question"`
	JoinAnswerHash string         `gorm:"type:varchar(128);not null;default:''" json:"-"`
	OrgID          uint64         `gorm:"index:idx_org_id;not null;default:0" json:"org_id"`
	OrgName        string         `gorm:"type:varchar(64);not null;default:'个人'" json:"org_name"`
	OwnerID        uint64         `gorm:"not null" json:"owner_id"`
	Status         int8           `gorm:"type:tinyint;not null;default:1" json:"status"`
	CreatedAt      time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
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

// DomainListItem represents a domain row with visibility, org attribution, and caller permissions.
type DomainListItem struct {
	ID           uint64    `json:"id"`
	DomainUID    string    `json:"domain_uid"`
	DomainCode   string    `json:"domain_code"`
	DomainName   string    `json:"domain_name"`
	Description  string    `json:"description"`
	Visibility   string    `json:"visibility"`
	JoinMode     string    `json:"join_mode"`
	JoinQuestion string    `json:"join_question"`
	OrgID        uint64    `json:"org_id"`
	OrgName      string    `json:"org_name"`
	OwnerID      uint64    `json:"owner_id"`
	Status       int8      `json:"status"`
	MyRole       string    `json:"my_role"`
	CanManage    bool      `json:"can_manage"`
	HasOrgAccess bool      `json:"has_org_access"`
	MemberCount  int64     `json:"member_count"`
	CreatedAt    time.Time `json:"created_at"`
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

// DomainQuery defines filters and pagination for GET /api/v1/domains.
type DomainQuery struct {
	Tab       string
	Keyword   string
	DomainUID string
	OrgID     uint64
	Page      int
	PageSize  int
}

// CreateDomainRequest represents POST /api/v1/domains payload.
type CreateDomainRequest struct {
	DomainCode   string `json:"domain_code" validate:"required,min=2,max=64"`
	DomainName   string `json:"domain_name" validate:"required,min=2,max=128"`
	Description  string `json:"description" validate:"max=512"`
	Visibility   string `json:"visibility" validate:"omitempty,oneof=public private org_private"`
	OrgID        uint64 `json:"org_id"`
	JoinMode     string `json:"join_mode" validate:"omitempty,oneof=approval_only approval_or_quiz"`
	JoinQuestion string `json:"join_question" validate:"max=512"`
	JoinAnswer   string `json:"join_answer" validate:"max=256"`
}

// UpdateDomainJoinSettingsRequest represents PUT /api/v1/domains/:id/join-settings payload.
type UpdateDomainJoinSettingsRequest struct {
	JoinMode     string `json:"join_mode" validate:"required,oneof=approval_only approval_or_quiz"`
	JoinQuestion string `json:"join_question" validate:"max=512"`
	JoinAnswer   string `json:"join_answer" validate:"max=256"`
}

// JoinDomainRequest represents POST /api/v1/domains/:id/join-requests payload.
type JoinDomainRequest struct {
	Reason string `json:"reason" validate:"max=500"`
}

// JoinDomainByQuizRequest represents POST /api/v1/domains/:id/join-by-quiz payload.
type JoinDomainByQuizRequest struct {
	Answer string `json:"answer" validate:"required,max=256"`
}

// UpdateDomainRequest represents PUT /api/v1/domains/:id payload.
type UpdateDomainRequest struct {
	DomainName  string  `json:"domain_name" validate:"omitempty,min=2,max=128"`
	Description *string `json:"description" validate:"omitempty,max=512"`
	Visibility  string  `json:"visibility" validate:"omitempty,oneof=public private org_private"`
}

// AddDomainMemberRequest represents POST /api/v1/domains/:id/members payload.
type AddDomainMemberRequest struct {
	UserID     uint64 `json:"user_id"`
	Username   string `json:"username"`
	DomainRole string `json:"domain_role" validate:"required,oneof=owner admin member"`
}

// UpdateDomainMemberRoleRequest represents PUT /api/v1/domains/:id/members/:userId payload.
type UpdateDomainMemberRoleRequest struct {
	DomainRole string `json:"domain_role" validate:"required,oneof=admin member"`
}
