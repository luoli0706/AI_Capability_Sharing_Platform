package validator

import (
	"strings"
	"testing"
)

type samplePayload struct {
	Username string `validate:"required,min=3"`
	Email    string `validate:"required,email"`
}

func TestCustomValidator(t *testing.T) {
	v := New()

	if err := v.Validate(samplePayload{
		Username: "alice",
		Email:    "alice@example.com",
	}); err != nil {
		t.Fatalf("expected valid payload, got error: %v", err)
	}

	err := v.Validate(samplePayload{
		Username: "ab",
		Email:    "not-an-email",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "Username") || !strings.Contains(err.Error(), "Email") {
		t.Fatalf("expected error to mention Username and Email, got: %s", err.Error())
	}
}
