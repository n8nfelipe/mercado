package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const sessionTokenBytes = 32

func NewSessionToken() (string, []byte, error) {
	buffer := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", nil, fmt.Errorf("generate session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buffer)
	return token, HashSessionToken(token), nil
}

func HashSessionToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
