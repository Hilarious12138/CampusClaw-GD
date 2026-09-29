package main

import (
	"database/sql"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

const maxFile = 10 * 1024 * 1024

type SchoolClass struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}
type Material struct {
	ID        string    `json:"id"`
	Filename  string    `json:"filename"`
	Type      string    `json:"type"`
	Size      int       `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
	Uploader  string    `json:"uploaderName"`
	Body      string    `json:"body,omitempty"`
}

func (a *App) class(r *http.Request, u User) (SchoolClass, error) {
	var c SchoolClass
	id := r.PathValue("classId")
	if !idPattern.MatchString(id) {
		return c, fail(404, "未找到班级或材料。")
	}
	e := a.db.QueryRowContext(r.Context(), "SELECT c.id,c.code,c.name FROM classes c JOIN class_memberships cm ON cm.class_id=c.id WHERE cm.user_id=? AND c.id=?", u.ID, id).Scan(&c.ID, &c.Code, &c.Name)
	if errors.Is(e, sql.ErrNoRows) {
		e = fail(404, "未找到班级或材料。")
	}
	return c, e
}
func (a *App) list(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	c, e := a.class(r, u)
	if e != nil {
		return e
	}
	rows, e := a.db.QueryContext(r.Context(), `SELECT m.id,m.filename,m.type,m.size_bytes,m.created_at,u.display_name FROM materials m JOIN class_memberships cm ON cm.class_id=m.class_id JOIN users u ON u.id=m.uploader_id WHERE cm.user_id=? AND m.class_id=? ORDER BY m.created_at DESC,m.id`, u.ID, c.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	items := []Material{}
	for rows.Next() {
		var m Material
		if e = rows.Scan(&m.ID, &m.Filename, &m.Type, &m.Size, &m.CreatedAt, &m.Uploader); e != nil {
			return e
		}
		items = append(items, m)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return send(w, 200, struct {
		Class     SchoolClass `json:"class"`
		Materials []Material  `json:"materials"`
	}{c, items})
}
func (a *App) readMaterial(r *http.Request, u User) (Material, []byte, error) {
	var m Material
	var raw []byte
	cid, id := r.PathValue("classId"), r.PathValue("id")
	if !idPattern.MatchString(cid) || !idPattern.MatchString(id) {
		return m, nil, fail(404, "未找到班级或材料。")
	}
	e := a.db.QueryRowContext(r.Context(), `SELECT m.id,m.filename,m.type,m.size_bytes,m.created_at,u.display_name,mc.body,mc.original FROM materials m JOIN material_contents mc ON mc.material_id=m.id JOIN class_memberships cm ON cm.class_id=m.class_id JOIN users u ON u.id=m.uploader_id WHERE cm.user_id=? AND m.class_id=? AND m.id=?`, u.ID, cid, id).Scan(&m.ID, &m.Filename, &m.Type, &m.Size, &m.CreatedAt, &m.Uploader, &m.Body, &raw)
	if errors.Is(e, sql.ErrNoRows) {
		e = fail(404, "未找到班级或材料。")
	}
	return m, raw, e
}
func (a *App) detail(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	m, _, e := a.readMaterial(r, u)
	if e != nil {
		return e
	}
	return send(w, 200, m)
}
func (a *App) download(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	m, raw, e := a.readMaterial(r, u)
	if e != nil {
		return e
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": m.Filename}))
	w.WriteHeader(200)
	_, e = w.Write(raw)
	return e
}
func parseText(raw []byte) (string, error) {
	if !utf8.Valid(raw) {
		return "", fail(415, "文件必须使用 UTF-8 编码。")
	}
	s := strings.TrimPrefix(string(raw), "\ufeff")
	for _, ch := range s {
		if ch == 0 || (ch < 32 && ch != '\n' && ch != '\r' && ch != '\t') {
			return "", fail(415, "文件不是有效文本。")
		}
	}
	if strings.TrimSpace(s) == "" {
		return "", fail(422, "未解析到有效正文。")
	}
	return s, nil
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) error {
	u, e := a.user(r)
	if e != nil {
		return e
	}
	if u.Role != "teacher" {
		return fail(403, "学生账号不能上传材料。")
	}
	c, e := a.class(r, u)
	if e != nil {
		return e
	}
	if e = a.csrf(r, u); e != nil {
		return e
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxFile+65536)
	reader, e := r.MultipartReader()
	if e != nil {
		return fail(422, "请选择一个文件。")
	}
	part, e := reader.NextPart()
	if e != nil {
		return fail(422, "请选择一个文件。")
	}
	defer part.Close()
	name := strings.ReplaceAll(filepath.Base(part.FileName()), "\\", "_")
	ext := strings.ToLower(filepath.Ext(name))
	if part.FormName() != "file" || name == "" || len([]rune(name)) > 255 {
		return fail(422, "文件名称无效。")
	}
	if ext != ".txt" && ext != ".md" {
		return fail(415, "仅支持 TXT 和 Markdown 文件。")
	}
	f, e := os.CreateTemp(a.config.UploadDir, "pending-")
	if e != nil {
		return e
	}
	path := f.Name()
	defer os.Remove(path)
	defer f.Close()
	start := time.Now()
	n, e := io.Copy(f, io.LimitReader(part, maxFile+1))
	if n > maxFile {
		return fail(413, "文件不能超过 10 MiB。")
	}
	if e != nil {
		return fail(422, "文件传输失败。")
	}
	if _, e = reader.NextPart(); e != io.EOF {
		return fail(422, "每次仅允许一个文件，不允许额外字段。")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	body, e := parseText(raw)
	if e != nil {
		return e
	}
	if time.Since(start) > 10*time.Second {
		return fail(422, "解析超时。")
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var allowed string
	e = tx.QueryRowContext(r.Context(), "SELECT u.id FROM users u JOIN class_memberships cm ON cm.user_id=u.id WHERE u.id=? AND u.role='teacher' AND cm.class_id=? FOR SHARE", u.ID, c.ID).Scan(&allowed)
	if errors.Is(e, sql.ErrNoRows) {
		return fail(404, "未找到班级或材料。")
	}
	if e != nil {
		return e
	}
	id := newID()
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO materials(id,class_id,uploader_id,filename,type,size_bytes) VALUES(?,?,?,?,?,?)", id, c.ID, u.ID, name, ext[1:], n); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "INSERT INTO material_contents VALUES(?,?,?)", id, body, raw); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return send(w, 201, map[string]string{"id": id})
}
