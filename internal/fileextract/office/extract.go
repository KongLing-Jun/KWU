package office

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type Options struct {
	Limits Limits
}

type archive struct {
	files  map[string]*zip.File
	limits Limits
}

func Extract(ctx context.Context, p string, opts Options) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	format, err := formatForExtract(p)
	if err != nil {
		return "", err
	}
	limits := opts.Limits.withDefaults()
	if err := checkInputSize(p, format, limits); err != nil {
		return "", err
	}
	a, closeArchive, err := openArchive(ctx, p, format, limits)
	if err != nil {
		return "", err
	}
	defer closeArchive()
	rendered, err := extractByFormat(ctx, a, format, p)
	if err != nil {
		return "", err
	}
	return finalizeOutput(rendered, format, p, limits)
}

func formatForExtract(p string) (Format, error) {
	if LegacyFormat(p) {
		return "", &Error{Kind: ErrUnsupported, Path: p}
	}
	format, ok := SupportedFormat(p)
	if !ok {
		return "", fmt.Errorf("not a supported Office format: %s", p)
	}
	return format, nil
}

func checkInputSize(p string, format Format, limits Limits) error {
	info, err := os.Stat(p)
	if err != nil {
		return fmt.Errorf("read %s: %w", p, err)
	}
	if info.Size() > limits.MaxInputBytes {
		return officeError(ErrTooLarge, format, p, nil, fmt.Sprintf("input size %d bytes > %d bytes", info.Size(), limits.MaxInputBytes))
	}
	return nil
}

func openArchive(ctx context.Context, p string, format Format, limits Limits) (*archive, func(), error) {
	zr, err := zip.OpenReader(p)
	if err != nil {
		return nil, func() {}, officeError(ErrCorrupt, format, p, err, "")
	}
	a, err := newArchive(ctx, &zr.Reader, limits, format, p)
	if err != nil {
		zr.Close()
		return nil, func() {}, err
	}
	return a, func() { zr.Close() }, nil
}

func extractByFormat(ctx context.Context, a *archive, format Format, p string) (string, error) {
	switch format {
	case FormatDOCX:
		return extractDOCX(ctx, a, p)
	case FormatXLSX:
		return extractXLSX(ctx, a, p)
	case FormatPPTX:
		return extractPPTX(ctx, a, p)
	default:
		return "", fmt.Errorf("unsupported Office format: %s", format)
	}
}

func finalizeOutput(rendered string, format Format, p string, limits Limits) (string, error) {
	rendered = limitOutput(rendered, limits.MaxOutputBytes)
	if !hasVisibleBody(rendered) {
		return "", &Error{Kind: ErrNoText, Format: format, Path: p}
	}
	return rendered, nil
}

func newArchive(ctx context.Context, zr *zip.Reader, limits Limits, format Format, p string) (*archive, error) {
	if len(zr.File) > limits.MaxEntries {
		return nil, officeError(ErrTooLarge, format, p, nil, fmt.Sprintf("zip entry count %d > %d", len(zr.File), limits.MaxEntries))
	}
	files := make(map[string]*zip.File, len(zr.File))
	var total uint64
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		total += f.UncompressedSize64
		if total > limits.MaxUncompressedBytes {
			return nil, officeError(ErrTooLarge, format, p, nil, fmt.Sprintf("zip uncompressed size > %d bytes", limits.MaxUncompressedBytes))
		}
		name := cleanZipName(f.Name)
		if name != "" {
			files[name] = f
		}
	}
	return &archive{files: files, limits: limits}, nil
}

func cleanZipName(name string) string {
	name = filepath.ToSlash(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "/")
	name = path.Clean(name)
	if name == "." || strings.HasPrefix(name, "../") || name == ".." {
		return ""
	}
	return name
}

func (a *archive) readXML(ctx context.Context, format Format, source, entry string) ([]byte, error) {
	f, ok := a.files[cleanZipName(entry)]
	if !ok {
		return nil, missingEntry(format, source, entry)
	}
	if f.UncompressedSize64 > a.limits.MaxXMLBytes {
		return nil, officeError(ErrTooLarge, format, source, nil, fmt.Sprintf("XML entry %s size %d bytes > %d bytes", entry, f.UncompressedSize64, a.limits.MaxXMLBytes))
	}
	r, err := f.Open()
	if err != nil {
		return nil, officeError(ErrCorrupt, format, source, err, "")
	}
	defer r.Close()
	limited := io.LimitReader(r, int64(a.limits.MaxXMLBytes)+1)
	b, err := io.ReadAll(limited)
	if err != nil {
		return nil, officeError(ErrCorrupt, format, source, err, "")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if uint64(len(b)) > a.limits.MaxXMLBytes {
		return nil, officeError(ErrTooLarge, format, source, nil, fmt.Sprintf("XML entry %s exceeded %d bytes", entry, a.limits.MaxXMLBytes))
	}
	return b, nil
}

func (a *archive) readXMLIfExists(ctx context.Context, format Format, source, entry string) ([]byte, bool, error) {
	if _, ok := a.files[cleanZipName(entry)]; !ok {
		return nil, false, nil
	}
	b, err := a.readXML(ctx, format, source, entry)
	return b, true, err
}

func (a *archive) entriesWithPrefixSuffix(prefix, suffix string) []string {
	var names []string
	for name := range a.files {
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			names = append(names, name)
		}
	}
	sortStrings(names)
	return names
}

func relTarget(basePart, target string) string {
	target = filepath.ToSlash(strings.TrimSpace(target))
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "/") {
		return cleanZipName(target)
	}
	return cleanZipName(path.Join(path.Dir(basePart), target))
}
