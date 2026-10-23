package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      = 64 * 1024
	argonIterations  = 3
	argonParallelism = 4
	argonSaltLength  = 16
	argonKeyLength   = 32
)

var ErrInvalidHash = errors.New("invalid password hash")

type PasswordParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

var defaultPasswordParams = PasswordParams{
	Memory:      argonMemory,
	Iterations:  argonIterations,
	Parallelism: argonParallelism,
	SaltLength:  argonSaltLength,
	KeyLength:   argonKeyLength,
}

func HashPassword(password string) (string, error) {
	return hashPassword(password, defaultPasswordParams)
}

func hashPassword(password string, params PasswordParams) (string, error) {
	salt := make([]byte, params.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, params.KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Iterations,
		params.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	params, salt, expected, err := decodeHash(encoded)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, params.Iterations, params.Memory, params.Parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func decodeHash(encoded string) (PasswordParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	version, err := strconv.Atoi(strings.TrimPrefix(parts[2], "v="))
	if err != nil || version != argon2.Version {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}
	if len(salt) < 8 || len(key) < 16 {
		return PasswordParams{}, nil, nil, ErrInvalidHash
	}

	return PasswordParams{
		Memory:      memory,
		Iterations:  iterations,
		Parallelism: parallelism,
		SaltLength:  uint32(len(salt)),
		KeyLength:   uint32(len(key)),
	}, salt, key, nil
}
