package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

const cookieName = "campus_session"

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	CSRFToken   string `json:"csrfToken"`
}

func token() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (a *App) key(t string) string {
	h := hmac.New(sha256.New, []byte(a.config.SessionSecret))
	h.Write([]byte(t))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *App) user(r *http.Request) (User, error) {
	var u User
	c, e := r.Cookie(cookieName)
	if e != nil || len(c.Value) != 43 {
		return u, fail(401, "请先登录。")
	}
	e = a.db.QueryRowContext(r.Context(), `SELECT u.id,u.username,u.display_name,u.role,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.id=? AND s.expires_at>UTC_TIMESTAMP(6)`, a.key(c.Value)).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &u.CSRFToken)
	if errors.Is(e, sql.ErrNoRows) {
		return u, fail(401, "请先登录。")
	}
	return u, e
}
func (a *App) checkOrigin(r *http.Request) error {
	origin := r.Header.Get("Origin")
	if origin != "" && (origin == a.config.Origin || a.config.DevOrigin != "" && origin == a.config.DevOrigin) {
		return nil
	}
	return fail(403, "请求来源无效。")
}
func (a *App) csrf(r *http.Request, u User) error {
	if e := a.checkOrigin(r); e != nil {
		return e
	}
	got := r.Header.Get("X-CSRF-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(u.CSRFToken)) != 1 {
		return fail(403, "安全校验已失效，请刷新后重试。")
	}
	return nil
}
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	if e := a.checkOrigin(r); e != nil {
		return e
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if json.NewDecoder(r.Body).Decode(&body) != nil || len(body.Password) > 1024 || len(body.Username) > 100 || body.Password == "" {
		return fail(401, "账号或密码不正确。")
	}
	var id, hash string
	e := a.db.QueryRowContext(r.Context(), "SELECT id,password_hash FROM users WHERE username=?", strings.TrimSpace(body.Username)).Scan(&id, &hash)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if hash == "" {
		hash = a.dummyHash
	}
	valid := passwordMatches(hash, body.Password)
	if !valid || id == "" {
		return fail(401, "账号或密码不正确。")
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if old, e := r.Cookie(cookieName); e == nil {
		if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=?", a.key(old.Value)); e != nil {
			return e
		}
	}
	t := token()
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO sessions VALUES(?,?,?,?)", a.key(t), id, token(), time.Now().UTC().Add(8*time.Hour)); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "DELETE FROM sessions WHERE expires_at<=UTC_TIMESTAMP(6)"); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: t, Path: "/", HttpOnly: true, Secure: a.config.Secure, SameSite: http.SameSiteLaxMode, MaxAge: 8 * 60 * 60})
	return send(w, 200, map[string]bool{"ok": true})
}
func (a *App) logout(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	if e = a.csrf(r, u); e != nil {
		return e
	}
	c, _ := r.Cookie(cookieName)
	if _, e = a.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE id=?", a.key(c.Value)); e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.config.Secure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(204)
	return nil
}
func (a *App) me(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT c.id,c.code,c.name FROM classes c JOIN class_memberships cm ON cm.class_id=c.id WHERE cm.user_id=? ORDER BY c.code", u.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	classes := []SchoolClass{}
	for rows.Next() {
		var c SchoolClass
		if e = rows.Scan(&c.ID, &c.Code, &c.Name); e != nil {
			return e
		}
		classes = append(classes, c)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return send(w, 200, struct {
		User
		Classes []SchoolClass `json:"classes"`
	}{u, classes})
}
func (a *App) pageAuth(w http.ResponseWriter, r *http.Request) error {
	if _, e := a.user(r); e != nil {
		return e
	}
	w.WriteHeader(204)
	return nil
}
func (a *App) health(w http.ResponseWriter, r *http.Request) error {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var n int
	e := a.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM users WHERE 1=0)+(SELECT count(*) FROM sessions WHERE 1=0)+(SELECT count(*) FROM classes WHERE 1=0)+(SELECT count(*) FROM class_memberships WHERE 1=0)+(SELECT count(*) FROM materials WHERE 1=0)+(SELECT count(*) FROM material_contents WHERE 1=0)+(SELECT count(*) FROM seed_versions WHERE version=1)`).Scan(&n)
	if e != nil || n != 1 {
		return send(w, 503, map[string]string{"status": "unavailable"})
	}
	return send(w, 200, map[string]string{"status": "ok"})
}
