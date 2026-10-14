package main

import (
	"strings"
	"testing"
)

func TestSkillRoutingAndPrompt(t *testing.T) {
	a := Skill{ID: strings.Repeat("a", 32), Name: "数学解题引导", Trigger: "方程,等式", Enabled: true}
	b := Skill{ID: strings.Repeat("b", 32), Name: "语文阅读", Trigger: "方程、阅读", Enabled: true}
	if s := routeSkill("方程中的等式怎么处理？", []Skill{b, a}); s == nil || s.ID != a.ID {
		t.Fatal("highest score")
	}
	if s := routeSkill("方程怎么处理？", []Skill{b, a}); s == nil || s.ID != a.ID {
		t.Fatal("stable tie")
	}
	if s := routeSkill("天气怎样？", []Skill{b, a}); s != nil {
		t.Fatal("unmatched route")
	}
	b.Enabled = false
	if s := routeSkill("阅读怎样？", []Skill{b}); s != nil {
		t.Fatal("disabled route")
	}
	agent := Agent{Prompt: "你是数学助手。"}
	if !strings.Contains(systemForAgent(agent, &a, true), "不写最终答案") {
		t.Fatal("guidance absent")
	}
	if strings.Contains(systemForAgent(agent, nil, false), "不写最终答案") {
		t.Fatal("unselected guidance leaked")
	}
}

func TestCitationCleanerAcrossTokens(t *testing.T) {
	var c citationCleaner
	var out strings.Builder
	for _, piece := range []string{"先看[", "1", "]等式，再看[", "999", "]。保留[x]和[4"} {
		out.WriteString(c.Push(piece))
	}
	out.WriteString(c.Finish())
	if got := out.String(); got != "先看等式，再看。保留[x]和[4" {
		t.Fatal(got)
	}
	if !validStreamCitations("问题本身有依据", 1) || !validStreamCitations("出处[1]", 1) || validStreamCitations("出处[2]", 1) {
		t.Fatal("stream citations")
	}
}
