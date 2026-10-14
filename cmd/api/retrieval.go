package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"unicode"
)

const notFound = "资料中未找到相关内容"

func readJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	if e := d.Decode(v); e != nil {
		return fail(400, "请求 JSON 无效。")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return fail(400, "请求 JSON 无效。")
	}
	return nil
}
func (a *App) retrievalIdentity(r *http.Request) (User, string, error) {
	u, e := a.user(r)
	if e != nil {
		return u, "", e
	}
	u, e = a.withClasses(r, u)
	if e != nil {
		return u, "", e
	}
	if len(u.Classes) == 0 {
		return u, "", fail(403, "账号未加入班级。")
	}
	if len(u.Classes) != 1 {
		return u, "", fail(409, "当前账号有多个班级，无法唯一确定检索范围。")
	}
	return u, u.Classes[0].ID, nil
}

type Hit struct {
	MaterialID   string   `json:"material_id"`
	Title        string   `json:"material_title"`
	ID           string   `json:"chunk_id"`
	Index        int      `json:"chunk_index"`
	Start        int      `json:"start_char"`
	End          int      `json:"end_char"`
	Basis        string   `json:"offset_basis"`
	Excerpt      string   `json:"excerpt"`
	URL          string   `json:"material_url"`
	Score        float64  `json:"score"`
	KeywordScore *float64 `json:"keyword_score"`
	KeywordRank  *int     `json:"keyword_rank"`
	VectorScore  *float64 `json:"vector_score"`
	VectorRank   *int     `json:"vector_rank"`
	RRF          *float64 `json:"rrf_score"`
	Text         string   `json:"-"`
}
type SearchResult struct {
	Hits        []Hit          `json:"hits"`
	Message     string         `json:"message"`
	Mode        string         `json:"mode"`
	QueryVector map[string]any `json:"query_vector"`
	IndexStatus map[string]int `json:"index_status"`
}

const hitSelect = `SELECT c.material_id,m.filename,c.id,c.chunk_index,c.start_char,c.end_char,c.offset_basis,c.chunk_text FROM knowledge_chunks c JOIN material_indexes i ON i.material_id=c.material_id AND i.version=c.version JOIN materials m ON m.id=c.material_id AND m.class_id=c.class_id JOIN class_memberships cm ON cm.class_id=m.class_id WHERE c.index_status='ready' AND i.state='ready' AND c.class_id=? AND cm.user_id=?`

func scanHit(row interface{ Scan(...any) error }) (Hit, error) {
	var h Hit
	e := row.Scan(&h.MaterialID, &h.Title, &h.ID, &h.Index, &h.Start, &h.End, &h.Basis, &h.Text)
	rs := []rune(h.Text)
	if len(rs) > 200 {
		rs = rs[:200]
	}
	h.Excerpt = string(rs)
	h.URL = "/?material=" + h.MaterialID + "&chunk=" + h.ID
	return h, e
}
func validQuery(query string) bool {
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return true
		}
	}
	return false
}
func (a *App) keyword(r *http.Request, u User, cid, query string) ([]Hit, error) {
	out := []Hit{}
	if !validQuery(query) {
		return out, nil
	}
	scope, scopeArgs := scopeSQL(r)
	args := append([]any{query, cid, u.ID, query}, scopeArgs...)
	rows, e := a.db.QueryContext(r.Context(), `SELECT c.id,MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE) AS relevance FROM knowledge_chunks c JOIN material_indexes i ON i.material_id=c.material_id AND i.version=c.version JOIN materials m ON m.id=c.material_id AND m.class_id=c.class_id JOIN class_memberships cm ON cm.class_id=m.class_id WHERE c.class_id=? AND cm.user_id=? AND c.index_status='ready' AND i.state='ready' AND MATCH(c.chunk_text) AGAINST(? IN NATURAL LANGUAGE MODE)>0`+scope+` ORDER BY relevance DESC,c.id LIMIT 50`, args...)
	if e != nil {
		return nil, e
	}
	type candidate struct {
		id    string
		score float64
	}
	cs := []candidate{}
	for rows.Next() {
		var c candidate
		if e = rows.Scan(&c.id, &c.score); e != nil {
			rows.Close()
			return nil, e
		}
		cs = append(cs, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	for _, c := range cs {
		h, err := scanHit(a.db.QueryRowContext(r.Context(), hitSelect+" AND c.id=?", cid, u.ID, c.id))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !allowedMaterial(r, h.MaterialID) {
			continue
		}
		rank := len(out) + 1
		score := c.score
		h.KeywordRank = &rank
		h.KeywordScore = &score
		h.Score = score
		out = append(out, h)
	}
	return out, nil
}
func (a *App) vector(r *http.Request, u User, cid, query string) ([]Hit, error) {
	vectors, e := a.embed(r.Context(), []string{query})
	if e != nil {
		return nil, e
	}
	var res struct {
		Result []struct {
			ID    string  `json:"id"`
			Score float64 `json:"score"`
		} `json:"result"`
	}
	must := []any{map[string]any{"key": "class_id", "match": map[string]string{"value": cid}}}
	if ids, scoped := materialScope(r); scoped {
		must = append(must, map[string]any{"key": "material_id", "match": map[string]any{"any": ids}})
	}
	e = a.qdrant(r.Context(), "POST", "/points/search", map[string]any{"vector": vectors[0], "filter": map[string]any{"must": must}, "limit": 50, "score_threshold": 0.35, "with_payload": false, "with_vector": false}, &res)
	if e != nil {
		return nil, e
	}
	sort.Slice(res.Result, func(i, j int) bool {
		if res.Result[i].Score == res.Result[j].Score {
			return res.Result[i].ID < res.Result[j].ID
		}
		return res.Result[i].Score > res.Result[j].Score
	})
	out := []Hit{}
	seen := map[string]bool{}
	for _, c := range res.Result {
		if c.Score < 0.35 || math.IsNaN(c.Score) || math.IsInf(c.Score, 0) || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		h, err := scanHit(a.db.QueryRowContext(r.Context(), hitSelect+" AND c.id=?", cid, u.ID, c.ID))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !allowedMaterial(r, h.MaterialID) {
			continue
		}
		rank := len(out) + 1
		score := c.Score
		h.VectorRank = &rank
		h.VectorScore = &score
		h.Score = score
		out = append(out, h)
	}
	return out, nil
}
func fuse(keyword, vector []Hit) []Hit {
	merged := map[string]Hit{}
	for _, h := range keyword {
		merged[h.ID] = h
	}
	for _, h := range vector {
		if old, ok := merged[h.ID]; ok {
			old.VectorRank = h.VectorRank
			old.VectorScore = h.VectorScore
			merged[h.ID] = old
		} else {
			merged[h.ID] = h
		}
	}
	out := []Hit{}
	for _, h := range merged {
		score := 0.0
		if h.KeywordRank != nil {
			score += 1 / float64(60+*h.KeywordRank)
		}
		if h.VectorRank != nil {
			score += 1 / float64(60+*h.VectorRank)
		}
		h.Score = score
		h.RRF = &score
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	return out
}
func (a *App) search(r *http.Request, u User, cid, query, mode string, top int) (SearchResult, error) {
	out := SearchResult{Hits: []Hit{}, Mode: mode, IndexStatus: map[string]int{}, QueryVector: map[string]any{"generated": false, "dimension": 0}}
	if mode == "" {
		mode = "hybrid"
		out.Mode = mode
	}
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 1 || len([]rune(query)) > 500 || top < 1 || top > 20 || (mode != "keyword" && mode != "vector" && mode != "hybrid") {
		return out, fail(400, "问题须为 1～500 字，top_k 为 1～20，模式须为 keyword/vector/hybrid。")
	}
	if ids, scoped := materialScope(r); scoped && len(ids) == 0 {
		out.Message = notFound
		return out, nil
	}
	scope, scopeArgs := scopeSQL(r)
	rows, e := a.db.QueryContext(r.Context(), "SELECT i.state,COUNT(*) FROM material_indexes i JOIN materials m ON m.id=i.material_id JOIN class_memberships cm ON cm.class_id=m.class_id WHERE m.class_id=? AND cm.user_id=?"+scope+" GROUP BY i.state", append([]any{cid, u.ID}, scopeArgs...)...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var state string
		var n int
		if e = rows.Scan(&state, &n); e != nil {
			rows.Close()
			return out, e
		}
		out.IndexStatus[state] = n
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	var k, v []Hit
	if mode != "vector" {
		k, e = a.keyword(r, u, cid, query)
		if e != nil {
			return out, e
		}
	}
	if mode != "keyword" {
		v, e = a.vector(r, u, cid, query)
		if e != nil {
			return out, e
		}
		out.QueryVector = map[string]any{"generated": true, "dimension": a.models.Dimension}
	}
	switch mode {
	case "keyword":
		out.Hits = k
	case "vector":
		out.Hits = v
	default:
		out.Hits = fuse(k, v)
	}
	if len(out.Hits) > top {
		out.Hits = out.Hits[:top]
	}
	if len(out.Hits) == 0 {
		out.Message = notFound
	}
	return out, nil
}
func (a *App) searchHandler(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	var in struct {
		Query string `json:"query"`
		Mode  string `json:"mode"`
		Top   *int   `json:"top_k"`
	}
	if e = readJSON(w, r, &in); e != nil {
		return e
	}
	top := 5
	if in.Top != nil {
		top = *in.Top
	}
	out, e := a.search(r, u, cid, in.Query, in.Mode, top)
	if e != nil {
		return e
	}
	return send(w, 200, out)
}
func (a *App) chunkDetail(w http.ResponseWriter, r *http.Request) error {
	u, cid, e := a.retrievalIdentity(r)
	if e != nil {
		return e
	}
	h, e := scanHit(a.db.QueryRowContext(r.Context(), hitSelect+" AND c.id=?", cid, u.ID, r.PathValue("id")))
	if errors.Is(e, sql.ErrNoRows) {
		return fail(404, "未找到切片。")
	}
	if e != nil {
		return e
	}
	return send(w, 200, struct {
		Hit
		Text string `json:"chunk_text"`
	}{h, h.Text})
}
