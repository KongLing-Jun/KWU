// Package frontmatter provides a minimal, dependency-free parser for the
// ---fenced "key: value" blocks that prefix skill, command, and memory files.
// It mirrors the YAML-like frontmatter convention without pulling in a YAML
// library, keeping KWU's single-(TOML)-dependency promise.
package frontmatter

import "strings"

// Split separates an optional leading ---fenced block of "key: value" lines from
// the body. It returns the parsed keys (lowercased) and the remaining body. With
// no opening/closing fence the whole input is the body. An opened but never
// closed fence treats the entire input as body (no partial parse).
//
// Values are trimmed of surrounding whitespace and outer quotes (" or ').
// A key with an empty value heads either a section ("metadata:") whose indented
// "key: value" lines flatten (metadata.type → fm["type"]), or a YAML list whose
// "- item" lines are joined comma-separated (allowed-tools → "read_file, grep"),
// so list-valued keys from skills authored for other agent tools survive.
// The last write wins for duplicate keys.
func Split(s string) (map[string]string, string) {
	fm := map[string]string{}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return fm, s
	}
	if closing := closingFenceLine(lines); closing >= 0 {
		return parseFrontmatterLines(lines[1:closing]), strings.Join(lines[closing+1:], "\n")
	}
	return fm, s // opened but never closed: treat all as body
}

func closingFenceLine(lines []string) int {
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return i
		}
	}
	return -1
}

func parseFrontmatterLines(content []string) map[string]string {
	fm := map[string]string{}
	for j := 0; j < len(content); j++ {
		key, rawVal, val, ok := parseFrontmatterLine(content[j])
		if !ok {
			continue
		}
		if rawVal == "|" || rawVal == ">" {
			block, next := collectBlockScalar(content, j+1, rawVal == ">")
			fm[key] = block
			j = next - 1
			continue
		}
		if val == "" {
			items, next := collectListItems(content, j+1)
			j = next - 1
			if len(items) > 0 {
				fm[key] = strings.Join(items, ", ")
			}
			continue
		}
		fm[key] = val
	}
	return fm
}

func parseFrontmatterLine(line string) (key, rawVal, val string, ok bool) {
	k, v, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", "", false
	}
	rawVal = strings.TrimSpace(v)
	return strings.ToLower(strings.TrimSpace(k)), rawVal, strings.Trim(rawVal, `"'`), true
}

func collectListItems(content []string, start int) ([]string, int) {
	var items []string
	for start < len(content) {
		item, ok := strings.CutPrefix(strings.TrimSpace(content[start]), "-")
		if !ok {
			break
		}
		items = append(items, strings.Trim(strings.TrimSpace(item), `"'`))
		start++
	}
	return items, start
}

func collectBlockScalar(lines []string, start int, folded bool) (string, int) {
	block, minIndent, next := collectIndentedBlock(lines, start)
	block = trimBlockIndent(block, minIndent)
	if folded {
		return strings.TrimSpace(strings.Join(block, " ")), next
	}
	return strings.TrimRight(strings.Join(block, "\n"), "\n"), next
}

func collectIndentedBlock(lines []string, start int) ([]string, int, int) {
	var block []string
	minIndent := -1
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			block = append(block, "")
			continue
		}
		indent := leadingWhitespace(line)
		if indent == 0 {
			break
		}
		if minIndent < 0 || indent < minIndent {
			minIndent = indent
		}
		block = append(block, line)
	}
	if minIndent < 0 {
		minIndent = 0
	}
	return block, minIndent, i
}

func trimBlockIndent(block []string, minIndent int) []string {
	for j, line := range block {
		if strings.TrimSpace(line) == "" {
			block[j] = ""
			continue
		}
		if len(line) >= minIndent {
			block[j] = line[minIndent:]
		}
	}
	return block
}

func leadingWhitespace(s string) int {
	n := 0
	for n < len(s) && (s[n] == ' ' || s[n] == '\t') {
		n++
	}
	return n
}
