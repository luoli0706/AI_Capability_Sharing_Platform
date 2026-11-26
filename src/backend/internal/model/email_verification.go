package model

import "time"

const (
	EmailPurposeRegister      = "register"
	EmailPurposeBindEmail     = "bind_email"
	EmailPurposeResetPassword = "reset_password"
)

// EmailVerificationCode stores hashed 6-digit email verification codes with cooldown & attempt tracking.
type EmailVerificationCode struct {
	ID          uint64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Email       string     `gorm:"type:varchar(128);index:idx_email_purpose_created,priority:1;not null" json:"email"`
	Purpose     string     `gorm:"type:varchar(32);index:idx_email_purpose_created,priority:2;not null" json:"purpose"`
	CodeHash    string     `gorm:"type:varchar(128);not null" json:"-"`
	ClientIP    string     `gorm:"type:varchar(64);index:idx_ip_purpose_created,priority:1;not null" json:"client_ip"`
	UserID      *uint64    `gorm:"index" json:"user_id,omitempty"`
	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts int        `gorm:"not null;default:5" json:"max_attempts"`
	UsedAt      *time.Time `gorm:"type:datetime" json:"used_at,omitempty"`
	ExpiresAt   time.Time  `gorm:"type:datetime;not null;index" json:"expires_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime;index:idx_email_purpose_created,priority:3;index:idx_ip_purpose_created,priority:3" json:"created_at"`
}

func (EmailVerificationCode) TableName() string {
	return "email_verification_code"
}

// EmailDailyQuota tracks atomic daily email sending quotas per Asia/Shanghai calendar date and purpose.
type EmailDailyQuota struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	BizDate   string    `gorm:"type:varchar(16);uniqueIndex:uk_date_purpose,priority:1;not null" json:"biz_date"`
	Purpose   string    `gorm:"type:varchar(32);uniqueIndex:uk_date_purpose,priority:2;not null" json:"purpose"`
	SentCount int       `gorm:"not null;default:0" json:"sent_count"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (EmailDailyQuota) TableName() string {
	return "email_daily_quota"
}
