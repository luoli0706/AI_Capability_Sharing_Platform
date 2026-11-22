package kbcodec

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
)

const (
	// MagicHeader identifies files encrypted at rest by ASCP.
	MagicHeader = "ASCPENC1"
	saltSize    = 16
)

var magicBytes = []byte(MagicHeader)

// DeriveKey derives a 32-byte AES-256 key from the given secret using SHA-256.
func DeriveKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// IsEncryptedAtRest reports whether data starts with the ASCP at-rest encryption header.
func IsEncryptedAtRest(data []byte) bool {
	return bytes.HasPrefix(data, magicBytes)
}

// EncryptAtRest encrypts plain bytes using AES-256-GCM with a random 12-byte nonce.
// Output format: ASCPENC1 (8 bytes) || nonce (12 bytes) || ciphertext.
func EncryptAtRest(plain []byte, secret string) ([]byte, error) {
	block, err := aes.NewCipher(DeriveKey(secret))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	out := make([]byte, len(magicBytes)+nonceSize, len(magicBytes)+nonceSize+len(plain)+gcm.Overhead())
	copy(out, magicBytes)
	nonce := out[len(magicBytes) : len(magicBytes)+nonceSize]
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	out = gcm.Seal(out, nonce, plain, nil)
	return out, nil
}

// DecryptAtRest decrypts data if it has the ASCPENC1 header; otherwise returns data as-is
// for backward compatibility with legacy unencrypted files.
func DecryptAtRest(data []byte, secret string) ([]byte, error) {
	if !IsEncryptedAtRest(data) {
		return data, nil
	}

	block, err := aes.NewCipher(DeriveKey(secret))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	headerLen := len(magicBytes) + nonceSize
	if len(data) < headerLen+gcm.Overhead() {
		return nil, errors.New("kbcodec: invalid encrypted payload")
	}

	nonce := data[len(magicBytes):headerLen]
	ciphertext := data[headerLen:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ObfuscateTransport applies a fast xorshift128 stream XOR using a random 16-byte salt.
// Output format: salt (16 bytes) || xored_plain.
func ObfuscateTransport(plain []byte) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}
	return ObfuscateTransportWithSalt(plain, salt)
}

// ObfuscateTransportWithSalt applies the transport stream XOR using the provided 16-byte salt.
func ObfuscateTransportWithSalt(plain, salt []byte) ([]byte, error) {
	if len(salt) != saltSize {
		return nil, errors.New("kbcodec: salt must be 16 bytes")
	}
	out := make([]byte, saltSize+len(plain))
	copy(out[:saltSize], salt)
	xorKeystream(out[saltSize:], plain, salt)
	return out, nil
}

// DeobfuscateTransport reverses ObfuscateTransport by reading the 16-byte salt prefix
// and XORing the remaining payload with the identical xorshift128 keystream.
func DeobfuscateTransport(obf []byte) ([]byte, error) {
	if len(obf) < saltSize {
		return nil, errors.New("kbcodec: obfuscated payload too short")
	}
	salt := obf[:saltSize]
	out := make([]byte, len(obf)-saltSize)
	xorKeystream(out, obf[saltSize:], salt)
	return out, nil
}

func xorKeystream(dst, src, salt []byte) {
	s0 := binary.LittleEndian.Uint32(salt[0:4]) ^ 0x9E3779B9
	s1 := binary.LittleEndian.Uint32(salt[4:8]) ^ 0x85EBCA6B
	s2 := binary.LittleEndian.Uint32(salt[8:12]) ^ 0xC2B2AE35
	s3 := binary.LittleEndian.Uint32(salt[12:16]) ^ 0x27D4EB2F
	if s0 == 0 {
		s0 |= 1
	}
	if s1 == 0 {
		s1 |= 1
	}
	if s2 == 0 {
		s2 |= 1
	}
	if s3 == 0 {
		s3 |= 1
	}

	n := len(src)
	i := 0
	for i+4 <= n {
		t := s0 ^ (s0 << 11)
		s0 = s1
		s1 = s2
		s2 = s3
		s3 = s3 ^ (s3 >> 19) ^ (t ^ (t >> 8))

		dst[i] = src[i] ^ byte(s3)
		dst[i+1] = src[i+1] ^ byte(s3>>8)
		dst[i+2] = src[i+2] ^ byte(s3>>16)
		dst[i+3] = src[i+3] ^ byte(s3>>24)
		i += 4
	}
	if i < n {
		t := s0 ^ (s0 << 11)
		s0 = s1
		s1 = s2
		s2 = s3
		s3 = s3 ^ (s3 >> 19) ^ (t ^ (t >> 8))

		for j := 0; i+j < n; j++ {
			dst[i+j] = src[i+j] ^ byte(s3>>(8*j))
		}
	}
}
