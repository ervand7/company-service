package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func TestJWTManagerGenerateAndValidate(t *testing.T) {
	manager := NewJWTManager("secret")

	tokenString, err := manager.Generate("user-123", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	got, err := manager.Validate(tokenString)
	if err != nil {
		t.Fatal(err)
	}

	claims, ok := got.(*Claims)
	if !ok {
		t.Fatalf("expected *Claims, got %T", got)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %q", claims.Subject)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("expected expiration claim")
	}
	if claims.IssuedAt == nil {
		t.Fatal("expected issued-at claim")
	}
}

func TestJWTManagerValidateRejectsExpiredToken(t *testing.T) {
	manager := NewJWTManager("secret")
	tokenString := signedToken(t, "secret", jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(-time.Hour)),
		},
	})

	_, err := manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestJWTManagerValidateRequiresExpiration(t *testing.T) {
	manager := NewJWTManager("secret")
	tokenString := signedToken(t, "secret", jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-123"},
	})

	_, err := manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected missing expiration error")
	}
	if !strings.Contains(err.Error(), "token expiration is required") {
		t.Fatalf("expected expiration error, got %v", err)
	}
}

func TestJWTManagerValidateRejectsWrongSecret(t *testing.T) {
	manager := NewJWTManager("secret")
	tokenString := signedToken(t, "other-secret", jwt.SigningMethodHS256, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})

	_, err := manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected signature error")
	}
}

func TestJWTManagerValidateRejectsUnexpectedSigningMethod(t *testing.T) {
	manager := NewJWTManager("secret")
	tokenString := signedToken(t, "secret", jwt.SigningMethodHS384, Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(time.Hour)),
		},
	})

	_, err := manager.Validate(tokenString)
	if err == nil {
		t.Fatal("expected signing method error")
	}
	if !strings.Contains(err.Error(), "unexpected signing method") {
		t.Fatalf("expected signing method error, got %v", err)
	}
}

func TestJWTManagerValidateRejectsMalformedToken(t *testing.T) {
	manager := NewJWTManager("secret")

	_, err := manager.Validate("not-a-token")
	if err == nil {
		t.Fatal("expected malformed token error")
	}
}

func signedToken(t *testing.T, secret string, method jwt.SigningMethod, claims Claims) string {
	t.Helper()

	tokenString, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	if tokenString == "" {
		t.Fatal("expected signed token")
	}

	return tokenString
}
