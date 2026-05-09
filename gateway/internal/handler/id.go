package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("web-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
