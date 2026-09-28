package openai

import (
	"strings"

	"kwu/internal/frontmatter"
)

const (
	thinkOpen           = "<think>"
	thinkClose          = "</think>"
	frontmatterProbeMax = 64 * 1024
)

type reasoningPrefixSplitter struct {
	think thinkSplitter
	front frontmatterSplitter
}

func (s *reasoningPrefixSplitter) push(chunk string) (reasoning, text string) {
	r, txt := s.think.push(chunk)
	reasoning += r
	if txt != "" {
		r, txt = s.front.push(txt)
		reasoning += r
		text += txt
	}
	return reasoning, text
}

func (s *reasoningPrefixSplitter) flush() (reasoning, text string) {
	r, txt := s.think.flush()
	reasoning += r
	if txt != "" {
		r, txt = s.front.push(txt)
		reasoning += r
		text += txt
	}
	r, txt = s.front.flush()
	return reasoning + r, text + txt
}

type thinkState int

const (
	thinkProbe thinkState = iota
	thinkInside
	thinkPassthrough
)

type frontmatterState int

const (
	frontmatterProbe frontmatterState = iota
	frontmatterPassthrough
)

type frontmatterProbeResult int

const (
	frontmatterNeedMore frontmatterProbeResult = iota
	frontmatterReady
	frontmatterNotPresent
)

type frontmatterSplitter struct {
	state frontmatterState
	buf   string
}

func (f *frontmatterSplitter) push(s string) (reasoning, text string) {
	if f.state == frontmatterPassthrough {
		return "", s
	}

	f.buf += s
	trimmed := strings.TrimLeft(f.buf, " \t\r\n")
	switch classifyFrontmatterProbe(trimmed, len(f.buf)) {
	case frontmatterNeedMore:
		return "", ""
	case frontmatterNotPresent:
		return "", f.drainPassthrough()
	}

	fm, body := frontmatter.Split(trimmed)
	reasoning = frontmatterReasoning(fm)
	if reasoning == "" {
		return "", f.drainPassthrough()
	}

	f.state = frontmatterPassthrough
	f.buf = ""
	return reasoning, strings.TrimLeft(body, " \t\r\n")
}

func classifyFrontmatterProbe(trimmed string, buffered int) frontmatterProbeResult {
	if trimmed == "" {
		return frontmatterNeedMore
	}
	if len(trimmed) < len("---") {
		if strings.HasPrefix("---", trimmed) {
			return frontmatterNeedMore
		}
		return frontmatterNotPresent
	}
	if !strings.HasPrefix(trimmed, "---") {
		return frontmatterNotPresent
	}
	if len(trimmed) == len("---") {
		return frontmatterNeedMore
	}
	if trimmed[3] != '\n' && trimmed[3] != '\r' {
		return frontmatterNotPresent
	}
	if hasClosingFrontmatterFence(trimmed) {
		return frontmatterReady
	}
	if buffered > frontmatterProbeMax {
		return frontmatterNotPresent
	}
	return frontmatterNeedMore
}

func (f *frontmatterSplitter) flush() (reasoning, text string) {
	if f.buf == "" {
		return "", ""
	}
	out := f.buf
	f.buf = ""
	return "", out
}

func (f *frontmatterSplitter) drainPassthrough() string {
	f.state = frontmatterPassthrough
	out := f.buf
	f.buf = ""
	return out
}

// thinkSplitter peels a leading <think>...</think> block out of the content
// stream into reasoning text. MiniMax-M3 inlines its chain-of-thought this way
// instead of populating reasoning_content. It only arms on a <think> at the very
// start of the turn, so an answer that merely mentions the tag is never hijacked.
type thinkSplitter struct {
	state thinkState
	buf   string
}

func (t *thinkSplitter) push(s string) (reasoning, text string) {
	switch t.state {
	case thinkPassthrough:
		return "", s
	case thinkInside:
		return t.scanClose(s)
	}

	t.buf += s
	trimmed := strings.TrimLeft(t.buf, " \t\r\n")
	if len(trimmed) < len(thinkOpen) {
		if strings.HasPrefix(thinkOpen, trimmed) {
			return "", "" // still could become <think> once more arrives
		}
		return "", t.drainPassthrough()
	}
	if strings.HasPrefix(trimmed, thinkOpen) {
		t.state = thinkInside
		t.buf = ""
		return t.scanClose(trimmed[len(thinkOpen):])
	}
	return "", t.drainPassthrough()
}

func (t *thinkSplitter) scanClose(s string) (reasoning, text string) {
	t.buf += s
	if idx := strings.Index(t.buf, thinkClose); idx >= 0 {
		r := t.buf[:idx]
		rest := strings.TrimLeft(t.buf[idx+len(thinkClose):], " \t\r\n")
		t.buf = ""
		t.state = thinkPassthrough
		return r, rest
	}
	keep := markerSuffixLen(t.buf, thinkClose)
	r := t.buf[:len(t.buf)-keep]
	t.buf = t.buf[len(t.buf)-keep:]
	return r, ""
}

// flush emits whatever is buffered when the stream ends mid-decision: an
// unterminated <think> block is reasoning; anything else is text.
func (t *thinkSplitter) flush() (reasoning, text string) {
	if t.buf == "" {
		return "", ""
	}
	out := t.buf
	t.buf = ""
	if t.state == thinkInside {
		return out, ""
	}
	return "", out
}

func (t *thinkSplitter) drainPassthrough() string {
	t.state = thinkPassthrough
	out := t.buf
	t.buf = ""
	return out
}

// markerSuffixLen returns the length of the longest proper suffix of s that is a
// prefix of marker — the tail to hold back in case the rest of the tag arrives
// in the next delta.
func markerSuffixLen(s, marker string) int {
	max := len(marker) - 1
	if max > len(s) {
		max = len(s)
	}
	for n := max; n > 0; n-- {
		if strings.HasPrefix(marker, s[len(s)-n:]) {
			return n
		}
	}
	return 0
}

func hasClosingFrontmatterFence(s string) bool {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return true
		}
	}
	return false
}

func frontmatterReasoning(fm map[string]string) string {
	for _, key := range []string{
		"reasoning_content",
		"reasoning",
		"thinking",
		"thoughts",
		"thought",
		"analysis",
	} {
		if v := strings.TrimSpace(fm[key]); v != "" {
			return v
		}
	}
	return ""
}
