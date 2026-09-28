package office

import (
	"context"
	"encoding/xml"
	"io"
	"strings"
)

func extractDOCX(ctx context.Context, a *archive, source string) (string, error) {
	lines, err := readWordEntries(ctx, a, source, a.entriesWithPrefixSuffix("word/header", ".xml"))
	if err != nil {
		return "", err
	}
	docLines, err := readWordEntry(ctx, a, source, "word/document.xml")
	if err != nil {
		return "", err
	}
	lines = append(lines, docLines...)
	footerLines, err := readWordEntries(ctx, a, source, a.entriesWithPrefixSuffix("word/footer", ".xml"))
	if err != nil {
		return "", err
	}
	lines = append(lines, footerLines...)
	return renderDOCX(source, lines), nil
}

func readWordEntries(ctx context.Context, a *archive, source string, entries []string) ([]string, error) {
	var lines []string
	for _, entry := range entries {
		part, err := readWordEntry(ctx, a, source, entry)
		if err != nil {
			return nil, err
		}
		lines = append(lines, part...)
	}
	return lines, nil
}

func readWordEntry(ctx context.Context, a *archive, source, entry string) ([]string, error) {
	data, err := a.readXML(ctx, FormatDOCX, source, entry)
	if err != nil {
		return nil, err
	}
	part, err := parseWordText(ctx, data)
	if err != nil {
		return nil, xmlError(FormatDOCX, source, entry, err)
	}
	return part, nil
}

func parseWordText(ctx context.Context, data []byte) ([]string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	state := &wordTextState{}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
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
	return state.lines, nil
}

type wordTextState struct {
	lines       []string
	paragraph   strings.Builder
	cellParts   []string
	row         []string
	inText      bool
	inCell      bool
	inParagraph bool
}

func (s *wordTextState) start(local string) {
	switch local {
	case "tr":
		s.row = nil
	case "tc":
		s.inCell = true
		s.cellParts = nil
	case "p":
		s.inParagraph = true
		s.paragraph.Reset()
	case "t":
		s.inText = true
	case "tab":
		s.writeParagraphByte('\t')
	case "br", "cr":
		s.writeParagraphByte('\n')
	}
}

func (s *wordTextState) charData(text string) {
	if s.inText && s.inParagraph {
		appendTextPart(&s.paragraph, text)
	}
}

func (s *wordTextState) end(local string) {
	switch local {
	case "t":
		s.inText = false
	case "p":
		s.endParagraph()
	case "tc":
		s.endCell()
	case "tr":
		s.endRow()
	}
}

func (s *wordTextState) writeParagraphByte(ch byte) {
	if s.inParagraph {
		s.paragraph.WriteByte(ch)
	}
}

func (s *wordTextState) endParagraph() {
	text := normalizeText(s.paragraph.String())
	if text != "" {
		s.appendParagraph(text)
	}
	s.inParagraph = false
	s.paragraph.Reset()
}

func (s *wordTextState) appendParagraph(text string) {
	if s.inCell {
		s.cellParts = append(s.cellParts, text)
		return
	}
	s.lines = append(s.lines, text)
}

func (s *wordTextState) endCell() {
	s.inCell = false
	s.row = append(s.row, strings.Join(s.cellParts, " "))
	s.cellParts = nil
}

func (s *wordTextState) endRow() {
	if rowHasText(s.row) {
		s.lines = append(s.lines, strings.Join(s.row, "\t"))
	}
	s.row = nil
}

func rowHasText(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}
