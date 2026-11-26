package mailer

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/mail"
	"regexp"
	"strings"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dmclient "github.com/alibabacloud-go/dm-20151123/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

const (
	PurposeRegister      = "register"
	PurposeBindEmail     = "bind_email"
	PurposeResetPassword = "reset_password"

	DefaultEndpoint    = "dm.aliyuncs.com"
	DefaultAccountName = "verify@email.7thcv.cn"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrPlusAliasEmail  = errors.New("email '+' alias is not allowed")
	ErrDisposableEmail = errors.New("disposable email domain is not allowed")
)

var emailDomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

var disposableDomains = []string{
	"yopmail.com",
	"yopmail.fr",
	"yopmail.net",
	"yopmail.org",
	"tempmail.com",
	"temp-mail.org",
	"temp-mail.io",
	"tempmailo.com",
	"10minutemail.com",
	"10minutemail.net",
	"guerrillamail.com",
	"guerrillamail.net",
	"guerrillamail.org",
	"guerrillamailblock.com",
	"grr.la",
	"mailinator.com",
	"trashmail.com",
	"trashmail.net",
	"sharklasers.com",
	"getnada.com",
	"dispostable.com",
	"fakeinbox.com",
	"mohmal.com",
	"maildrop.cc",
	"throwawaymail.com",
	"mailnesia.com",
	"mintemail.com",
	"emailondeck.com",
}

// Mailer sends verification code and notification emails.
type Mailer interface {
	SendVerificationCode(ctx context.Context, toEmail, purpose, code string) error
	SendPasswordResetNotice(ctx context.Context, toEmail, username string) error
}

// AliyunDirectMailer sends emails via Alibaba Cloud DirectMail SingleSendMail API.
type AliyunDirectMailer struct {
	client      *dmclient.Client
	accountName string
}

// NewAliyunDirectMailer creates an AliyunDirectMailer from AK/SK and endpoint settings.
func NewAliyunDirectMailer(ak, sk, endpoint, accountName string) (*AliyunDirectMailer, error) {
	if strings.TrimSpace(ak) == "" || strings.TrimSpace(sk) == "" {
		return nil, errors.New("aliyun directmail AK/SK is not configured")
	}
	if strings.TrimSpace(endpoint) == "" {
		endpoint = DefaultEndpoint
	}
	if strings.TrimSpace(accountName) == "" {
		accountName = DefaultAccountName
	}

	cfg := &openapi.Config{
		AccessKeyId:     tea.String(strings.TrimSpace(ak)),
		AccessKeySecret: tea.String(strings.TrimSpace(sk)),
		Endpoint:        tea.String(strings.TrimSpace(endpoint)),
	}
	cli, err := dmclient.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("init aliyun directmail client: %w", err)
	}
	return &AliyunDirectMailer{
		client:      cli,
		accountName: strings.TrimSpace(accountName),
	}, nil
}

// SendVerificationCode sends a 6-digit verification code email for register or bind_email.
func (m *AliyunDirectMailer) SendVerificationCode(_ context.Context, toEmail, purpose, code string) error {
	if m == nil || m.client == nil {
		return errors.New("aliyun directmail client is not initialized")
	}

	subject, actionTitle := buildSubjectAndAction(purpose)
	htmlBody := buildVerificationHTML(actionTitle, code)

	req := &dmclient.SingleSendMailRequest{
		AccountName:    tea.String(m.accountName),
		AddressType:    tea.Int32(1),
		ReplyToAddress: tea.Bool(false),
		ToAddress:      tea.String(toEmail),
		Subject:        tea.String(subject),
		HtmlBody:       tea.String(htmlBody),
	}

	if _, err := m.client.SingleSendMail(req); err != nil {
		return fmt.Errorf("aliyun SingleSendMail failed: %w", err)
	}
	return nil
}

// SendPasswordResetNotice sends a notification email informing the user that their password has been reset to 000000.
func (m *AliyunDirectMailer) SendPasswordResetNotice(_ context.Context, toEmail, username string) error {
	if m == nil || m.client == nil {
		return errors.New("aliyun directmail client is not initialized")
	}

	subject := "【AI能力共享平台】密码重置成功通知"
	htmlBody := buildPasswordResetNoticeHTML(username)

	req := &dmclient.SingleSendMailRequest{
		AccountName:    tea.String(m.accountName),
		AddressType:    tea.Int32(1),
		ReplyToAddress: tea.Bool(false),
		ToAddress:      tea.String(toEmail),
		Subject:        tea.String(subject),
		HtmlBody:       tea.String(htmlBody),
	}

	if _, err := m.client.SingleSendMail(req); err != nil {
		return fmt.Errorf("aliyun SingleSendMail failed: %w", err)
	}
	return nil
}

func buildSubjectAndAction(purpose string) (string, string) {
	switch purpose {
	case PurposeBindEmail:
		return "【AI能力共享平台】修改绑定邮箱验证码", "修改绑定邮箱"
	case PurposeResetPassword:
		return "【AI能力共享平台】找回密码验证码", "找回密码"
	default:
		return "【AI能力共享平台】注册验证码", "账号注册"
	}
}

func buildPasswordResetNoticeHTML(username string) string {
	safeUser := html.EscapeString(strings.TrimSpace(username))
	if safeUser == "" {
		safeUser = "用户"
	}
	return fmt.Sprintf(
		`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:520px;margin:0 auto;padding:24px;border:1px solid #e5e7eb;border-radius:10px;color:#111827;">`+
			`<h2 style="margin:0 0 12px;font-size:18px;">AI能力共享平台 - 密码重置成功通知</h2>`+
			`<p style="margin:0 0 12px;font-size:14px;line-height:1.6;color:#374151;">您好，<strong>%s</strong>！您的账号密码已通过绑定邮箱验证重置成功。</p>`+
			`<p style="margin:0 0 12px;font-size:14px;line-height:1.6;color:#374151;">您的临时登录密码已重置为：</p>`+
			`<div style="margin:16px 0;padding:14px 20px;background:#fef2f2;border:1px solid #fecaca;border-radius:8px;text-align:center;font-size:28px;font-weight:700;letter-spacing:6px;color:#dc2626;">000000</div>`+
			`<p style="margin:0 0 8px;font-size:13px;line-height:1.6;color:#b91c1c;"><strong>安全提示：</strong>当前密码为默认临时密码 <strong>000000（六个零）</strong>，强烈建议您立即登录平台并前往「个人中心」更改密码！</p>`+
			`<p style="margin:0;font-size:12px;color:#9ca3af;">如非本人操作，请立即联系网站管理员处理。</p>`+
			`</div>`,
		safeUser,
	)
}

func buildVerificationHTML(actionTitle, code string) string {
	safeAction := html.EscapeString(actionTitle)
	safeCode := html.EscapeString(code)
	return fmt.Sprintf(
		`<div style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;max-width:520px;margin:0 auto;padding:24px;border:1px solid #e5e7eb;border-radius:10px;color:#111827;">`+
			`<h2 style="margin:0 0 12px;font-size:18px;">AI能力共享平台 - %s验证</h2>`+
			`<p style="margin:0 0 16px;font-size:14px;line-height:1.6;color:#374151;">您好！您正在进行%s操作，本次验证码为：</p>`+
			`<div style="margin:16px 0;padding:14px 20px;background:#f3f4f6;border-radius:8px;text-align:center;font-size:28px;font-weight:700;letter-spacing:6px;color:#1d4ed8;">%s</div>`+
			`<p style="margin:0 0 8px;font-size:13px;color:#4b5563;">验证码有效期为 <strong>10 分钟</strong>，请勿泄露给他人。</p>`+
			`<p style="margin:0;font-size:12px;color:#9ca3af;">如非本人操作，请忽略此邮件。</p>`+
			`</div>`,
		safeAction, safeAction, safeCode,
	)
}

// NormalizeAndValidateEmail trims, lowercases, validates format, and rejects '+' alias addresses.
func NormalizeAndValidateEmail(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" || len(normalized) > 128 || strings.ContainsAny(normalized, " \t\r\n") {
		return "", ErrInvalidEmail
	}

	addr, err := mail.ParseAddress(normalized)
	if err != nil || addr.Address != normalized || addr.Name != "" {
		return "", ErrInvalidEmail
	}

	atIdx := strings.LastIndexByte(normalized, '@')
	if atIdx <= 0 || atIdx >= len(normalized)-1 {
		return "", ErrInvalidEmail
	}
	localPart := normalized[:atIdx]
	domainPart := normalized[atIdx+1:]

	if strings.Contains(localPart, "+") {
		return "", ErrPlusAliasEmail
	}
	if !emailDomainRegex.MatchString(domainPart) {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

// IsDisposableEmail checks whether the email's domain (or parent domain) is in the disposable blacklist.
func IsDisposableEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	atIdx := strings.LastIndexByte(normalized, '@')
	if atIdx < 0 || atIdx >= len(normalized)-1 {
		return false
	}
	domain := normalized[atIdx+1:]
	for _, blocked := range disposableDomains {
		if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
			return true
		}
	}
	return false
}
