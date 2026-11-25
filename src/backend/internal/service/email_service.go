package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/mailer"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

const (
	DailyQuotaRegister   = 100
	DailyQuotaBindEmail  = 75
	EmailCooldownWindow  = 60 * time.Second
	EmailCodeTTL         = 10 * time.Minute
	EmailCodeMaxAttempts = 5
)

type EmailService interface {
	SendPublicRegisterCode(ctx context.Context, rawEmail, clientIP string) error
	SendBindEmailCode(ctx context.Context, userID uint64, rawEmail, clientIP string) error
	VerifyCode(rawEmail, purpose, code string) error
}

type emailService struct {
	emailRepo  repository.EmailRepository
	userRepo   repository.UserRepository
	mailer     mailer.Mailer
	hmacSecret string
	loc        *time.Location
	nowFn      func() time.Time
}

func NewEmailService(
	emailRepo repository.EmailRepository,
	userRepo repository.UserRepository,
	m mailer.Mailer,
	hmacSecret string,
) EmailService {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &emailService{
		emailRepo:  emailRepo,
		userRepo:   userRepo,
		mailer:     m,
		hmacSecret: hmacSecret,
		loc:        loc,
		nowFn:      time.Now,
	}
}

func (s *emailService) SendPublicRegisterCode(ctx context.Context, rawEmail, clientIP string) error {
	normalized, err := validateAndCheckDisposableEmail(rawEmail)
	if err != nil {
		return err
	}

	if _, err := s.userRepo.FindByEmail(normalized); err == nil {
		return NewConflict("email already exists")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.sendCode(ctx, nil, normalized, model.EmailPurposeRegister, clientIP, DailyQuotaRegister, "注册名额已耗尽，等明日再来")
}

func (s *emailService) SendBindEmailCode(ctx context.Context, userID uint64, rawEmail, clientIP string) error {
	normalized, err := validateAndCheckDisposableEmail(rawEmail)
	if err != nil {
		return err
	}

	existing, err := s.userRepo.FindByEmail(normalized)
	if err == nil && existing.ID != userID {
		return NewConflict("email already in use")
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return s.sendCode(ctx, &userID, normalized, model.EmailPurposeBindEmail, clientIP, DailyQuotaBindEmail, "今日修改绑定邮箱名额已耗尽，等明日再来")
}

func (s *emailService) VerifyCode(rawEmail, purpose, code string) error {
	normalized, err := validateAndCheckDisposableEmail(rawEmail)
	if err != nil {
		return err
	}
	trimmedCode := strings.TrimSpace(code)
	if len(trimmedCode) != 6 {
		return NewBadRequest("verification code must be 6 digits")
	}

	err = s.emailRepo.VerifyAndConsumeCode(normalized, purpose, trimmedCode, s.hmacSecret, s.nowFn())
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrVerificationCodeMaxAttempts):
			return NewBadRequest("验证码错误次数过多，请重新获取")
		case errors.Is(err, repository.ErrVerificationCodeInvalidOrExpired):
			return NewBadRequest("验证码无效或已过期")
		default:
			return err
		}
	}
	return nil
}

func (s *emailService) sendCode(
	ctx context.Context,
	userID *uint64,
	normalizedEmail, purpose, clientIP string,
	dailyLimit int,
	quotaExhaustedMsg string,
) error {
	if s.mailer == nil {
		return &AppError{HTTPStatus: 503, Code: 50301, Message: "邮件服务未配置，请联系管理员"}
	}

	now := s.nowFn()
	emailLimited, ipLimited, err := s.emailRepo.CheckCooldown(normalizedEmail, purpose, clientIP, EmailCooldownWindow, now)
	if err != nil {
		return err
	}
	if emailLimited || ipLimited {
		return NewTooManyRequests("发送过于频繁，请 60 秒后再试")
	}

	bizDate := now.In(s.loc).Format("2006-01-02")
	acquired, err := s.emailRepo.TryAcquireDailyQuota(bizDate, purpose, dailyLimit)
	if err != nil {
		return err
	}
	if !acquired {
		return NewTooManyRequests(quotaExhaustedMsg)
	}

	plainCode, err := repository.GenerateSixDigitCode()
	if err != nil {
		_ = s.emailRepo.ReleaseDailyQuota(bizDate, purpose)
		return err
	}

	record := &model.EmailVerificationCode{
		Email:       normalizedEmail,
		Purpose:     purpose,
		CodeHash:    repository.HashVerificationCode(plainCode, s.hmacSecret),
		ClientIP:    clientIP,
		UserID:      userID,
		Attempts:    0,
		MaxAttempts: EmailCodeMaxAttempts,
		ExpiresAt:   now.Add(EmailCodeTTL),
		CreatedAt:   now,
	}
	if err := s.emailRepo.CreateCode(record); err != nil {
		_ = s.emailRepo.ReleaseDailyQuota(bizDate, purpose)
		return err
	}

	if err := s.mailer.SendVerificationCode(ctx, normalizedEmail, purpose, plainCode); err != nil {
		_ = s.emailRepo.ReleaseDailyQuota(bizDate, purpose)
		_ = s.emailRepo.InvalidateCode(record.ID, now)
		return &AppError{HTTPStatus: 502, Code: 50201, Message: "验证码邮件发送失败，请稍后重试"}
	}
	return nil
}

func validateAndCheckDisposableEmail(raw string) (string, error) {
	normalized, err := mailer.NormalizeAndValidateEmail(raw)
	if err != nil {
		if errors.Is(err, mailer.ErrPlusAliasEmail) {
			return "", NewBadRequest("不支持包含 '+' 别名的邮箱地址")
		}
		return "", NewBadRequest("邮箱格式不正确")
	}
	if mailer.IsDisposableEmail(normalized) {
		return "", NewBadRequest("不支持使用临时邮箱地址")
	}
	return normalized, nil
}
