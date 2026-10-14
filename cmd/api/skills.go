package main

import (
	"context"
	"database/sql"
	"errors"
	"github.com/go-sql-driver/mysql"
	"net/http"
	"strings"
	"time"
)

const defaultSkillSource = "名称：解题引导\n适用条件：解题,方程,思路\n资料：本班知识库\n工具：knowledge_search\n启用：是\n\n先询问学生已知条件和已尝试步骤。每次只给下一步提示或检查问题，帮助学生自行推导，不直接给最终答案或完整解答。根据本班证据解释相关方法并标注出处。"

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Trigger     string   `json:"trigger"`
	Subject     string   `json:"subject"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Source      string   `json:"source"`
	Tools       []string `json:"tools"`
}

func parseSkill(source string) (Skill, error) {
	s := Skill{Source: source, Tools: []string{"knowledge_search"}}
	if len([]rune(source)) > 8000 {
		return s, fail(400, "技能文件不能超过 8000 字。")
	}
	normalized := strings.ReplaceAll(source, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	fields := map[string]string{}
	bodyStart, seenHeader := -1, false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if strings.TrimSpace(line) == "" {
			if seenHeader {
				bodyStart = i + 1
				break
			}
			continue
		}
		seenHeader = true
		line = strings.Replace(line, ":", "：", 1)
		kv := strings.SplitN(line, "：", 2)
		if len(kv) != 2 {
			return s, fail(400, "技能头部格式无效。")
		}
		key := strings.TrimSpace(kv[0])
		if key != "名称" && key != "适用条件" && key != "资料" && key != "工具" && key != "启用" {
			return s, fail(400, "技能字段无效。")
		}
		if _, ok := fields[key]; ok {
			return s, fail(400, "技能字段重复。")
		}
		fields[key] = strings.TrimSpace(kv[1])
	}
	if bodyStart < 0 {
		return s, fail(400, "技能文件需要空行分隔头部和说明。")
	}
	if _, ok := fields["名称"]; !ok {
		return s, fail(400, "缺少名称。")
	}
	if _, ok := fields["适用条件"]; !ok {
		return s, fail(400, "缺少适用条件。")
	}
	if fields["工具"] == "" {
		fields["工具"] = "knowledge_search"
	}
	if fields["工具"] != "knowledge_search" || (fields["启用"] != "是" && fields["启用"] != "否") {
		return s, fail(400, "技能仅支持名称、适用条件、资料、工具 knowledge_search 和启用是/否。")
	}
	s.Name = fields["名称"]
	s.Trigger = fields["适用条件"]
	s.Subject = fields["资料"]
	s.Description = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	s.Enabled = fields["启用"] == "是"
	if n := len([]rune(s.Name)); n < 1 || n > 32 {
		return s, fail(400, "技能名称须为 1～32 字。")
	}
	if len([]rune(s.Trigger)) < 1 || len([]rune(s.Trigger)) > 120 || len([]rune(s.Subject)) > 32 || len([]rune(s.Description)) < 1 || len([]rune(s.Description)) > 2000 {
		return s, fail(400, "技能条件、资料或说明长度无效。")
	}
	return s, nil
}
func scanSkill(row interface{ Scan(...any) error }) (Skill, error) {
	s := Skill{Tools: []string{"knowledge_search"}}
	e := row.Scan(&s.ID, &s.Name, &s.Trigger, &s.Subject, &s.Description, &s.Enabled, &s.Source)
	return s, e
}

const skillFields = "id,name,`trigger`,subject,description,enabled,source"

func resourceError(e error) error {
	if errors.Is(e, sql.ErrNoRows) {
		return fail(404, "未找到资源。")
	}
	return e
}
func uniqueError(e error) error {
	var me *mysql.MySQLError
	if errors.As(e, &me) && me.Number == 1062 {
		return fail(400, "本班已有同名技能。")
	}
	return e
}
func (a *App) configIdentity(r *http.Request, write bool) (User, string, error) {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return u, cid, e
	}
	if write {
		if u.Role != "teacher" {
			return u, cid, fail(403, "仅教师可修改配置。")
		}
		if e = a.checkOrigin(r); e != nil {
			return u, cid, e
		}
	}
	return u, cid, nil
}
func (a *App) promptHandler(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.configIdentity(r, r.Method == "PUT")
	if e != nil {
		return e
	}
	var body string
	if r.Method == "PUT" {
		var in struct {
			Body string `json:"body"`
		}
		if e = readJSON(w, r, &in); e != nil {
			return e
		}
		body = strings.TrimSpace(in.Body)
		if n := len([]rune(body)); n < 1 || n > 2000 {
			return fail(400, "提示词须为 1～2000 字。")
		}
		_, e = a.db.ExecContext(r.Context(), "INSERT INTO class_prompts(class_id,body,updated_by) VALUES(?,?,?) ON DUPLICATE KEY UPDATE body=VALUES(body),updated_by=VALUES(updated_by)", cid, body, u.ID)
	} else {
		e = a.db.QueryRowContext(r.Context(), "SELECT body FROM class_prompts WHERE class_id=?", cid).Scan(&body)
		if errors.Is(e, sql.ErrNoRows) {
			e = nil
		}
	}
	if e != nil {
		return e
	}
	return send(w, 200, map[string]string{"body": body})
}
func (a *App) skillsHandler(w http.ResponseWriter, r *http.Request) error {
	_, cid, e := a.configIdentity(r, r.Method != "GET")
	if e != nil {
		return e
	}
	if r.Method == "GET" {
		rows, e := a.db.QueryContext(r.Context(), "SELECT "+skillFields+" FROM skills WHERE class_id=? ORDER BY name,id", cid)
		if e != nil {
			return e
		}
		defer rows.Close()
		out := []Skill{}
		for rows.Next() {
			s, e := scanSkill(rows)
			if e != nil {
				return e
			}
			out = append(out, s)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		return send(w, 200, map[string]any{"skills": out})
	}
	var in struct {
		Source string `json:"source"`
	}
	if e = readJSON(w, r, &in); e != nil {
		return e
	}
	s, e := parseSkill(in.Source)
	if e != nil {
		return e
	}
	s.ID = newID()
	_, e = a.db.ExecContext(r.Context(), "INSERT INTO skills(id,class_id,source,name,`trigger`,subject,description,enabled) VALUES(?,?,?,?,?,?,?,?)", s.ID, cid, s.Source, s.Name, s.Trigger, s.Subject, s.Description, s.Enabled)
	if e != nil {
		return uniqueError(e)
	}
	return send(w, 201, s)
}
func (a *App) skillHandler(w http.ResponseWriter, r *http.Request) error {
	_, cid, e := a.configIdentity(r, r.Method != "GET")
	if e != nil {
		return e
	}
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		return fail(404, "未找到资源。")
	}
	if r.Method == "GET" {
		s, e := scanSkill(a.db.QueryRowContext(r.Context(), "SELECT "+skillFields+" FROM skills WHERE id=? AND class_id=?", id, cid))
		if e != nil {
			return resourceError(e)
		}
		return send(w, 200, s)
	}
	tx, e := a.db.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var exists string
	if e = tx.QueryRowContext(r.Context(), "SELECT id FROM skills WHERE id=? AND class_id=? FOR UPDATE", id, cid).Scan(&exists); e != nil {
		return resourceError(e)
	}
	var s Skill
	if r.Method == "DELETE" {
		_, e = tx.ExecContext(r.Context(), "DELETE FROM skills WHERE id=? AND class_id=?", id, cid)
	} else {
		var in struct {
			Source string `json:"source"`
		}
		if e = readJSON(w, r, &in); e != nil {
			return e
		}
		s, e = parseSkill(in.Source)
		if e != nil {
			return e
		}
		s.ID = id
		_, e = tx.ExecContext(r.Context(), "UPDATE skills SET source=?,name=?,`trigger`=?,subject=?,description=?,enabled=? WHERE id=? AND class_id=?", s.Source, s.Name, s.Trigger, s.Subject, s.Description, s.Enabled, id, cid)
	}
	if e != nil {
		return uniqueError(e)
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if r.Method == "DELETE" {
		w.WriteHeader(204)
		return nil
	}
	return send(w, 200, s)
}
func seedGuidance(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK('campusclaw_guidance_seed',10)").Scan(&locked); e != nil || locked != 1 {
		return errors.New("seed lock")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('campusclaw_guidance_seed')")
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM seed_versions WHERE version=2").Scan(&count); e != nil {
		return e
	}
	if count != 0 {
		return tx.Commit()
	}
	rows, e := tx.QueryContext(ctx, "SELECT id FROM classes")
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	s, _ := parseSkill(defaultSkillSource)
	for _, cid := range ids {
		if _, e = tx.ExecContext(ctx, "INSERT IGNORE INTO class_prompts(class_id,body) VALUES(?,?)", cid, "请用简体中文，以提问和分步提示帮助学生自行思考。"); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT IGNORE INTO skills(id,class_id,source,name,`trigger`,subject,description,enabled) VALUES(?,?,?,?,?,?,?,true)", newID(), cid, s.Source, s.Name, s.Trigger, s.Subject, s.Description); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO seed_versions(version) VALUES(2)"); e != nil {
		return e
	}
	return tx.Commit()
}
