package mailer

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeAndValidateEmail(t *testing.T) {
	got, err := NormalizeAndValidateEmail("  Alice.Test@Example.COM ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "alice.test@example.com" {
		t.Fatalf("expected normalized email, got %q", got)
	}

	if _, err := NormalizeAndValidateEmail("alice+tag@example.com"); !errors.Is(err, ErrPlusAliasEmail) {
		t.Fatalf("expected ErrPlusAliasEmail, got %v", err)
	}

	for _, invalid := range []string{"", "not-an-email", "Alice <alice@example.com>", "a@localhost", "a b@example.com"} {
		if _, err := NormalizeAndValidateEmail(invalid); err == nil {
			t.Fatalf("expected error for %q, got nil", invalid)
		}
	}
}

func TestIsDisposableEmail(t *testing.T) {
	blocked := []string{
		"user@yopmail.com",
		"user@sub.yopmail.fr",
		"user@YOPMAIL.NET",
		"user@tempmail.com",
		"user@temp-mail.org",
		"user@10minutemail.com",
		"user@guerrillamail.com",
		"user@mailinator.com",
		"user@trashmail.com",
		"user@sharklasers.com",
		"user@getnada.com",
		"user@dispostable.com",
		"user@fakeinbox.com",
		"user@mohmal.com",
		"user@maildrop.cc",
		"user@throwawaymail.com",
	}
	for _, e := range blocked {
		if !IsDisposableEmail(e) {
			t.Fatalf("expected %q to be blocked as disposable", e)
		}
	}

	allowed := []string{"user@gmail.com", "user@qq.com", "user@7thcv.cn", "user@my-yopmail.com"}
	for _, e := range allowed {
		if IsDisposableEmail(e) {
			t.Fatalf("expected %q to be allowed", e)
		}
	}
}

func TestBuildSubjectAndHTML(t *testing.T) {
	regSubj, regAction := buildSubjectAndAction(PurposeRegister)
	if regSubj != "【AI能力共享平台】注册验证码" {
		t.Fatalf("unexpected register subject: %q", regSubj)
	}
	bindSubj, _ := buildSubjectAndAction(PurposeBindEmail)
	if bindSubj != "【AI能力共享平台】修改绑定邮箱验证码" {
		t.Fatalf("unexpected bind subject: %q", bindSubj)
	}
	resetSubj, resetAction := buildSubjectAndAction(PurposeResetPassword)
	if resetSubj != "【AI能力共享平台】找回密码验证码" || resetAction != "找回密码" {
		t.Fatalf("unexpected reset subject/action: %q / %q", resetSubj, resetAction)
	}
	htmlBody := buildVerificationHTML(regAction, "123456")
	if !strings.Contains(htmlBody, "123456") || !strings.Contains(htmlBody, "10 分钟") {
		t.Fatalf("expected code and 10 分钟 in htmlBody, got %s", htmlBody)
	}
	noticeHTML := buildPasswordResetNoticeHTML("alice")
	if !strings.Contains(noticeHTML, "000000") || !strings.Contains(noticeHTML, "个人中心") || !strings.Contains(noticeHTML, "alice") {
		t.Fatalf("expected 000000, 个人中心, and username in noticeHTML, got %s", noticeHTML)
	}
}
