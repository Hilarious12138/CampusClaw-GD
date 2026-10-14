package main

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

type ChunkOptions struct {
	Strategy           string  `json:"strategy"`
	MaxLength          int     `json:"max_length"`
	OverlapRatio       float64 `json:"overlap_ratio"`
	Delimiter          string  `json:"delimiter"`
	RemoveURLs         bool    `json:"remove_urls"`
	RemoveEmails       bool    `json:"remove_emails"`
	CollapseWhitespace bool    `json:"collapse_whitespace"`
}
type TextChunk struct {
	Start, End int
	Text       string
}

func (o ChunkOptions) normalized() (ChunkOptions, error) {
	if o.Strategy == "" {
		o.Strategy = "auto"
	}
	switch o.Strategy {
	case "auto", "hierarchy":
		return ChunkOptions{Strategy: o.Strategy, MaxLength: 800, OverlapRatio: 0.1}, nil
	case "custom":
		if o.MaxLength < 100 || o.MaxLength > 2000 || math.IsNaN(o.OverlapRatio) || o.OverlapRatio < 0 || o.OverlapRatio > 0.5 {
			return o, fail(400, "切片长度须为 100～2000，重叠比例须为 0～0.5。")
		}
		if o.Delimiter == "" {
			o.Delimiter = "newline"
		}
		if o.Delimiter != "newline" && o.Delimiter != "blankline" && o.Delimiter != "period" {
			return o, fail(400, "切片分隔符无效。")
		}
	default:
		return o, fail(400, "切片策略无效。")
	}
	return o, nil
}

var urlRE = regexp.MustCompile(`https?://[^\s<>]+`)
var emailRE = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
var headingRE = regexp.MustCompile(`(?m)^#{1,3}[ \t]+`)

func splitText(body string, o ChunkOptions) (string, string, []TextChunk) {
	text, basis := body, "original"
	if o.Strategy == "custom" {
		if o.RemoveURLs {
			text = urlRE.ReplaceAllString(text, "")
			basis = "processed"
		}
		if o.RemoveEmails {
			text = emailRE.ReplaceAllString(text, "")
			basis = "processed"
		}
		if o.CollapseWhitespace {
			text = strings.Join(strings.Fields(text), " ")
			basis = "processed"
		}
	}
	rs := []rune(text)
	chunks := []TextChunk{}
	window := func(lo, hi int) {
		for start := lo; start < hi; {
			end := start + o.MaxLength
			if end > hi {
				end = hi
			}
			if end < hi {
				// Prefer a boundary in the latter half, guaranteeing progress with overlap.
				for j := end; j > start+o.MaxLength/2; j-- {
					boundary := false
					switch o.Delimiter {
					case "newline":
						boundary = rs[j-1] == '\n'
					case "blankline":
						boundary = j >= 2 && rs[j-1] == '\n' && rs[j-2] == '\n'
					case "period":
						boundary = rs[j-1] == '。' || rs[j-1] == '.'
					default:
						boundary = rs[j-1] == '\n' || rs[j-1] == '。'
					}
					if boundary {
						end = j
						break
					}
				}
			}
			part := string(rs[start:end])
			if strings.TrimFunc(part, unicode.IsSpace) != "" {
				chunks = append(chunks, TextChunk{start, end, part})
			}
			if end == hi {
				break
			}
			next := end - int(float64(o.MaxLength)*o.OverlapRatio)
			if next <= start {
				next = start + 1
			}
			start = next
		}
	}
	if o.Strategy == "hierarchy" {
		start := 0
		for _, loc := range headingRE.FindAllStringIndex(text, -1) {
			pos := len([]rune(text[:loc[0]]))
			if pos > start {
				window(start, pos)
			}
			start = pos
		}
		window(start, len(rs))
	} else {
		window(0, len(rs))
	}
	return text, basis, chunks
}
