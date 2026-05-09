package biz

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

func newConversationID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	value := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b[:])
	return "c_" + strings.ToLower(value), nil
}
