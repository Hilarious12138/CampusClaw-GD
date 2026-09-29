package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func openDB(c Config) (*sql.DB, error) {
	cfg := mysql.NewConfig()
	cfg.User = c.DBUser
	cfg.Passwd = c.DBPassword
	cfg.Net = "tcp"
	cfg.Addr = c.DBAddr
	cfg.DBName = c.DBName
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = 2 * time.Second
	cfg.ReadTimeout = 12 * time.Second
	cfg.WriteTimeout = 12 * time.Second
	cfg.Params = map[string]string{"charset": "utf8mb4", "time_zone": "'+00:00'"}
	db, e := sql.Open("mysql", cfg.FormatDSN())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}
func migrate(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK('campusclaw_migrate',20)").Scan(&locked); e != nil || locked != 1 {
		return errors.New("迁移锁不可用")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('campusclaw_migrate')")
	if _, e = conn.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS schema_versions(version INT PRIMARY KEY)"); e != nil {
		return e
	}
	var version int
	e = conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0) FROM schema_versions").Scan(&version)
	if e != nil {
		return e
	}
	if version > 1 {
		return errors.New("不支持的迁移版本")
	}
	if version == 1 {
		return nil
	}
	for _, statement := range strings.Split(schema, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, e = conn.ExecContext(ctx, statement); e != nil {
			return e
		}
	}
	_, e = conn.ExecContext(ctx, "INSERT INTO schema_versions VALUES(1)")
	return e
}
func seed(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, e := db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK('campusclaw_seed',20)").Scan(&locked); e != nil || locked != 1 {
		return errors.New("种子锁不可用")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('campusclaw_seed')")
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM seed_versions WHERE version=1").Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return tx.Commit()
	}
	accounts := []string{"teacher_a", "student_a", "teacher_b", "student_b"}
	hashes := make([]string, 4)
	for i, a := range accounts {
		p := os.Getenv("SEED_" + strings.ToUpper(a) + "_PASSWORD")
		if len(p) < 12 {
			return errors.New("首次种子初始化需要四个至少12字符的密码")
		}
		hashes[i], e = passwordHash(p)
		if e != nil {
			return e
		}
	}
	for i, code := range []string{"A", "B"} {
		cid, tid, sid, mid := newID(), newID(), newID(), newID()
		if _, e = tx.ExecContext(ctx, "INSERT INTO classes VALUES(?,?,?)", cid, code, "软件工程 · "+code+" 班"); e != nil {
			return e
		}
		for offset, role := range []string{"teacher", "student"} {
			id, name := tid, code+"班教师"
			if offset == 1 {
				id = sid
				name = code + "班学生"
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO users VALUES(?,?,?,?,?)", id, accounts[i*2+offset], name, hashes[i*2+offset], role); e != nil {
				return e
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO class_memberships VALUES(?,?)", id, cid); e != nil {
				return e
			}
		}
		body := fmt.Sprintf("# %s班 · 软件工程学习指南\n\n欢迎来到 CampusClaw。\n\n这是一份%s班专属的合成教学材料。\n\n## 本周学习目标\n1. 理解需求规约与验收场景。\n2. 学会按角色和班级组织教学材料。\n3. 用一次小迭代验证真实的软件行为。\n\n## 课前准备\n阅读本班材料，记录自己的思考。\n", code, code)
		if _, e = tx.ExecContext(ctx, "INSERT INTO materials(id,class_id,uploader_id,filename,type,size_bytes) VALUES(?,?,?,?,?,?)", mid, cid, tid, code+"班-学习指南.md", "md", len([]byte(body))); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO material_contents VALUES(?,?,?)", mid, body, []byte(body)); e != nil {
			return e
		}
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO seed_versions(version) VALUES(1)"); e != nil {
		return e
	}
	return tx.Commit()
}
