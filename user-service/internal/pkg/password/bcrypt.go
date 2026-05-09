package password

import (
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(raw string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(hash)
}

func ComparePassword(raw, encoded string) bool {
	return bcrypt.CompareHashAndPassword([]byte(encoded), []byte(raw)) == nil
}
