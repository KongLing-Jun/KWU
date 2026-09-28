package automation

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var markdownLinkPattern = regexp.MustCompile(`!?\[([^\]]+)\]\(([^)]+)\)`)

func (s *service) ExportRunResult(ctx context.Context, req RunResultExportRequest) (RunResultExport, error) {
	if err := ctx.Err(); err != nil {
		return RunResultExport{}, err
	}
	if err := s.requireStore(); err != nil {
		return RunResultExport{}, err
	}
	taskID := strings.TrimSpace(req.TaskID)
	runID := strings.TrimSpace(req.RunID)
	if taskID == "" || runID == "" {
		return RunResultExport{}, errors.New("automation: export task id and run id are required")
	}
	format, err := normalizeRunResultExportFormat(req.Format)
	if err != nil {
		return RunResultExport{}, err
	}
	manifest, err := s.store.ReadRunManifest(taskID, runID)
	if err != nil {
		return RunResultExport{}, err
	}
	result, err := s.store.ReadResult(taskID, runID)
	if errors.Is(err, os.ErrNotExist) {
		return RunResultExport{}, errors.New("automation: run result not found")
	}
	if err != nil {
		return RunResultExport{}, err
	}
	body, err := buildRunResultExportPayload(format, manifest, result)
	if err != nil {
		return RunResultExport{}, err
	}
	return RunResultExport{
		Filename:      runResultExportFilename(manifest, format.extension),
		MimeType:      format.mimeType,
		Payload:       base64.StdEncoding.EncodeToString(body),
		Base64Encoded: true,
	}, nil
}

type runResultExportFormat struct {
	format    string
	label     string
	extension string
	mimeType  string
}

var runResultExportFormats = []runResultExportFormat{
	{format: "md", label: "MD", extension: "md", mimeType: "text/markdown;charset=utf-8"},
	{format: "docx", label: "DOCX", extension: "docx", mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
	{format: "txt", label: "TXT", extension: "txt", mimeType: "text/plain;charset=utf-8"},
	{format: "json", label: "JSON", extension: "json", mimeType: "application/json;charset=utf-8"},
	{format: "xlsx", label: "XLSX", extension: "xlsx", mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	{format: "pptx", label: "PPT", extension: "pptx", mimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation"},
}

func normalizeRunResultExportFormat(raw string) (runResultExportFormat, error) {
	key := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(raw)), ".")
	switch key {
	case "markdown":
		key = "md"
	case "text":
		key = "txt"
	case "word":
		key = "docx"
	case "excel":
		key = "xlsx"
	case "ppt", "powerpoint":
		key = "pptx"
	}
	for _, format := range runResultExportFormats {
		if format.format == key {
			return format, nil
		}
	}
	return runResultExportFormat{}, fmt.Errorf("%w: unsupported result export format %q", ErrInvalidTask, raw)
}

func availableRunResultExportFormats(result string) []RunResultExportFormat {
	if strings.TrimSpace(result) == "" {
		return nil
	}
	out := make([]RunResultExportFormat, 0, len(runResultExportFormats))
	for _, format := range runResultExportFormats {
		out = append(out, RunResultExportFormat{
			Format:    format.format,
			Label:     format.label,
			Extension: format.extension,
			MimeType:  format.mimeType,
		})
	}
	return out
}

func buildRunResultExportPayload(format runResultExportFormat, manifest RunManifest, result string) ([]byte, error) {
	blocks := parseMarkdownExportBlocks(result)
	switch format.extension {
	case "md":
		return []byte(result), nil
	case "txt":
		return []byte(markdownBlocksPlainText(blocks)), nil
	case "json":
		return json.MarshalIndent(struct {
			Manifest RunManifest           `json:"manifest"`
			Blocks   []markdownExportBlock `json:"blocks"`
		}{Manifest: manifest, Blocks: blocks}, "", "  ")
	case "docx":
		return buildDocxResult(blocks)
	case "xlsx":
		return buildXlsxResult(blocks)
	case "pptx":
		return buildPptxResult(blocks)
	default:
		return nil, fmt.Errorf("%w: unsupported result export extension %q", ErrInvalidTask, format.extension)
	}
}

type markdownExportBlock struct {
	Kind  string `json:"kind"`
	Level int    `json:"level,omitempty"`
	Text  string `json:"text"`
}

func parseMarkdownExportBlocks(markdown string) []markdownExportBlock {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	blocks := make([]markdownExportBlock, 0, len(lines))
	var paragraph []string
	flushParagraph := func() {
		if len(paragraph) == 0 {
			return
		}
		text := stripInlineMarkdown(strings.Join(paragraph, " "))
		if text != "" {
			blocks = append(blocks, markdownExportBlock{Kind: "paragraph", Text: text})
		}
		paragraph = paragraph[:0]
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			flushParagraph()
			continue
		}
		if isMarkdownRule(line) {
			flushParagraph()
			continue
		}
		if level, text, ok := parseMarkdownHeading(line); ok {
			flushParagraph()
			blocks = append(blocks, markdownExportBlock{Kind: "heading", Level: level, Text: stripInlineMarkdown(text)})
			continue
		}
		if text, ok := parseMarkdownListItem(line); ok {
			flushParagraph()
			blocks = append(blocks, markdownExportBlock{Kind: "bullet", Text: stripInlineMarkdown(text)})
			continue
		}
		if strings.HasPrefix(line, ">") {
			flushParagraph()
			blocks = append(blocks, markdownExportBlock{Kind: "quote", Text: stripInlineMarkdown(strings.TrimSpace(strings.TrimPrefix(line, ">")))})
			continue
		}
		paragraph = append(paragraph, line)
	}
	flushParagraph()
	if len(blocks) == 0 && strings.TrimSpace(markdown) != "" {
		blocks = append(blocks, markdownExportBlock{Kind: "paragraph", Text: stripInlineMarkdown(markdown)})
	}
	return blocks
}

func parseMarkdownHeading(line string) (int, string, bool) {
	level := 0
	for level < len(line) && level < 6 && line[level] == '#' {
		level++
	}
	if level == 0 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	return level, strings.TrimSpace(line[level+1:]), true
}

func parseMarkdownListItem(line string) (string, bool) {
	for _, prefix := range []string{"- ", "* ", "+ ", "• "} {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), true
		}
	}
	for i, r := range line {
		if r < '0' || r > '9' {
			if r == '.' && i > 0 && i+1 < len(line) && line[i+1] == ' ' {
				return strings.TrimSpace(line[i+2:]), true
			}
			break
		}
	}
	return "", false
}

func isMarkdownRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	for _, r := range line {
		if r != '-' && r != '_' && r != '*' {
			return false
		}
	}
	return true
}

func stripInlineMarkdown(text string) string {
	text = markdownLinkPattern.ReplaceAllString(text, "$1 ($2)")
	replacer := strings.NewReplacer("**", "", "__", "", "`", "", "\\*", "*", "\\_", "_")
	return strings.TrimSpace(replacer.Replace(text))
}

func markdownBlocksPlainText(blocks []markdownExportBlock) string {
	lines := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind == "bullet" {
			lines = append(lines, "- "+block.Text)
		} else {
			lines = append(lines, block.Text)
		}
	}
	return strings.Join(lines, "\n")
}

func runResultExportFilename(manifest RunManifest, extension string) string {
	base := sanitizeExportFilenamePart(firstNonEmptyString(manifest.TaskName, manifest.TaskID, "automation-result"))
	runID := sanitizeExportFilenamePart(manifest.RunID)
	if runID != "" {
		base += "-" + runID
	}
	return base + "." + extension
}

func sanitizeExportFilenamePart(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = false
	}
	return strings.Trim(b.String(), " .-")
}

type exportZipEntry struct {
	name string
	body string
}

func buildZip(entries []exportZipEntry) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		w, err := zw.Create(entry.name)
		if err != nil {
			_ = zw.Close()
			return nil, err
		}
		if _, err := w.Write([]byte(entry.body)); err != nil {
			_ = zw.Close()
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xmlText(value string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(value))
	return buf.String()
}

func buildDocxResult(blocks []markdownExportBlock) ([]byte, error) {
	return buildZip([]exportZipEntry{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + wordParagraphs(blocks) + `<w:sectPr/></w:body></w:document>`},
	})
}

func wordParagraphs(blocks []markdownExportBlock) string {
	if len(blocks) == 0 {
		blocks = []markdownExportBlock{{Kind: "paragraph"}}
	}
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(`<w:p>`)
		switch block.Kind {
		case "heading":
			b.WriteString(`<w:pPr><w:pStyle w:val="Heading` + strconv.Itoa(clampHeadingLevel(block.Level)) + `"/></w:pPr>`)
		case "bullet":
			b.WriteString(`<w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>`)
		}
		b.WriteString(`<w:r>`)
		if block.Kind == "quote" {
			b.WriteString(`<w:rPr><w:i/></w:rPr>`)
		}
		b.WriteString(`<w:t xml:space="preserve">`)
		if block.Kind == "bullet" {
			b.WriteString("• ")
		}
		b.WriteString(xmlText(block.Text))
		b.WriteString(`</w:t></w:r></w:p>`)
	}
	return b.String()
}

func clampHeadingLevel(level int) int {
	if level < 1 {
		return 1
	}
	if level > 3 {
		return 3
	}
	return level
}

func buildXlsxResult(blocks []markdownExportBlock) ([]byte, error) {
	return buildZip([]exportZipEntry{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Result" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`},
		{"xl/worksheets/sheet1.xml", xlsxResultSheet(blocks)},
	})
}

func xlsxResultSheet(blocks []markdownExportBlock) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	b.WriteString(xlsxExportRow(1, []string{"type", "level", "text"}))
	for i, block := range blocks {
		level := ""
		if block.Level > 0 {
			level = strconv.Itoa(block.Level)
		}
		b.WriteString(xlsxExportRow(i+2, []string{block.Kind, level, block.Text}))
	}
	b.WriteString(`</sheetData></worksheet>`)
	return b.String()
}

func xlsxExportRow(row int, cells []string) string {
	var b strings.Builder
	b.WriteString(`<row r="` + strconv.Itoa(row) + `">`)
	for i, value := range cells {
		ref := string(rune('A'+i)) + strconv.Itoa(row)
		b.WriteString(`<c r="` + ref + `" t="inlineStr"><is><t xml:space="preserve">` + xmlText(value) + `</t></is></c>`)
	}
	b.WriteString(`</row>`)
	return b.String()
}

func buildPptxResult(blocks []markdownExportBlock) ([]byte, error) {
	slides := markdownSlides(blocks)
	entries := []exportZipEntry{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
` + pptContentTypeOverrides(len(slides)) + `</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`},
		{"ppt/presentation.xml", `<?xml version="1.0" encoding="UTF-8"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><p:sldIdLst>` + pptSlideIDs(len(slides)) + `</p:sldIdLst><p:sldSz cx="9144000" cy="5143500"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>`},
		{"ppt/_rels/presentation.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
` + pptSlideRelationships(len(slides)) + `
</Relationships>`},
	}
	for i, slide := range slides {
		entries = append(entries, exportZipEntry{name: "ppt/slides/slide" + strconv.Itoa(i+1) + ".xml", body: pptSlideXML(slide)})
	}
	return buildZip(entries)
}

type pptSlide struct {
	title string
	body  []markdownExportBlock
}

func markdownSlides(blocks []markdownExportBlock) []pptSlide {
	if len(blocks) == 0 {
		return []pptSlide{{title: "Automation Result"}}
	}
	slides := []pptSlide{}
	current := pptSlide{title: "Automation Result"}
	for _, block := range blocks {
		if block.Kind == "heading" {
			if current.title != "Automation Result" || len(current.body) > 0 {
				slides = append(slides, current)
			}
			current = pptSlide{title: block.Text}
			continue
		}
		current.body = append(current.body, block)
	}
	slides = append(slides, current)
	return slides
}

func pptContentTypeOverrides(count int) string {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString(`<Override PartName="/ppt/slides/slide` + strconv.Itoa(i) + `.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
`)
	}
	return b.String()
}

func pptSlideIDs(count int) string {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString(`<p:sldId id="` + strconv.Itoa(255+i) + `" r:id="rId` + strconv.Itoa(i) + `"/>`)
	}
	return b.String()
}

func pptSlideRelationships(count int) string {
	var b strings.Builder
	for i := 1; i <= count; i++ {
		b.WriteString(`<Relationship Id="rId` + strconv.Itoa(i) + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide` + strconv.Itoa(i) + `.xml"/>
`)
	}
	return b.String()
}

func pptSlideXML(slide pptSlide) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="2" name="Title"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="457200" y="274320"/><a:ext cx="8229600" cy="685800"/></a:xfrm></p:spPr><p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:r><a:rPr sz="3000" b="1"/><a:t>` + xmlText(slide.title) + `</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="3" name="Body"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="685800" y="1143000"/><a:ext cx="7772400" cy="3657600"/></a:xfrm></p:spPr><p:txBody><a:bodyPr wrap="square"/><a:lstStyle/>` + pptParagraphs(slide.body) + `</p:txBody></p:sp></p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>`
}

func pptParagraphs(blocks []markdownExportBlock) string {
	if len(blocks) == 0 {
		blocks = []markdownExportBlock{{Kind: "paragraph", Text: ""}}
	}
	var b strings.Builder
	for _, block := range blocks {
		b.WriteString(`<a:p>`)
		if block.Kind == "bullet" {
			b.WriteString(`<a:pPr marL="342900" indent="-171450"><a:buChar char="•"/></a:pPr>`)
		}
		b.WriteString(`<a:r><a:t>`)
		b.WriteString(xmlText(block.Text))
		b.WriteString(`</a:t></a:r></a:p>`)
	}
	return b.String()
}
