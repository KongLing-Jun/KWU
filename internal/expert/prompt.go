package expert

import "strings"

const promptHeader = "Expert role"

func ApplyPrompt(base string, ex Expert) string {
	prompt := strings.TrimSpace(ex.Prompt)
	if prompt == "" {
		return base
	}
	block := promptHeader + ": " + ex.Name + "\n" + prompt
	base = strings.TrimRight(base, "\n")
	if base == "" {
		return block
	}
	return base + "\n\n" + block
}
