package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChunkStrategies(t *testing.T) {
	samples := []string{"短文😀。", strings.Repeat("中文😀ab", 500), strings.Repeat("长句。\n\n", 300), "   ", "# 一级\n正文\n## 二级\n" + strings.Repeat("乙", 1000) + "\n### 三级\n尾片"}
	for _, strategy := range []string{"auto", "custom", "hierarchy"} {
		for _, body := range samples {
			o, e := (ChunkOptions{Strategy: strategy, MaxLength: 100, OverlapRatio: .5, Delimiter: "newline"}).normalized()
			if e != nil {
				t.Fatal(e)
			}
			text, basis, chunks := splitText(body, o)
			if basis != "original" || text != body {
				t.Fatal("original altered")
			}
			prev := -1
			for _, c := range chunks {
				if c.Start <= prev || c.End <= c.Start || c.End-c.Start > o.MaxLength || string([]rune(text)[c.Start:c.End]) != c.Text {
					t.Fatalf("bad chunk %#v", c)
				}
				prev = c.Start
			}
			if strings.TrimSpace(body) != "" && len(chunks) == 0 {
				t.Fatal("empty")
			}
		}
	}
	o, _ := (ChunkOptions{Strategy: "auto", MaxLength: 1, RemoveURLs: true}).normalized()
	text, _, c := splitText(strings.Repeat("a", 900), o)
	if len(c) != 2 || c[0].End != 800 || c[1].Start != 720 || len(text) != 900 {
		t.Fatal("800/80")
	}
	o, _ = (ChunkOptions{Strategy: "custom", MaxLength: 100, RemoveURLs: true, RemoveEmails: true, CollapseWhitespace: true}).normalized()
	text, basis, c := splitText("参考 https://example.com\n\n联系 a@b.com  正文", o)
	if text != "参考 联系 正文" || basis != "processed" || c[0].Text != text {
		t.Fatal(text, basis)
	}
	for _, o := range []ChunkOptions{{Strategy: "custom", MaxLength: 99}, {Strategy: "custom", MaxLength: 2001}, {Strategy: "custom", MaxLength: 800, OverlapRatio: .51}, {Strategy: "custom", MaxLength: 800, Delimiter: "bad"}, {Strategy: "bad"}} {
		if _, e := o.normalized(); e == nil {
			t.Fatal("invalid accepted")
		}
	}
	o, _ = (ChunkOptions{Strategy: "hierarchy"}).normalized()
	_, _, c = splitText("# 一\n甲\n## 二\n乙\n### 三\n丙", o)
	if len(c) != 3 || c[0].Text != "# 一\n甲\n" || !strings.HasPrefix(c[1].Text, "## 二") {
		t.Fatal("headings")
	}
}
func TestRRFAndReferences(t *testing.T) {
	one, two := 1, 2
	out := fuse([]Hit{{ID: "A", KeywordRank: &one}, {ID: "B", KeywordRank: &two}}, []Hit{{ID: "B", VectorRank: &one}, {ID: "C", VectorRank: &two}})
	if len(out) != 3 || out[0].ID != "B" || out[1].ID != "A" || out[2].ID != "C" {
		t.Fatal(out)
	}
	for _, s := range []string{"无引用", "引用[0]", "引用[3]", "引用[999999999999999999999]"} {
		if validCitations(s, 2) {
			t.Fatal(s)
		}
	}
	if !validCitations("依据[1]和[2]", 2) {
		t.Fatal("valid references")
	}
}
func TestEmbeddingValidationAndSecrecy(t *testing.T) {
	response := `{"data":[{"index":1,"embedding":[0,1]},{"index":0,"embedding":[1,0]}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing auth")
		}
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in["model"] != "test" {
			t.Error("model")
		}
		w.Write([]byte(response))
	}))
	defer server.Close()
	a := App{models: ModelConfig{EmbeddingURL: server.URL, EmbeddingKey: "test-secret", EmbeddingModel: "test", Dimension: 2}}
	out, e := a.embed(context.Background(), []string{"a", "b"})
	if e != nil || out[0][0] != 1 {
		t.Fatal(out, e)
	}
	for _, body := range []string{`{"data":[]}`, `{"data":[{"index":0,"embedding":[0,0]},{"index":1,"embedding":[0,1]}]}`, `{"data":[{"index":0,"embedding":[1]},{"index":1,"embedding":[0,1]}]}`, `{"data":[{"index":0,"embedding":[1,0]},{"index":0,"embedding":[0,1]}]}`} {
		response = body
		if _, e = a.embed(context.Background(), []string{"a", "b"}); e == nil {
			t.Fatal("invalid response")
		}
	}
}
