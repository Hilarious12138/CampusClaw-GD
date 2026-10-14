package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Conversation struct {
	ID      string    `json:"id"`
	AgentID string    `json:"agent_id"`
	Title   string    `json:"title"`
	Created time.Time `json:"created_at"`
	Updated time.Time `json:"updated_at"`
}
type StoredMessage struct {
	ID        string    `json:"id"`
	TurnID    string    `json:"turn_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Status    string    `json:"status"`
	Citations []Hit     `json:"citations"`
	SkillName string    `json:"skill_name"`
	Created   time.Time `json:"created_at"`
}

const conversationFields = "id,COALESCE(agent_id,''),title,created_at,updated_at"

func scanConversation(row interface{ Scan(...any) error }) (Conversation, error) {
	var c Conversation
	e := row.Scan(&c.ID, &c.AgentID, &c.Title, &c.Created, &c.Updated)
	return c, e
}
func (a *App) readConversation(r *http.Request, u User, cid string) (Conversation, error) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		return Conversation{}, fail(404, "未找到资源。")
	}
	c, e := scanConversation(a.db.QueryRowContext(r.Context(), "SELECT "+conversationFields+" FROM conversations WHERE id=? AND class_id=? AND user_id=?", id, cid, u.ID))
	return c, resourceError(e)
}
func (a *App) conversationsHandler(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	if r.Method == "GET" {
		rows, e := a.db.QueryContext(r.Context(), "SELECT "+conversationFields+" FROM conversations WHERE class_id=? AND user_id=? ORDER BY updated_at DESC,id", cid, u.ID)
		if e != nil {
			return e
		}
		defer rows.Close()
		out := []Conversation{}
		for rows.Next() {
			c, e := scanConversation(rows)
			if e != nil {
				return e
			}
			out = append(out, c)
		}
		if e = rows.Err(); e != nil {
			return e
		}
		return send(w, 200, map[string]any{"conversations": out})
	}
	if e = a.checkOrigin(r); e != nil {
		return e
	}
	var in struct {
		AgentID string `json:"agent_id"`
	}
	if e = readJSON(w, r, &in); e != nil {
		return e
	}
	if in.AgentID == "" {
		return fail(400, "请选择本班已启用的智能体。")
	}
	agent, e := a.readAgent(r, cid, in.AgentID, false)
	if e != nil {
		return e
	}
	id := newID()
	if _, e = a.db.ExecContext(r.Context(), "INSERT INTO conversations(id,class_id,user_id,agent_id) VALUES(?,?,?,?)", id, cid, u.ID, agent.ID); e != nil {
		return e
	}
	r.SetPathValue("id", id)
	c, e := a.readConversation(r, u, cid)
	if e != nil {
		return e
	}
	return send(w, 201, c)
}
func recoverMessages(ctx context.Context, conn *sql.Conn, id string) error {
	_, e := conn.ExecContext(ctx, "UPDATE conversation_messages SET status='failed' WHERE conversation_id=? AND role='assistant' AND status='generating'", id)
	return e
}
func (a *App) conversationDetail(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	c, e := a.readConversation(r, u, cid)
	if e != nil {
		return e
	}
	conn, e := a.db.Conn(r.Context())
	if e != nil {
		return e
	}
	var locked int
	if e = conn.QueryRowContext(r.Context(), "SELECT GET_LOCK(?,0)", "chat_"+c.ID).Scan(&locked); e == nil && locked == 1 {
		e = recoverMessages(r.Context(), conn, c.ID)
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", "chat_"+c.ID)
	}
	conn.Close()
	if e != nil {
		return e
	}
	rows, e := a.db.QueryContext(r.Context(), "SELECT id,turn_id,role,content,status,citations,config_snapshot,created_at FROM conversation_messages WHERE conversation_id=? ORDER BY created_at,IF(role='user',0,1),id", c.ID)
	if e != nil {
		return e
	}
	defer rows.Close()
	out := []StoredMessage{}
	for rows.Next() {
		var m StoredMessage
		var raw, snapshot []byte
		if e = rows.Scan(&m.ID, &m.TurnID, &m.Role, &m.Content, &m.Status, &raw, &snapshot, &m.Created); e != nil {
			return e
		}
		m.Citations = []Hit{}
		if e = json.Unmarshal(raw, &m.Citations); e != nil {
			return e
		}
		var config struct {
			SkillName string `json:"skill_name"`
		}
		if e = json.Unmarshal(snapshot, &config); e != nil {
			return e
		}
		m.SkillName = config.SkillName
		out = append(out, m)
	}
	if e = rows.Err(); e != nil {
		return e
	}
	return send(w, 200, map[string]any{"conversation": c, "messages": out})
}
func (a *App) conversationMessages(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	if e = a.checkOrigin(r); e != nil {
		return e
	}
	c, e := a.readConversation(r, u, cid)
	if e != nil {
		return e
	}
	var in struct {
		Content string `json:"content"`
	}
	if e = readJSON(w, r, &in); e != nil {
		return e
	}
	question := strings.TrimSpace(in.Content)
	if n := len([]rune(question)); n < 2 || n > 500 {
		return fail(400, "问题须为 2～500 字。")
	}
	claims, e := a.claims(r)
	if e != nil {
		return e
	}
	deadline := time.Now().Add(120 * time.Second)
	if claims.ExpiresAt.Time.Before(deadline) {
		deadline = claims.ExpiresAt.Time
	}
	ctx, cancel := context.WithDeadline(r.Context(), deadline)
	defer cancel()
	r = r.WithContext(ctx)
	conn, e := a.db.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked int
	if e = conn.QueryRowContext(ctx, "SELECT GET_LOCK(?,0)", "chat_"+c.ID).Scan(&locked); e != nil {
		return e
	}
	if locked != 1 {
		return fail(409, "此会话正在生成，请稍后再发。")
	}
	defer conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", "chat_"+c.ID)
	if e = recoverMessages(ctx, conn, c.ID); e != nil {
		return e
	}
	agent, e := a.readAgent(r, cid, c.AgentID, true)
	if e != nil {
		return e
	}
	chosen, e := a.agentSkill(r, cid, agent, question)
	if e != nil {
		return e
	}
	s := Skill{}
	if chosen != nil {
		s = *chosen
	}
	// Commit the current question before reading the bounded context window.
	turn, uid, aid := newID(), newID(), newID()
	snapshot, _ := json.Marshal(map[string]any{"agent_id": agent.ID, "agent_name": agent.Name, "prompt": agent.Prompt, "skill_name": s.Name, "skill_source": s.Source, "material_ids": agent.MaterialIDs})
	citations := []byte("[]")
	tx, e := conn.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "INSERT INTO conversation_messages(id,conversation_id,turn_id,role,content,status,citations,config_snapshot) VALUES(?,?,?,'user',?,'completed',JSON_ARRAY(),?), (?,?,?,'assistant','','generating',?,?)", uid, c.ID, turn, question, snapshot, aid, c.ID, turn, citations, snapshot); e != nil {
		return e
	}
	title := []rune(question)
	if len(title) > 24 {
		title = title[:24]
	}
	if _, e = tx.ExecContext(ctx, "UPDATE conversations SET title=IF((SELECT COUNT(*) FROM conversation_messages WHERE conversation_id=? AND role='user')=1,?,title),updated_at=UTC_TIMESTAMP(6) WHERE id=?", c.ID, string(title), c.ID); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	status := "failed"
	var answer strings.Builder
	defer func() {
		if status != "completed" {
			saveCtx, done := context.WithTimeout(context.Background(), 4*time.Second)
			defer done()
			_, _ = conn.ExecContext(saveCtx, "UPDATE conversation_messages SET content=?,status=? WHERE id=? AND status='generating'", answer.String(), status, aid)
		}
	}()
	scoped := withMaterials(r, agent.MaterialIDs)
	result, e := a.search(scoped, u, cid, question, "hybrid", 4)
	if e != nil {
		return e
	}
	hits := []Hit{}
	for _, hit := range result.Hits {
		if strings.TrimSpace(hit.Text) == "" {
			continue
		}
		if s.Subject != "" && !strings.Contains(hit.Title, s.Subject) {
			continue
		}
		hits = append(hits, hit)
	}
	result.Hits = hits
	citations, _ = json.Marshal(hits)
	if _, e = conn.ExecContext(ctx, "UPDATE conversation_messages SET citations=? WHERE id=?", citations, aid); e != nil {
		return e
	}
	rc := http.NewResponseController(w)
	if e = rc.SetWriteDeadline(deadline); e != nil {
		return e
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)
	emit := func(event string, value any) error {
		b, _ := json.Marshal(value)
		if _, e = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b); e != nil {
			return e
		}
		return rc.Flush()
	}
	conclusion := "ok"
	var skillMeta any
	if chosen != nil {
		skillMeta = map[string]string{"id": s.ID, "name": s.Name}
	}
	if e = emit("meta", map[string]any{"turn_id": turn, "status": conclusion, "skill_name": s.Name, "skill": skillMeta, "agent_id": agent.ID, "citations": result.Hits, "index_status": result.IndexStatus}); e != nil {
		status = "cancelled"
		return nil
	}
	messages := []Message{{"system", systemForAgent(agent, chosen, len(result.Hits) > 0)}}
	rows, e := conn.QueryContext(ctx, `SELECT m.role,m.content FROM conversation_messages m WHERE m.conversation_id=? AND TRIM(m.content)<>'' AND m.role IN ('user','assistant') AND (m.turn_id=? OR (m.status='completed' AND EXISTS(SELECT 1 FROM conversation_messages a WHERE a.turn_id=m.turn_id AND a.role='assistant' AND a.status='completed'))) ORDER BY m.created_at DESC,IF(m.role='assistant',0,1),m.id DESC LIMIT 6`, c.ID, turn)
	if e != nil {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "HISTORY_FAILED", "message": "读取历史失败。"})
		return nil
	}
	history := []Message{}
	for rows.Next() {
		var m Message
		if e = rows.Scan(&m.Role, &m.Content); e != nil {
			rows.Close()
			_ = emit("error", map[string]string{"turn_id": turn, "code": "HISTORY_FAILED", "message": "读取历史失败。"})
			return nil
		}
		rs := []rune(m.Content)
		if len(rs) > 500 {
			m.Content = string(rs[:500])
		}
		history = append(history, m)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "HISTORY_FAILED", "message": "读取历史失败。"})
		return nil
	}
	var evidence strings.Builder
	for i, h := range result.Hits {
		rs := []rune(h.Text)
		if len(rs) > 800 {
			rs = rs[:800]
		}
		fmt.Fprintf(&evidence, "[%d] 材料《%s》切片%d\n%s\n", i+1, h.Title, h.Index, string(rs))
	}
	for i := len(history) - 1; i >= 0; i-- {
		m := history[i]
		if i == 0 && m.Role == "user" {
			m.Content += "\n<evidence>\n" + evidence.String() + "</evidence>"
		}
		messages = append(messages, m)
	}
	upstream, e := a.openModelStream(ctx, messages)
	if e != nil {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "MODEL_FAILED", "message": "对话服务暂时不可用。"})
		return nil
	}
	defer upstream.Close()
	type piece struct {
		text string
		err  error
		done bool
	}
	pieces := make(chan piece, 1)
	go func() {
		defer close(pieces)
		err := parseModelStream(upstream, func(text string) error {
			select {
			case pieces <- piece{text: text}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		select {
		case pieces <- piece{err: err, done: true}:
		case <-ctx.Done():
		}
	}()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	generationOK := false
	cleaner := citationCleaner{}
	writeDelta := func(text string) error {
		if text == "" {
			return nil
		}
		answer.WriteString(text)
		return emit("delta", map[string]string{"text": text})
	}
	for running := true; running; {
		select {
		case <-ctx.Done():
			status = "cancelled"
			_ = emit("error", map[string]string{"turn_id": turn, "code": "CANCELLED", "message": "生成已中止或超时。"})
			return nil
		case <-heartbeat.C:
			if _, e = fmt.Fprint(w, ": heartbeat\n\n"); e != nil || rc.Flush() != nil {
				status = "cancelled"
				return nil
			}
		case item, ok := <-pieces:
			if !ok {
				running = false
				break
			}
			if item.done {
				if item.err != nil {
					_ = emit("error", map[string]string{"turn_id": turn, "code": "MODEL_FAILED", "message": "生成中断，请查看会话状态后重试。"})
					return nil
				}
				generationOK = true
				continue
			}
			if answer.Len()+len(item.text) > 16000 {
				_ = emit("error", map[string]string{"turn_id": turn, "code": "OUTPUT_LIMIT", "message": "回答超过长度限制。"})
				return nil
			}
			text := item.text
			if len(result.Hits) == 0 {
				text = cleaner.Push(text)
			}
			if e = writeDelta(text); e != nil {
				status = "cancelled"
				return nil
			}
		}
	}
	if len(result.Hits) == 0 {
		if e = writeDelta(cleaner.Finish()); e != nil {
			status = "cancelled"
			return nil
		}
	}
	final := answer.String()
	if !generationOK || strings.TrimSpace(final) == "" || (len(result.Hits) > 0 && !validStreamCitations(final, len(result.Hits))) || (len(result.Hits) == 0 && strings.TrimSpace(final) == notFound) {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "INVALID_CITATIONS", "message": "回答引用校验失败，草稿未作为成功答案保存。"})
		return nil
	}
	fresh, freshClass, identityError := a.retrievalIdentity(r)
	if identityError != nil || freshClass != cid || fresh.ID != u.ID {
		status = "cancelled"
		_ = emit("error", map[string]string{"turn_id": turn, "code": "UNAUTHORIZED", "message": "登录已失效。"})
		return nil
	}
	if _, e = a.readConversation(r, u, cid); e != nil {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "NOT_FOUND", "message": "未找到资源。"})
		return nil
	}
	saved, saveError := conn.ExecContext(ctx, "UPDATE conversation_messages SET content=?,status='completed',citations=? WHERE id=? AND status='generating'", final, citations, aid)
	var affected int64
	if saveError == nil {
		affected, saveError = saved.RowsAffected()
	}
	if saveError != nil || affected != 1 {
		_ = emit("error", map[string]string{"turn_id": turn, "code": "SAVE_FAILED", "message": "保存失败，回答未完成。"})
		return nil
	}
	status = "completed"
	_ = emit("done", map[string]string{"turn_id": turn, "message_id": aid, "status": status})
	return nil
}
