package main

import (
	"github.com/golang-jwt/jwt/v5"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJWTValidation(t *testing.T) {
	a := App{config: Config{JWTSecret: strings.Repeat("s", 48)}}
	id := strings.Repeat("a", 32)
	value, e := a.issueToken(id)
	if e != nil {
		t.Fatal(e)
	}
	check := func(v string) bool {
		r := httptest.NewRequest("GET", "/api/me", nil)
		r.Header.Set("Authorization", "Bearer "+v)
		_, e := a.claims(r)
		return e == nil
	}
	if !check(value) {
		t.Fatal("valid rejected")
	}
	if check(value + "x") {
		t.Fatal("tamper accepted")
	}
	now := time.Now()
	valid := func() jwt.RegisteredClaims {
		return jwt.RegisteredClaims{Issuer: jwtIssuer, Audience: jwt.ClaimStrings{jwtAudience}, Subject: id, ID: strings.Repeat("b", 32), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL))}
	}
	for _, name := range []string{"expired", "issuer", "audience", "missing-exp", "missing-iat", "missing-nbf", "missing-jti", "subject", "future", "algorithm", "none"} {
		t.Run(name, func(t *testing.T) {
			c := valid()
			method := jwt.SigningMethodHS256
			var key any = []byte(a.config.JWTSecret)
			switch name {
			case "expired":
				c.ExpiresAt = jwt.NewNumericDate(now.Add(-time.Minute))
			case "issuer":
				c.Issuer = "other"
			case "audience":
				c.Audience = jwt.ClaimStrings{"other"}
			case "missing-exp":
				c.ExpiresAt = nil
			case "missing-iat":
				c.IssuedAt = nil
			case "missing-nbf":
				c.NotBefore = nil
			case "missing-jti":
				c.ID = ""
			case "subject":
				c.Subject = "bad"
			case "future":
				c.NotBefore = jwt.NewNumericDate(now.Add(time.Hour))
			case "algorithm":
				method = jwt.SigningMethodHS384
			}
			var tok *jwt.Token
			if name == "none" {
				tok = jwt.NewWithClaims(jwt.SigningMethodNone, c)
				key = jwt.UnsafeAllowNoneSignatureType
			} else {
				tok = jwt.NewWithClaims(method, c)
			}
			v, e := tok.SignedString(key)
			if e != nil {
				t.Fatal(e)
			}
			if check(v) {
				t.Fatal("invalid accepted")
			}
		})
	}
	for _, location := range []string{"cookie", "query"} {
		r := httptest.NewRequest("GET", "/api/me?token="+value, nil)
		if location == "cookie" {
			r.Header.Set("Cookie", "campus_session="+value)
		}
		if _, e := a.claims(r); e == nil {
			t.Fatal("non-Bearer accepted")
		}
	}
}
