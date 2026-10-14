package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type ModelConfig struct {
	EmbeddingURL, EmbeddingKey, EmbeddingModel, ChatURL, ChatKey, ChatModel, QdrantURL string
	Dimension                                                                          int
}

func modelConfig() ModelConfig {
	dim, _ := strconv.Atoi(os.Getenv("EMBEDDING_DIM"))
	if dim == 0 {
		dim = 2048
	}
	env := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}
	return ModelConfig{strings.TrimRight(os.Getenv("EMBEDDING_BASE_URL"), "/"), os.Getenv("EMBEDDING_API_KEY"), env("EMBEDDING_MODEL", "course-embedding"), strings.TrimRight(env("CHAT_BASE_URL", os.Getenv("EMBEDDING_BASE_URL")), "/"), env("CHAT_API_KEY", os.Getenv("EMBEDDING_API_KEY")), env("CHAT_MODEL", "course-chat"), strings.TrimRight(env("QDRANT_URL", "http://qdrant:6333"), "/"), dim}
}

// Never propagate provider bodies, URLs or authentication headers into logs/errors.
func remoteJSON(ctx context.Context, method, url, key string, input, output any) error {
	var body io.Reader
	if input != nil {
		b, e := json.Marshal(input)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	r, e := http.NewRequestWithContext(ctx, method, url, body)
	if e != nil {
		return fail(503, "模型或向量服务配置无效。")
	}
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	client := http.Client{Timeout: 35 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(r)
	if e != nil {
		return fail(503, "模型或向量服务暂时不可用。")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fail(503, "模型或向量服务请求失败。")
	}
	if output != nil {
		if e = json.NewDecoder(io.LimitReader(res.Body, 32<<20)).Decode(output); e != nil {
			return fail(503, "模型或向量服务响应无效。")
		}
	}
	return nil
}
func (a *App) embed(ctx context.Context, texts []string) ([][]float64, error) {
	c := a.models
	if c.EmbeddingURL == "" || c.EmbeddingKey == "" {
		return nil, fail(503, "尚未配置嵌入服务。")
	}
	var res struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if e := remoteJSON(ctx, "POST", c.EmbeddingURL+"/embeddings", c.EmbeddingKey, map[string]any{"model": c.EmbeddingModel, "input": texts}, &res); e != nil {
		return nil, e
	}
	if len(res.Data) != len(texts) {
		return nil, fail(503, "嵌入返回数量不符。")
	}
	out := make([][]float64, len(texts))
	for _, d := range res.Data {
		if d.Index < 0 || d.Index >= len(out) || out[d.Index] != nil || len(d.Embedding) != c.Dimension {
			return nil, fail(503, "嵌入维度或顺序无效。")
		}
		norm := 0.0
		for _, v := range d.Embedding {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, fail(503, "嵌入向量无效。")
			}
			norm += v * v
		}
		if norm == 0 || math.IsInf(norm, 0) {
			return nil, fail(503, "嵌入向量无效。")
		}
		out[d.Index] = d.Embedding
	}
	return out, nil
}

const collection = "/collections/campusclaw_chunks"

func (a *App) qdrant(ctx context.Context, method, path string, in, out any) error {
	return remoteJSON(ctx, method, a.models.QdrantURL+collection+path, "", in, out)
}
func (a *App) ensureCollection(ctx context.Context) error {
	var res struct {
		Result struct {
			Config struct {
				Params struct {
					Vectors struct {
						Size     int    `json:"size"`
						Distance string `json:"distance"`
					} `json:"vectors"`
				} `json:"params"`
			} `json:"config"`
		} `json:"result"`
	}
	if e := a.qdrant(ctx, "GET", "", nil, &res); e != nil {
		if e = a.qdrant(ctx, "PUT", "", map[string]any{"vectors": map[string]any{"size": a.models.Dimension, "distance": "Cosine"}}, nil); e != nil {
			return e
		}
		if e = a.qdrant(ctx, "GET", "", nil, &res); e != nil {
			return e
		}
	}
	if res.Result.Config.Params.Vectors.Size != a.models.Dimension || res.Result.Config.Params.Vectors.Distance != "Cosine" {
		return fail(503, "向量集合维度或距离配置不匹配。")
	}
	return a.qdrant(ctx, "PUT", "/index?wait=true", map[string]any{"field_name": "class_id", "field_schema": "keyword"}, nil)
}
func (a *App) deleteVectors(ctx context.Context, mid string) error {
	return a.qdrant(ctx, "POST", "/points/delete?wait=true", map[string]any{"filter": map[string]any{"must": []any{map[string]any{"key": "material_id", "match": map[string]string{"value": mid}}}}}, nil)
}
func uuid() string {
	s := newID()
	return fmt.Sprintf("%s-%s-%s-%s-%s", s[:8], s[8:12], s[12:16], s[16:20], s[20:])
}
