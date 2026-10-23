package auth

import (
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("uma-senha-bem-longa")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
	if strings.Contains(hash, "uma-senha-bem-longa") {
		t.Fatal("hash contains the plain password")
	}

	ok, err := VerifyPassword("uma-senha-bem-longa", hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}
	if !ok {
		t.Fatal("valid password was rejected")
	}

	ok, err = VerifyPassword("outra-senha", hash)
	if err != nil {
		t.Fatalf("verify incorrect password: %v", err)
	}
	if ok {
		t.Fatal("incorrect password was accepted")
	}
}

func TestHashPasswordProducesUniqueSalt(t *testing.T) {
	first, err := HashPassword("uma-senha-bem-longa")
	if err != nil {
		t.Fatalf("first hash: %v", err)
	}
	second, err := HashPassword("uma-senha-bem-longa")
	if err != nil {
		t.Fatalf("second hash: %v", err)
	}
	if first == second {
		t.Fatal("hashes should differ because of unique salts")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	if _, err := VerifyPassword("secret", "not-a-hash"); err == nil {
		t.Fatal("malformed hash was accepted")
	}
}
