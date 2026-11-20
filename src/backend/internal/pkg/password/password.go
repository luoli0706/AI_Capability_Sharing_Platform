package password

import "golang.org/x/crypto/bcrypt"

// Hash generates a bcrypt hash from a plaintext password.
func Hash(plain string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Verify compares a plaintext password with a bcrypt hash.
func Verify(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
