package office

import "fmt"

type ErrorKind string

const (
	ErrUnsupported ErrorKind = "unsupported"
	ErrCorrupt     ErrorKind = "corrupt"
	ErrMissingXML  ErrorKind = "missing_xml"
	ErrXML         ErrorKind = "xml"
	ErrTooLarge    ErrorKind = "too_large"
	ErrNoText      ErrorKind = "no_visible_text"
)

type Error struct {
	Kind   ErrorKind
	Format Format
	Path   string
	Entry  string
	Detail string
	Cause  error
}

func (e *Error) Error() string {
	if format := errorFormatter(e.Kind); format != nil {
		return format(e, displayFormat(e.Format))
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return "Office extraction failed"
}

func (e *Error) Unwrap() error { return e.Cause }

func officeError(kind ErrorKind, format Format, path string, cause error, detail string) error {
	return &Error{Kind: kind, Format: format, Path: path, Cause: cause, Detail: detail}
}

func missingEntry(format Format, path, entry string) error {
	return &Error{Kind: ErrMissingXML, Format: format, Path: path, Entry: entry}
}

func xmlError(format Format, path, entry string, cause error) error {
	return &Error{Kind: ErrXML, Format: format, Path: path, Entry: entry, Cause: cause}
}

type formatErrorFunc func(*Error, string) string

func errorFormatter(kind ErrorKind) formatErrorFunc {
	return map[ErrorKind]formatErrorFunc{
		ErrUnsupported: formatUnsupportedError,
		ErrCorrupt:     formatCorruptError,
		ErrMissingXML:  formatMissingXMLError,
		ErrXML:         formatXMLError,
		ErrTooLarge:    formatTooLargeError,
		ErrNoText:      formatNoTextError,
	}[kind]
}

func displayFormat(format Format) string {
	if format == "" {
		return "Office"
	}
	return string(format)
}

func formatUnsupportedError(_ *Error, _ string) string {
	return "legacy Office binary format is not supported by the Go-only first version; please provide .docx/.xlsx/.pptx."
}

func formatCorruptError(e *Error, _ string) string {
	if e.Cause != nil {
		return fmt.Sprintf("corrupt Office ZIP container %s: %v", e.Path, e.Cause)
	}
	return fmt.Sprintf("corrupt Office ZIP container %s", e.Path)
}

func formatMissingXMLError(e *Error, format string) string {
	return fmt.Sprintf("%s Office file %s is missing required XML entry %s", format, e.Path, e.Entry)
}

func formatXMLError(e *Error, format string) string {
	if e.Cause != nil {
		return fmt.Sprintf("%s Office XML parse failed in %s: %v", format, e.Entry, e.Cause)
	}
	return fmt.Sprintf("%s Office XML parse failed in %s", format, e.Entry)
}

func formatTooLargeError(e *Error, format string) string {
	if e.Detail != "" {
		return fmt.Sprintf("%s Office file %s exceeds safety limit: %s", format, e.Path, e.Detail)
	}
	return fmt.Sprintf("%s Office file %s exceeds safety limit", format, e.Path)
}

func formatNoTextError(e *Error, format string) string {
	return fmt.Sprintf("%s Office file %s contains no visible text", format, e.Path)
}
