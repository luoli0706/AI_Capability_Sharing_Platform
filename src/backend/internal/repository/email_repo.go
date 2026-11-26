package repository

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"ascp-backend/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrVerificationCodeInvalidOrExpired = errors.New("verification code is invalid or expired")
	ErrVerificationCodeMaxAttempts      = errors.New("verification code exceeded maximum attempts")
)

type EmailRepository interface {
	CheckCooldown(email, purpose, clientIP string, window time.Duration, now time.Time) (emailLimited bool, ipLimited bool, err error)
	TryAcquireDailyQuota(bizDate, purpose string, limit int) (bool, error)
	TryAcquireDailyQuotaWithGlobal(bizDate, purpose string, purposeLimit, globalLimit int) (bool, string, error)
	ReleaseDailyQuota(bizDate, purpose string) error
	CreateCode(code *model.EmailVerificationCode) error
	InvalidateCode(id uint64, now time.Time) error
	VerifyAndConsumeCode(email, purpose, plainCode, hmacSecret string, now time.Time) error
}

type emailRepository struct {
	db *gorm.DB
}

func NewEmailRepository(db *gorm.DB) EmailRepository {
	return &emailRepository{db: db}
}

func (r *emailRepository) CheckCooldown(email, purpose, clientIP string, window time.Duration, now time.Time) (bool, bool, error) {
	cutoff := now.Add(-window)

	var emailCnt int64
	if err := r.db.Model(&model.EmailVerificationCode{}).
		Where("email = ? AND purpose = ? AND created_at > ?", email, purpose, cutoff).
		Count(&emailCnt).Error; err != nil {
		return false, false, err
	}

	var ipCnt int64
	if clientIP != "" {
		if err := r.db.Model(&model.EmailVerificationCode{}).
			Where("client_ip = ? AND purpose = ? AND created_at > ?", clientIP, purpose, cutoff).
			Count(&ipCnt).Error; err != nil {
			return false, false, err
		}
	}

	return emailCnt > 0, ipCnt > 0, nil
}

func (r *emailRepository) TryAcquireDailyQuota(bizDate, purpose string, limit int) (bool, error) {
	ok, _, err := r.TryAcquireDailyQuotaWithGlobal(bizDate, purpose, limit, 0)
	return ok, err
}

func (r *emailRepository) TryAcquireDailyQuotaWithGlobal(bizDate, purpose string, purposeLimit, globalLimit int) (bool, string, error) {
	acquired := false
	reason := ""

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Ensure row exists safely across both MySQL and SQLite under concurrency.
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "biz_date"}, {Name: "purpose"}},
			DoNothing: true,
		}).Create(&model.EmailDailyQuota{
			BizDate:   bizDate,
			Purpose:   purpose,
			SentCount: 0,
		}).Error; err != nil {
			return err
		}

		var rows []model.EmailDailyQuota
		query := tx.Where("biz_date = ?", bizDate)
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.Find(&rows).Error; err != nil {
			return err
		}

		totalSent := 0
		purposeSent := 0
		for _, row := range rows {
			totalSent += row.SentCount
			if row.Purpose == purpose {
				purposeSent = row.SentCount
			}
		}

		if globalLimit > 0 && totalSent >= globalLimit {
			reason = "global"
			return nil
		}
		if purposeLimit > 0 && purposeSent >= purposeLimit {
			reason = "purpose"
			return nil
		}

		res := tx.Model(&model.EmailDailyQuota{}).
			Where("biz_date = ? AND purpose = ?", bizDate, purpose).
			Updates(map[string]interface{}{
				"sent_count": gorm.Expr("sent_count + 1"),
				"updated_at": time.Now(),
			})
		if res.Error != nil {
			return res.Error
		}
		acquired = res.RowsAffected > 0
		return nil
	})
	if err != nil {
		return false, "", err
	}
	return acquired, reason, nil
}

func (r *emailRepository) ReleaseDailyQuota(bizDate, purpose string) error {
	return r.db.Model(&model.EmailDailyQuota{}).
		Where("biz_date = ? AND purpose = ? AND sent_count > 0", bizDate, purpose).
		Updates(map[string]interface{}{
			"sent_count": gorm.Expr("sent_count - 1"),
			"updated_at": time.Now(),
		}).Error
}

func (r *emailRepository) CreateCode(code *model.EmailVerificationCode) error {
	return r.db.Create(code).Error
}

func (r *emailRepository) InvalidateCode(id uint64, now time.Time) error {
	return r.db.Model(&model.EmailVerificationCode{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now).Error
}

func (r *emailRepository) VerifyAndConsumeCode(email, purpose, plainCode, hmacSecret string, now time.Time) error {
	expectedHash := HashVerificationCode(plainCode, hmacSecret)

	return r.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("email = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?", email, purpose, now).
			Order("id DESC")
		if tx.Dialector.Name() != "sqlite" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}

		var record model.EmailVerificationCode
		if err := query.First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrVerificationCodeInvalidOrExpired
			}
			return err
		}

		if record.Attempts >= record.MaxAttempts {
			_ = tx.Model(&record).Update("used_at", now).Error
			return ErrVerificationCodeMaxAttempts
		}

		if subtle.ConstantTimeCompare([]byte(record.CodeHash), []byte(expectedHash)) != 1 {
			newAttempts := record.Attempts + 1
			updates := map[string]interface{}{"attempts": newAttempts}
			if newAttempts >= record.MaxAttempts {
				updates["used_at"] = now
			}
			if err := tx.Model(&record).Updates(updates).Error; err != nil {
				return err
			}
			if newAttempts >= record.MaxAttempts {
				return ErrVerificationCodeMaxAttempts
			}
			return ErrVerificationCodeInvalidOrExpired
		}

		return tx.Model(&record).Update("used_at", now).Error
	})
}

// GenerateSixDigitCode generates a cryptographically random 6-digit numeric code.
func GenerateSixDigitCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashVerificationCode computes HMAC-SHA256(code, secret) as lowercase hex.
func HashVerificationCode(code, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}
