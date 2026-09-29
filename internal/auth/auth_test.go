package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("a-secure-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if strings.Contains(hash, "a-secure-password") {
		t.Fatal("hash contains the password")
	}
	if err := checkPassword(hash, "a-secure-password"); err != nil {
		t.Fatalf("compare: %v", err)
	}
	if err := checkPassword(hash, "wrong-password"); err == nil {
		t.Fatal("expected wrong password to fail")
	}
}

func TestTokenRoundTrip(t *testing.T) {
	secret := strings.Repeat("s", 32)
	admin := Admin{ID: "4d5e6f70-1111-2222-3333-444455556666", Name: "Owner", Email: "owner@example.com"}
	now := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

	signed, err := SignToken(secret, admin, now)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	got, err := ParseToken(secret, signed, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != admin {
		t.Fatalf("admin = %+v", got)
	}

	if _, err := ParseToken(strings.Repeat("x", 32), signed, now); err == nil {
		t.Fatal("expected a different secret to fail")
	}
	if _, err := ParseToken(secret, signed, now.Add(13*time.Hour)); err == nil {
		t.Fatal("expected an expired token to fail")
	}
}

func TestLoginRejectsMissingFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", LoginHandler(nil, strings.Repeat("s", 32)))

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"","password":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "INVALID_CREDENTIALS" {
		t.Fatalf("code = %q", body.Error.Code)
	}
}

func TestRequireAdminRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/me", RequireAdmin(nil, strings.Repeat("s", 32)), MeHandler())

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCreateAdminValidatesInput(t *testing.T) {
	if err := CreateAdmin(t.Context(), nil, "", "owner@example.com", "long-enough"); err != ErrInvalidAdmin {
		t.Fatalf("name error = %v", err)
	}
	if err := CreateAdmin(t.Context(), nil, "Owner", "owner@example.com", "short"); err != ErrInvalidPassword {
		t.Fatalf("password error = %v", err)
	}
}
