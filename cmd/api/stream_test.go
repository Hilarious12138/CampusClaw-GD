package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

type byteReader struct{ io.Reader }

func (b byteReader) Read(p []byte) (int, error) { return b.Reader.Read(p[:1]) }
func TestModelSSE(t *testing.T) {
	s := ": heart\r\nevent: message\r\ndata: {\"choices\":[{\"delta\":{\"content\":\"中文😀\"}}]}\r\n\r\ndata: {\"choices\":\n" + "data: [{\"delta\":{\"content\":\"[1]\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
	var out strings.Builder
	n := 0
	e := parseModelStream(byteReader{strings.NewReader(s)}, func(text string) error { n++; out.WriteString(text); return nil })
	if e != nil || n != 2 || out.String() != "中文😀[1]" {
		t.Fatal(out.String(), e)
	}
	for _, bad := range []string{"data: broken\n\n", "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n", "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}]}\n\n"} {
		if parseModelStream(strings.NewReader(bad), func(string) error { return nil }) == nil {
			t.Fatal("bad stream accepted")
		}
	}
}
func TestSkills(t *testing.T) {
	s, e := parseSkill(defaultSkillSource)
	if e != nil || s.Name != "解题引导" || !s.Enabled {
		t.Fatal(s, e)
	}
	commented := "# 课堂技能文件\n\n" + defaultSkillSource
	if parsed, e := parseSkill(commented); e != nil || parsed.Name != s.Name {
		t.Fatal("leading comments", e)
	}
	for _, bad := range []string{"", strings.Replace(defaultSkillSource, "knowledge_search", "execute_shell", 1), strings.Replace(defaultSkillSource, "启用：是", "启用：maybe", 1), strings.Replace(defaultSkillSource, "\n\n", "\n", 1), strings.Replace(defaultSkillSource, "名称：解题引导", "名称："+strings.Repeat("长", 33), 1)} {
		if _, e = parseSkill(bad); e == nil {
			t.Fatal("bad skill accepted")
		}
	}
}
func TestOpenAPIContract(t *testing.T) {
	var spec struct {
		Paths map[string]map[string]struct {
			OperationID string `json:"operationId"`
			Security    []any  `json:"security"`
		} `json:"paths"`
	}
	if e := json.Unmarshal(openAPI, &spec); e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	n := 0
	for path, item := range spec.Paths {
		for _, op := range item {
			if ids[op.OperationID] || op.OperationID == "" {
				t.Fatal("operation id")
			}
			ids[op.OperationID] = true
			n++
			public := path == "/health" || path == "/api/auth/login" || path == "/openapi.json" || path == "/swagger/" || path == "/swagger/index.html"
			if public != (len(op.Security) == 0) {
				t.Fatal("security", path)
			}
		}
	}
	if n != 30 {
		t.Fatal(n)
	}
}
