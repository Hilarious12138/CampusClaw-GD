package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

// parseModelStream accepts complete SSE frames, never assumes a network read is
// a character/token boundary. A clean finish marker is required before success.
func parseModelStream(input io.Reader, emit func(string) error) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	data := []string{}
	finished := false
	done := false
	dispatch := func() error {
		if len(data) == 0 {
			return nil
		}
		raw := strings.Join(data, "\n")
		data = nil
		if strings.TrimSpace(raw) == "[DONE]" {
			done = true
			return nil
		}
		var frame struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				Finish *string `json:"finish_reason"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal([]byte(raw), &frame) != nil || len(frame.Error) > 0 {
			return errors.New("invalid stream")
		}
		for _, choice := range frame.Choices {
			if choice.Delta.Content != "" {
				if e := emit(choice.Delta.Content); e != nil {
					return e
				}
			}
			if choice.Finish != nil {
				if *choice.Finish != "stop" {
					return errors.New("incomplete generation")
				}
				finished = true
			}
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if e := dispatch(); e != nil {
				return e
			}
			if done {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}
	if e := dispatch(); e != nil {
		return e
	}
	if done || finished {
		return nil
	}
	return errors.New("stream ended without completion")
}

type idleBody struct {
	io.ReadCloser
	timer *time.Timer
}

func (b *idleBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	if n > 0 {
		b.timer.Reset(30 * time.Second)
	}
	return n, e
}
func (b *idleBody) Close() error { b.timer.Stop(); return b.ReadCloser.Close() }
func (a *App) openModelStream(ctx context.Context, messages []Message) (io.ReadCloser, error) {
	c := a.models
	if c.ChatURL == "" || c.ChatKey == "" {
		return nil, fail(503, "尚未配置对话服务。")
	}
	body, _ := json.Marshal(map[string]any{"model": c.ChatModel, "messages": messages, "stream": true, "temperature": 0, "max_tokens": 700})
	r, e := http.NewRequestWithContext(ctx, "POST", c.ChatURL+"/chat/completions", bytes.NewReader(body))
	if e != nil {
		return nil, fail(503, "对话服务配置无效。")
	}
	r.Header.Set("Authorization", "Bearer "+c.ChatKey)
	r.Header.Set("Content-Type", "application/json")
	client := http.Client{Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, e := client.Do(r)
	if e != nil {
		return nil, fail(503, "对话服务暂时不可用。")
	}
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") {
		res.Body.Close()
		return nil, fail(503, "对话服务不支持当前流式请求。")
	}
	b := &idleBody{ReadCloser: res.Body}
	b.timer = time.AfterFunc(30*time.Second, func() { res.Body.Close() })
	return b, nil
}

// Strip unsupported [n] references before forwarding, including references split
// across upstream token boundaries. Ordinary brackets in math prose are kept.
type citationCleaner struct{ pending string }

func (c *citationCleaner) Push(text string) string {
	c.pending += text
	var out strings.Builder
	for len(c.pending) > 0 {
		start := strings.IndexByte(c.pending, '[')
		if start < 0 {
			out.WriteString(c.pending)
			c.pending = ""
			break
		}
		out.WriteString(c.pending[:start])
		c.pending = c.pending[start:]
		end := strings.IndexByte(c.pending, ']')
		if end >= 0 && citationRE.MatchString(c.pending[:end+1]) && citationRE.FindString(c.pending[:end+1]) == c.pending[:end+1] {
			c.pending = c.pending[end+1:]
			continue
		}
		possible := true
		for i := 1; i < len(c.pending); i++ {
			if c.pending[i] < '0' || c.pending[i] > '9' {
				possible = false
				break
			}
		}
		if end < 0 && possible {
			break
		}
		out.WriteByte('[')
		c.pending = c.pending[1:]
	}
	return out.String()
}
func (c *citationCleaner) Finish() string { out := c.pending; c.pending = ""; return out }
