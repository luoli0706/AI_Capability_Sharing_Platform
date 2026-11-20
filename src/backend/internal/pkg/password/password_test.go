package password

import "testing"

func TestHashAndVerify(t *testing.T) {
	plain := "S3cur3P@ssw0rd!"
	hashed, err := Hash(plain)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}
	if hashed == "" || hashed == plain {
		t.Fatalf("invalid hash output")
	}

	if !Verify(plain, hashed) {
		t.Fatalf("expected password verification to succeed")
	}
	if Verify("wrong-password", hashed) {
		t.Fatalf("expected wrong password verification to fail")
	}
}
