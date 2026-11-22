package kbcodec

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestAtRestEncryptDecryptRoundtrip(t *testing.T) {
	secret := "my-super-secret-jwt-key"
	cases := [][]byte{
		{},
		[]byte("hello world"),
		[]byte("# 知识库文档\n\n包含中文与 emoji 🚀 的 Markdown 内容。"),
		bytes.Repeat([]byte{0x00, 0xFF, 0x13, 0x37}, 4096),
	}

	for i, plain := range cases {
		enc, err := EncryptAtRest(plain, secret)
		if err != nil {
			t.Fatalf("case %d: EncryptAtRest failed: %v", i, err)
		}
		if !IsEncryptedAtRest(enc) {
			t.Fatalf("case %d: expected IsEncryptedAtRest=true", i)
		}
		if len(plain) > 0 && bytes.Contains(enc, plain) {
			t.Fatalf("case %d: ciphertext should not contain plaintext", i)
		}

		dec, err := DecryptAtRest(enc, secret)
		if err != nil {
			t.Fatalf("case %d: DecryptAtRest failed: %v", i, err)
		}
		if !bytes.Equal(dec, plain) {
			t.Fatalf("case %d: decrypted mismatch", i)
		}
	}
}

func TestAtRestWrongKeyFails(t *testing.T) {
	enc, err := EncryptAtRest([]byte("top secret content"), "correct-secret")
	if err != nil {
		t.Fatalf("EncryptAtRest failed: %v", err)
	}
	if _, err := DecryptAtRest(enc, "wrong-secret"); err == nil {
		t.Fatal("expected DecryptAtRest with wrong key to fail")
	}
}

func TestAtRestCorruptedPayloadFails(t *testing.T) {
	if _, err := DecryptAtRest([]byte(MagicHeader+"short"), "secret"); err == nil {
		t.Fatal("expected truncated encrypted payload to fail")
	}
}

func TestAtRestLegacyPlainPassthrough(t *testing.T) {
	legacy := []byte("# Legacy Unencrypted Markdown\nPlaintext on disk.")
	if IsEncryptedAtRest(legacy) {
		t.Fatal("expected IsEncryptedAtRest=false for legacy plaintext")
	}
	dec, err := DecryptAtRest(legacy, "any-secret")
	if err != nil {
		t.Fatalf("DecryptAtRest failed on legacy plaintext: %v", err)
	}
	if !bytes.Equal(dec, legacy) {
		t.Fatal("expected legacy plaintext to pass through unchanged")
	}
}

func TestTransportObfuscateRoundtrip(t *testing.T) {
	cases := [][]byte{
		{},
		[]byte("a"),
		[]byte("abc"),
		[]byte("abcd"),
		[]byte("Hello, ASCP! 测试中文与表情 🔐"),
		bytes.Repeat([]byte("0123456789abcdef"), 1024),
	}

	for i, plain := range cases {
		obf, err := ObfuscateTransport(plain)
		if err != nil {
			t.Fatalf("case %d: ObfuscateTransport failed: %v", i, err)
		}
		if len(obf) != saltSize+len(plain) {
			t.Fatalf("case %d: unexpected obf length: got %d want %d", i, len(obf), saltSize+len(plain))
		}
		if len(plain) > 0 && bytes.Equal(obf[saltSize:], plain) {
			t.Fatalf("case %d: obfuscated payload should differ from plaintext", i)
		}

		deobf, err := DeobfuscateTransport(obf)
		if err != nil {
			t.Fatalf("case %d: DeobfuscateTransport failed: %v", i, err)
		}
		if !bytes.Equal(deobf, plain) {
			t.Fatalf("case %d: deobfuscated mismatch", i)
		}
	}

	if _, err := DeobfuscateTransport([]byte("too-short")); err == nil {
		t.Fatal("expected DeobfuscateTransport on short input to fail")
	}
}

func TestTransportFixedSaltVector(t *testing.T) {
	// Fixed 16-byte salt for cross-language (Go <-> TypeScript) verification.
	salt := []byte{
		0x00, 0x01, 0x02, 0x03,
		0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b,
		0x0c, 0x0d, 0x0e, 0x0f,
	}
	plain := []byte("ASCP-KB-Vector-12345")

	obf, err := ObfuscateTransportWithSalt(plain, salt)
	if err != nil {
		t.Fatalf("ObfuscateTransportWithSalt failed: %v", err)
	}

	// Verify against independently computed xorshift128 vector:
	// s0 = 0x03020100 ^ 0x9E3779B9 = 0x9D3578B9
	// s1 = 0x07060504 ^ 0x85EBCA6B = 0x82EDCF6F
	// s2 = 0x0B0A0908 ^ 0xC2B2AE35 = 0xC9B8A73D
	// s3 = 0x0F0E0D0C ^ 0x27D4EB2F = 0x28DAE623
	wantPayloadHex := "70f05f4e07ca24df4234888a7b29d230034635cf"
	gotPayloadHex := hex.EncodeToString(obf[saltSize:])
	if gotPayloadHex != wantPayloadHex {
		t.Fatalf("fixed salt vector mismatch: got %s want %s", gotPayloadHex, wantPayloadHex)
	}

	deobf, err := DeobfuscateTransport(obf)
	if err != nil {
		t.Fatalf("DeobfuscateTransport failed: %v", err)
	}
	if !bytes.Equal(deobf, plain) {
		t.Fatalf("deobf mismatch: got %q want %q", deobf, plain)
	}
}
