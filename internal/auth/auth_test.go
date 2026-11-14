package auth

import (
	"testing"
	"time"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	secret := "s3cret"
	token, err := Sign(secret, Claims{UserID: 42, Email: "a@b.c", Exp: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	claims, err := Verify(secret, token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "a@b.c" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestVerifyRejectsTamperedToken(t *testing.T) {
	secret := "s3cret"
	token, _ := Sign(secret, Claims{UserID: 1, Exp: time.Now().Add(time.Hour).Unix()})
	tampered := token[:len(token)-2] + "xx"
	if _, err := Verify(secret, tampered); err == nil {
		t.Fatal("expected error for tampered token")
	}
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	secret := "s3cret"
	token, _ := Sign(secret, Claims{UserID: 1, Exp: time.Now().Add(-time.Hour).Unix()})
	if _, err := Verify(secret, token); err == nil {
		t.Fatal("expected error for expired token")
	}
}
