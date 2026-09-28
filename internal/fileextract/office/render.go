package office

import (
	"fmt"
	"strings"
)

func renderDOCX(source string, lines []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[office extract: docx]\n[source: %s]\n\n# Document\n", source)
	writeLines(&b, lines)
	return b.String()
}

func renderXLSX(source string, sheets []sheetText) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[office extract: xlsx]\n[source: %s]\n", source)
	for _, sheet := range sheets {
		fmt.Fprintf(&b, "\n# Sheet: %s\n", sheet.name)
		writeLines(&b, sheet.lines)
	}
	return b.String()
}

func renderPPTX(source string, slides [][]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[office extract: pptx]\n[source: %s]\n", source)
	for i, lines := range slides {
		fmt.Fprintf(&b, "\n# Slide %d\n", i+1)
		writeLines(&b, lines)
	}
	return b.String()
}

func writeLines(b *strings.Builder, lines []string) {
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

func limitOutput(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	if maxBytes <= len(outputTruncationNotice)+2 {
		return s[:maxBytes]
	}
	head := strings.TrimSpace(s[:maxBytes-len(outputTruncationNotice)-2])
	return head + "\n" + outputTruncationNotice + "\n"
}

func hasVisibleBody(s string) bool {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "[office extract:") || strings.HasPrefix(line, "[source:") || line == "# Document" || strings.HasPrefix(line, "# Slide ") {
			continue
		}
		return true
	}
	return false
}
