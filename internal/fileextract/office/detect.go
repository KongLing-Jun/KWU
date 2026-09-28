package office

import (
	"path/filepath"
	"strings"
)

type Format string

const (
	FormatDOCX Format = "docx"
	FormatXLSX Format = "xlsx"
	FormatPPTX Format = "pptx"
)

func SupportedFormat(p string) (Format, bool) {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".docx":
		return FormatDOCX, true
	case ".xlsx":
		return FormatXLSX, true
	case ".pptx":
		return FormatPPTX, true
	default:
		return "", false
	}
}

func LegacyFormat(p string) bool {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".doc", ".xls", ".ppt":
		return true
	default:
		return false
	}
}

func ShouldHandle(p string) bool {
	_, ok := SupportedFormat(p)
	return ok || LegacyFormat(p)
}
