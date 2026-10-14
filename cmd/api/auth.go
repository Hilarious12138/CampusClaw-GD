package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

const jwtIssuer = "campusclaw-gd"
const jwtAudience = "campusclaw-api"
const accessTTL = 30 * time.Minute

type User struct {
	ID          string        `json:"id"`
	Username    string        `json:"username"`
	DisplayName string        `json:"displayName"`
	Role        string        `json:"role"`
	Classes     []SchoolClass `json:"classes"`
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *App) issueToken(id string) (string, error) {
	now := time.Now()
	c := jwt.RegisteredClaims{Issuer: jwtIssuer, Subject: id, Audience: jwt.ClaimStrings{jwtAudience}, ID: newID(), IssuedAt: jwt.NewNumericDate(now), NotBefore: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(accessTTL))}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(a.config.JWTSecret))
}
func (a *App) claims(r *http.Request) (*jwt.RegisteredClaims, error) {
	h := r.Header.Values("Authorization")
	if len(h) != 1 {
		return nil, fail(401, "请先登录。")
	}
	parts := strings.Fields(h[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || len(parts[1]) > 4096 {
		return nil, fail(401, "请先登录。")
	}
	c := &jwt.RegisteredClaims{}
	t, e := jwt.ParseWithClaims(parts[1], c, func(t *jwt.Token) (any, error) { return []byte(a.config.JWTSecret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(jwtIssuer), jwt.WithAudience(jwtAudience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if e != nil || t == nil || !t.Valid || !idPattern.MatchString(c.Subject) || !idPattern.MatchString(c.ID) || c.IssuedAt == nil || c.NotBefore == nil || c.ExpiresAt == nil || !c.ExpiresAt.After(c.IssuedAt.Time) || c.ExpiresAt.Sub(c.IssuedAt.Time) > accessTTL {
		return nil, fail(401, "登录已失效，请重新登录。")
	}
	return c, nil
}
func (a *App) user(r *http.Request) (User, error) {
	var u User
	c, e := a.claims(r)
	if e != nil {
		return u, e
	}
	e = a.db.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.display_name,u.role FROM users u WHERE u.id=? AND NOT EXISTS(SELECT 1 FROM revoked_tokens WHERE jti=? AND expires_at>UTC_TIMESTAMP(6))`, c.Subject, c.ID).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role)
	if errors.Is(e, sql.ErrNoRows) {
		e = fail(401, "登录已失效，请重新登录。")
	}
	return u, e
}
func (a *App) withClasses(r *http.Request, u User) (User, error) {
	rows, e := a.db.QueryContext(r.Context(), "SELECT c.id,c.code,c.name FROM classes c JOIN class_memberships cm ON cm.class_id=c.id WHERE cm.user_id=? ORDER BY c.code", u.ID)
	if e != nil {
		return u, e
	}
	defer rows.Close()
	u.Classes = []SchoolClass{}
	for rows.Next() {
		var c SchoolClass
		if e = rows.Scan(&c.ID, &c.Code, &c.Name); e != nil {
			return u, e
		}
		u.Classes = append(u.Classes, c)
	}
	return u, rows.Err()
}
func (a *App) checkOrigin(r *http.Request) error {
	v := r.Header.Get("Origin")
	if v != "" && (v == a.config.Origin || a.config.DevOrigin != "" && v == a.config.DevOrigin) {
		return nil
	}
	return fail(403, "请求来源无效。")
}
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	if e := a.checkOrigin(r); e != nil {
		return e
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&b) != nil || len(b.Password) > 1024 || len(b.Username) > 100 || b.Password == "" {
		return fail(401, "账号或密码不正确。")
	}
	var u User
	var hash string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,username,display_name,role,password_hash FROM users WHERE username=?", strings.TrimSpace(b.Username)).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &hash)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if hash == "" {
		hash = a.dummyHash
	}
	if !passwordMatches(hash, b.Password) || u.ID == "" {
		return fail(401, "账号或密码不正确。")
	}
	u, e = a.withClasses(r, u)
	if e != nil {
		return e
	}
	value, e := a.issueToken(u.ID)
	if e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "campus_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	return send(w, 200, map[string]any{"access_token": value, "token_type": "Bearer", "expires_in": int(accessTTL.Seconds()), "user": u})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.user(r); e != nil {
		return e
	}
	if e := a.checkOrigin(r); e != nil {
		return e
	}
	c, e := a.claims(r)
	if e != nil {
		return e
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO revoked_tokens(jti,expires_at) VALUES(?,?) ON DUPLICATE KEY UPDATE expires_at=VALUES(expires_at)", c.ID, c.ExpiresAt.Time.UTC()); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM revoked_tokens WHERE expires_at<=UTC_TIMESTAMP(6)"); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	w.WriteHeader(204)
	return nil
}
func (a *App) me(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	u, e = a.withClasses(r, u)
	if e != nil {
		return e
	}
	return send(w, 200, u)
}
func (a *App) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var n int
	e := a.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM users WHERE 1=0)+(SELECT count(*) FROM revoked_tokens WHERE 1=0)+(SELECT count(*) FROM classes WHERE 1=0)+(SELECT count(*) FROM class_memberships WHERE 1=0)+(SELECT count(*) FROM materials WHERE 1=0)+(SELECT count(*) FROM material_contents WHERE 1=0)+(SELECT count(*) FROM seed_versions WHERE version=1)`).Scan(&n)
	if e != nil || n != 1 {
		return send(w, 503, map[string]string{"status": "unavailable"})
	}
	return send(w, 200, map[string]string{"status": "ok"})
}
