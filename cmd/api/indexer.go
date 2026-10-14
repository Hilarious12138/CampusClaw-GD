package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// Durable jobs live beside material contents. A connection-scoped advisory lock
// is the lease: process death releases it, and indexing jobs become recoverable.
func (a *App) indexWorker(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		a.indexNext(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) indexNext(ctx context.Context) {
	conn, e := a.db.Conn(ctx)
	if e != nil {
		return
	}
	defer conn.Close()
	var locked int
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK('campusclaw_index_worker',0)").Scan(&locked) != nil || locked != 1 {
		return
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('campusclaw_index_worker')")
	var mid string
	e = conn.QueryRowContext(ctx, "SELECT material_id FROM material_indexes WHERE state IN ('pending','indexing') AND next_attempt<=UTC_TIMESTAMP(6) ORDER BY next_attempt,material_id LIMIT 1").Scan(&mid)
	if e != nil {
		return
	}
	if conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", "idx_"+mid).Scan(&locked) != nil || locked != 1 {
		return
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", "idx_"+mid)
	a.processIndex(ctx, conn, mid)
}
func (a *App) processIndex(ctx context.Context, conn *sql.Conn, mid string) {
	jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	_, e := conn.ExecContext(jobCtx, "UPDATE material_indexes SET state='indexing',attempts=attempts+1 WHERE material_id=?", mid)
	if e != nil {
		return
	}
	if e = a.buildIndex(jobCtx, conn, mid); e != nil {
		// Keep non-ready rows invisible even if remote cleanup fails. Retry always
		// confirms remote deletion before replacing chunks; original is never deleted.
		cleanCtx, done := context.WithTimeout(context.Background(), 8*time.Second)
		_ = a.deleteVectors(cleanCtx, mid)
		done()
		cleanCtx, done = context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = conn.ExecContext(cleanCtx, "UPDATE knowledge_chunks SET index_status='failed' WHERE material_id=?", mid)
		_, _ = conn.ExecContext(cleanCtx, "UPDATE material_indexes SET state=IF(attempts<3,'pending','failed'),next_attempt=DATE_ADD(UTC_TIMESTAMP(6),INTERVAL 15 SECOND),last_error='索引失败，可稍后重建重试' WHERE material_id=?", mid)
		done()
	}
}
func (a *App) buildIndex(ctx context.Context, conn *sql.Conn, mid string) error {
	var cid, body, raw string
	var version int
	e := conn.QueryRowContext(ctx, "SELECT m.class_id,mc.body,i.options,i.version FROM material_indexes i JOIN materials m ON m.id=i.material_id JOIN material_contents mc ON mc.material_id=m.id WHERE m.id=?", mid).Scan(&cid, &body, &raw, &version)
	if e != nil {
		return e
	}
	var o ChunkOptions
	if e = json.Unmarshal([]byte(raw), &o); e != nil {
		return e
	}
	o, e = o.normalized()
	if e != nil {
		return e
	}
	text, basis, chunks := splitText(body, o)
	if len(chunks) == 0 {
		return errors.New("empty chunks")
	}
	if e = a.ensureCollection(ctx); e != nil {
		return e
	}
	if e = a.deleteVectors(ctx, mid); e != nil {
		return e
	}
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT id FROM knowledge_chunks WHERE material_id=? AND version=? ORDER BY chunk_index", mid, version)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(ids) != len(chunks) {
		if _, e = tx.ExecContext(ctx, "DELETE FROM knowledge_chunks WHERE material_id=?", mid); e != nil {
			return e
		}
		ids = make([]string, len(chunks))
		for i, ch := range chunks {
			ids[i] = uuid()
			if _, e = tx.ExecContext(ctx, "INSERT INTO knowledge_chunks(id,material_id,knowledge_entry_id,class_id,version,chunk_index,start_char,end_char,offset_basis,chunk_text) VALUES(?,?,?,?,?,?,?,?,?,?)", ids[i], mid, mid, cid, version, i+1, ch.Start, ch.End, basis, ch.Text); e != nil {
				return e
			}
		}
	} else {
		if _, e = tx.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status='pending' WHERE material_id=?", mid); e != nil {
			return e
		}
	}
	if basis == "original" {
		text = ""
	}
	if _, e = tx.ExecContext(ctx, "UPDATE material_indexes SET processed_text=?,offset_basis=? WHERE material_id=?", text, basis, mid); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	for start := 0; start < len(chunks); start += 8 {
		end := start + 8
		if end > len(chunks) {
			end = len(chunks)
		}
		input := []string{}
		for _, ch := range chunks[start:end] {
			input = append(input, ch.Text)
		}
		vectors, err := a.embed(ctx, input)
		if err != nil {
			return err
		}
		points := []any{}
		for j, v := range vectors {
			i := start + j
			points = append(points, map[string]any{"id": ids[i], "vector": v, "payload": map[string]any{"class_id": cid, "material_id": mid, "knowledge_entry_id": mid, "chunk_id": ids[i], "chunk_index": i + 1}})
		}
		if e = a.qdrant(ctx, "PUT", "/points?wait=true", map[string]any{"points": points}, nil); e != nil {
			return e
		}
	}
	tx, e = conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE knowledge_chunks SET index_status='ready' WHERE material_id=? AND version=?", mid, version); e != nil {
		return e
	}
	result, e := tx.ExecContext(ctx, "UPDATE material_indexes SET state='ready',last_error='' WHERE material_id=? AND version=?", mid, version)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return errors.New("index version changed")
	}
	return tx.Commit()
}
func (a *App) reindex(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	if u.Role != "teacher" {
		return fail(403, "仅教师可重建索引。")
	}
	if e = a.checkOrigin(r); e != nil {
		return e
	}
	var o ChunkOptions
	if e = readJSON(w, r, &o); e != nil {
		return e
	}
	o, e = o.normalized()
	if e != nil {
		return e
	}
	mid := r.PathValue("id")
	if !idPattern.MatchString(mid) {
		return fail(404, "未找到材料。")
	}
	conn, e := a.db.Conn(r.Context())
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(r.Context(), "SELECT GET_LOCK(?,0)", "idx_"+mid).Scan(&locked); e != nil {
		return e
	}
	if locked != 1 {
		return fail(409, "索引正在处理，请稍后重试。")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", "idx_"+mid)
	tx, e := conn.BeginTx(r.Context(), nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var found string
	e = tx.QueryRowContext(r.Context(), "SELECT m.id FROM materials m JOIN class_memberships cm ON cm.class_id=m.class_id JOIN users u ON u.id=cm.user_id WHERE m.id=? AND m.class_id=? AND cm.user_id=? AND u.role='teacher' FOR SHARE", mid, cid, u.ID).Scan(&found)
	if errors.Is(e, sql.ErrNoRows) {
		return fail(404, "未找到材料。")
	}
	if e != nil {
		return e
	}
	raw, _ := json.Marshal(o)
	if _, e = tx.ExecContext(r.Context(), "UPDATE material_indexes SET version=version+1,options=?,state='pending',attempts=0,next_attempt=UTC_TIMESTAMP(6),last_error='' WHERE material_id=?", raw, mid); e != nil {
		return e
	}
	if _, e = tx.ExecContext(r.Context(), "UPDATE knowledge_chunks SET index_status='pending' WHERE material_id=?", mid); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	return send(w, 202, map[string]string{"state": "pending"})
}
