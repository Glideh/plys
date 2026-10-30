package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := VerifyPassword("correct horse", hash); err != nil || !ok {
		t.Fatalf("expected match, got ok=%v err=%v", ok, err)
	}
	if ok, _ := VerifyPassword("wrong", hash); ok {
		t.Fatal("expected mismatch")
	}
}

func TestPasswordHashesAreSalted(t *testing.T) {
	a, _ := HashPassword("same")
	b, _ := HashPassword("same")
	if a == b {
		t.Fatal("two hashes of the same password must differ")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	for _, h := range []string{"", "plain", "$bcrypt$x$y$z$w", strings.Replace(mustHash(t), "argon2id", "argon2i", 1)} {
		if _, err := VerifyPassword("x", h); err != ErrInvalidHash {
			t.Errorf("hash %q: expected ErrInvalidHash, got %v", h, err)
		}
	}
}

func TestAccessTokenRoundTrip(t *testing.T) {
	iss := NewIssuer([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	id := uuid.New()
	tok, _, err := iss.Issue(id, RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := iss.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != id.String() || claims.Role != RoleAdmin {
		t.Fatalf("unexpected claims %+v", claims)
	}
}

func TestAccessTokenRejections(t *testing.T) {
	iss := NewIssuer([]byte("0123456789abcdef0123456789abcdef"), time.Minute)
	tok, _, _ := iss.Issue(uuid.New(), RoleUser)

	other := NewIssuer([]byte("fedcba9876543210fedcba9876543210"), time.Minute)
	if _, err := other.Parse(tok); err == nil {
		t.Error("token signed with another secret must be rejected")
	}

	expired := NewIssuer([]byte("0123456789abcdef0123456789abcdef"), -time.Minute)
	old, _, _ := expired.Issue(uuid.New(), RoleUser)
	if _, err := iss.Parse(old); err == nil {
		t.Error("expired token must be rejected")
	}
}

func mustHash(t *testing.T) string {
	t.Helper()
	h, err := HashPassword("x")
	if err != nil {
		t.Fatal(err)
	}
	return h
}
