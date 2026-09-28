package office

import (
	"encoding/xml"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func attr(se xml.StartElement, local string) string {
	for _, a := range se.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func relID(se xml.StartElement) string {
	for _, a := range se.Attr {
		if a.Name.Local == "id" && strings.Contains(a.Name.Space, "relationships") {
			return a.Value
		}
	}
	return attr(se, "id")
}

func normalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func appendTextPart(dst *strings.Builder, s string) {
	if s == "" {
		return
	}
	dst.WriteString(s)
}

func sortStrings(v []string) {
	sort.Strings(v)
}

func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if !unicode.IsLetter(r) {
			break
		}
		u := unicode.ToUpper(r)
		if u < 'A' || u > 'Z' {
			break
		}
		n = n*26 + int(u-'A'+1)
	}
	return n
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}

func collectTextByLocalName(data []byte, textLocal string) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	state := &paragraphTextState{textLocal: textLocal}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			state.start(t.Name.Local)
		case xml.CharData:
			state.charData(string(t))
		case xml.EndElement:
			state.end(t.Name.Local)
		}
	}
	state.flush()
	return state.lines, nil
}

type paragraphTextState struct {
	textLocal string
	lines     []string
	current   strings.Builder
	inText    bool
}

func (s *paragraphTextState) start(local string) {
	if local == s.textLocal {
		s.inText = true
	}
}

func (s *paragraphTextState) charData(text string) {
	if s.inText {
		s.current.WriteString(text)
	}
}

func (s *paragraphTextState) end(local string) {
	if local == s.textLocal {
		s.inText = false
	}
	if local == "p" {
		s.flush()
	}
}

func (s *paragraphTextState) flush() {
	if text := normalizeText(s.current.String()); text != "" {
		s.lines = append(s.lines, text)
	}
	s.current.Reset()
}
