package pythonruntime

import "strings"

type shellHeredoc struct {
	delimiter string
	stripTabs bool
	expand    bool
}

func isHeredocStart(text string, i int) bool {
	return text[i] == '<' && i+1 < len(text) && text[i+1] == '<' && (i+2 >= len(text) || text[i+2] != '<')
}

func (s *shellCommandScanner) consumeHeredoc(i int) int {
	next := i + 2
	stripTabs := next < len(s.text) && s.text[next] == '-'
	if stripTabs {
		next++
	}
	next = skipHeredocHorizontalSpace(s.text, next)
	delimiter, expand, end := readHeredocDelimiter(s.text, next)
	if delimiter != "" {
		s.heredocs = append(s.heredocs, shellHeredoc{delimiter: delimiter, stripTabs: stripTabs, expand: expand})
	}
	return end
}

func skipHeredocHorizontalSpace(text string, start int) int {
	for start < len(text) && isShellSpace(text[start]) && text[start] != '\n' {
		start++
	}
	return start
}

func readHeredocDelimiter(text string, start int) (string, bool, int) {
	var delimiter strings.Builder
	quoted := false
	for start < len(text) && !isHeredocWordEnd(text[start]) {
		switch text[start] {
		case '\\':
			quoted = true
			start++
			if start < len(text) {
				delimiter.WriteByte(text[start])
				start++
			}
		case '\'', '"':
			quoted = true
			quote := text[start]
			start = appendQuotedHeredocDelimiter(&delimiter, text, start+1, quote)
		default:
			delimiter.WriteByte(text[start])
			start++
		}
	}
	return delimiter.String(), !quoted, start
}

func appendQuotedHeredocDelimiter(delimiter *strings.Builder, text string, start int, quote byte) int {
	for start < len(text) && text[start] != quote {
		if text[start] == '\\' && quote == '"' && start+1 < len(text) {
			start++
		}
		delimiter.WriteByte(text[start])
		start++
	}
	if start < len(text) {
		start++
	}
	return start
}

func isHeredocWordEnd(value byte) bool {
	return isShellSpace(value) || strings.ContainsRune(";|&(){}<>", rune(value))
}

func (s *shellCommandScanner) skipHeredocBodies(start int) int {
	for _, heredoc := range s.heredocs {
		body, next := heredocBody(s.text, start, heredoc)
		if heredoc.expand {
			s.captureHeredocExpansions(body)
		}
		start = next
	}
	s.heredocs = nil
	return start
}

func heredocBody(text string, start int, heredoc shellHeredoc) (string, int) {
	bodyStart := start
	for start < len(text) {
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += start
		}
		line := strings.TrimSuffix(text[start:end], "\r")
		if heredoc.stripTabs {
			line = strings.TrimLeft(line, "\t")
		}
		next := min(end+1, len(text))
		if line == heredoc.delimiter {
			return text[bodyStart:start], next
		}
		start = next
	}
	return text[bodyStart:], len(text)
}

func (s *shellCommandScanner) captureHeredocExpansions(body string) {
	for i := 0; i < len(body); {
		if body[i] == '\\' {
			i = min(i+2, len(body))
			continue
		}
		if strings.HasPrefix(body[i:], "$(") && !strings.HasPrefix(body[i:], "$((") {
			command, next := readDollarCommandSubstitution(body, i+2)
			s.captureHeredocCommand(command)
			i = next
			continue
		}
		if body[i] == '`' {
			command, next := readBacktickCommandSubstitution(body, i+1)
			s.captureHeredocCommand(command)
			i = next
			continue
		}
		i++
	}
}

func readDollarCommandSubstitution(text string, start int) (string, int) {
	depth := 1
	quote := byte(0)
	for i := start; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if quote != 0 {
			if text[i] == quote {
				quote = 0
			}
			continue
		}
		if text[i] == '\'' || text[i] == '"' {
			quote = text[i]
			continue
		}
		switch text[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return text[start:i], i + 1
			}
		}
	}
	return text[start:], len(text)
}

func readBacktickCommandSubstitution(text string, start int) (string, int) {
	for i := start; i < len(text); i++ {
		if text[i] == '\\' {
			i++
			continue
		}
		if text[i] == '`' {
			return text[start:i], i + 1
		}
	}
	return text[start:], len(text)
}

func (s *shellCommandScanner) captureHeredocCommand(command string) {
	nested := shellCommandScanner{text: command, want: s.want, expectCommand: true, strictDynamic: s.strictDynamic}
	for _, binding := range nested.commandMatches(false) {
		reason := "unquoted heredoc command substitution has shell-specific resolution semantics"
		if binding.Unprovable != "" {
			reason += "; " + binding.Unprovable
		}
		s.matches = append(s.matches, ShellCommandBinding{Command: binding.Command, Wrapper: "heredoc", Unprovable: reason})
	}
}
