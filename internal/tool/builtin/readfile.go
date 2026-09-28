// Package builtin provides Clouvia's compile-time built-in tools. Each tool
// self-registers via init(); main blank-imports this package to wire them in.
package builtin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/text/transform"

	"kwu/internal/fileextract/office"
	fileenc "kwu/internal/fileutil/encoding"
	"kwu/internal/tool"
)

const (
	readFileBinaryPeek   = 8 * 1024   // bytes scanned for NUL before reading further
	readFileDetectSample = 256 * 1024 // bytes sampled for encoding detection before streaming
)

func init() { tool.RegisterBuiltin(readFile{}) }

// readFile reads a text file. workDir, when non-empty, is the directory a
// relative path is resolved against (see resolveIn); the zero value registered
// at init resolves against the process working directory.
type readFile struct {
	workDir string
	roots   []string
}

const (
	readFileDefaultLimit = 2000 // lines returned when limit is unset
)

type readFileArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

func (readFile) Name() string { return "read_file" }

func (readFile) Description() string {
	return "Read a text file, or extract main readable text from .docx, .xlsx, and .pptx Office Open XML files, with optional line offset/limit. Legacy Office binary formats .doc, .xls, and .ppt are not supported by the Go-only reader. Output prefixes each line with its 1-based number (e.g. `   42→...`) so subsequent edit_file calls can target exact text-file lines; Office output line numbers refer to the extracted text, not original pages, cells, or slide objects. Use `offset` and `limit` to page through large files; the tool reports pagination hints in a trailer."
}

func (readFile) Schema() json.RawMessage {
	return json.RawMessage(`{
"type":"object",
"properties":{
  "path":{"type":"string","description":"File path"},
  "offset":{"type":"integer","description":"0-based line offset to start reading from (default 0)","minimum":0},
  "limit":{"type":"integer","description":"Maximum lines to return (default 2000)","minimum":1}
},
"required":["path"]
}`)
}

func (readFile) ReadOnly() bool { return true }

func (r readFile) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var p readFileArgs
	if err := json.Unmarshal(args, &p); err != nil {
		return "", fmt.Errorf("invalid args: %w", err)
	}
	if err := p.normalize(r.workDir); err != nil {
		return "", err
	}
	if err := confineRead(r.roots, p.Path); err != nil {
		return "", err
	}
	return r.readResolvedPath(ctx, p)
}

func (p *readFileArgs) normalize(workDir string) error {
	if p.Path == "" {
		return fmt.Errorf("path is required")
	}
	p.Path = resolveIn(workDir, p.Path)
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Limit <= 0 {
		p.Limit = readFileDefaultLimit
	}
	return nil
}

func (r readFile) readResolvedPath(ctx context.Context, p readFileArgs) (string, error) {
	// A directory can be os.Open'd but not read as text — catch it up front with
	// an actionable message (and avoid the doubled "read X: read X:" the scanner's
	// error would otherwise produce) so the model switches to the ls tool.
	if info, err := os.Stat(p.Path); err == nil && info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file — use the ls tool to list it, or read a specific file inside it", p.Path)
	}

	if office.ShouldHandle(p.Path) {
		text, err := office.Extract(ctx, p.Path, office.Options{})
		if err != nil {
			return "", err
		}
		return r.scan(strings.NewReader(text), p.Offset, p.Limit)
	}
	return r.readTextPath(p.Path, p.Offset, p.Limit)
}

func (r readFile) readTextPath(path string, offset, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	defer f.Close()

	// Peek the first 8 KiB to reject binary files cheaply (a NUL byte) before
	// reading further — keeps a multi-GB archive from being slurped just to be
	// discarded.
	peek := make([]byte, readFileBinaryPeek)
	pn, perr := io.ReadFull(f, peek)
	peek = peek[:pn]
	peekEOF := perr != nil // whole file fit in the peek (EOF / ErrUnexpectedEOF)

	return r.scanPeekedText(path, f, peek, peekEOF, offset, limit)
}

func (r readFile) scanPeekedText(path string, f *os.File, peek []byte, peekEOF bool, offset, limit int) (string, error) {
	// BOM check first: UTF-16 files contain 0x00 for every ASCII character, so a
	// naive NUL check would misidentify them as binary.
	if out, ok, err := r.scanBOMText(path, f, peek, offset, limit); ok || err != nil {
		return out, err
	}

	// BOM-less UTF-16 (Windows source files) has a NUL for every ASCII char but
	// no BOM, so it reaches here; recognise it by its NUL pattern and decode it
	// rather than rejecting it as binary.
	if out, ok, err := r.scanUTF16NoBOM(path, f, peek, offset, limit); ok || err != nil {
		return out, err
	}

	if bytes.IndexByte(peek, 0) >= 0 {
		return "", fmt.Errorf("binary file %s (NUL byte detected); use `bash hexdump` or another tool", path)
	}

	sample, src, err := readFileEncodingSample(f, peek, peekEOF)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	enc, _ := fileenc.Detect(sample)
	if dec := fileenc.Decoder(enc); dec != nil {
		return r.scan(transform.NewReader(src, dec), offset, limit)
	}
	return r.scan(src, offset, limit)
}

func (r readFile) scanBOMText(path string, f *os.File, peek []byte, offset, limit int) (string, bool, error) {
	switch fileenc.DetectQuick(peek) {
	case fileenc.UTF16LE, fileenc.UTF16BE:
		rest, err := io.ReadAll(f)
		if err != nil {
			return "", true, fmt.Errorf("read %s: %w", path, err)
		}
		all := append(peek, rest...)
		out, err := r.scan(bytes.NewReader(fileenc.Decode(all, fileenc.DetectQuick(all))), offset, limit)
		return out, true, err
	case fileenc.UTF8BOM:
		body := peek
		if len(body) >= 3 {
			body = body[3:]
		}
		out, err := r.scan(io.MultiReader(bytes.NewReader(body), f), offset, limit)
		return out, true, err
	default:
		return "", false, nil
	}
}

func (r readFile) scanUTF16NoBOM(path string, f *os.File, peek []byte, offset, limit int) (string, bool, error) {
	k, ok := fileenc.DetectUTF16NoBOM(peek)
	if !ok {
		return "", false, nil
	}
	rest, err := io.ReadAll(f)
	if err != nil {
		return "", true, fmt.Errorf("read %s: %w", path, err)
	}
	all := append(peek, rest...)
	out, err := r.scan(bytes.NewReader(fileenc.Decode(all, k)), offset, limit)
	return out, true, err
}

func readFileEncodingSample(f *os.File, peek []byte, peekEOF bool) ([]byte, io.Reader, error) {
	// Read up to a bounded sample for encoding detection, then stream the rest —
	// so a large text file isn't slurped whole just to return a few lines.
	head := peek
	if !peekEOF {
		more := make([]byte, readFileDetectSample-len(peek))
		mn, merr := io.ReadFull(f, more)
		head = append(peek, more[:mn]...)
		peekEOF = merr != nil
	}

	// Detect from a char-safe slice: when more file follows, trim to the last
	// newline so the sample never ends mid multi-byte sequence (UTF-8 and GB18030
	// are ASCII-transparent, so '\n' is always a clean boundary).
	sample := head
	if !peekEOF {
		if i := bytes.LastIndexByte(head, '\n'); i >= 0 {
			sample = head[:i+1]
		}
	}
	return sample, io.MultiReader(bytes.NewReader(head), f), nil
}

// scan reads lines from src and returns the formatted output with line numbers.
func (r readFile) scan(src io.Reader, offset, limit int) (string, error) {
	scanner := bufio.NewScanner(src)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var collected []string
	lineNo := 0
	hasMore := false
	for scanner.Scan() {
		lineNo++
		if lineNo <= offset {
			continue
		}
		if len(collected) < limit {
			collected = append(collected, scanner.Text())
			continue
		}
		// A line past the requested window exists — stop here rather than reading
		// the rest of the file just to count the remainder.
		hasMore = true
		break
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scan: %w", err)
	}

	if lineNo == 0 {
		return "(empty file)", nil
	}
	if len(collected) == 0 {
		return fmt.Sprintf("(offset %d is past EOF — file has %d lines)", offset, lineNo), nil
	}

	maxShown := offset + len(collected)
	w := len(fmt.Sprint(maxShown))

	var b strings.Builder
	for i, line := range collected {
		fmt.Fprintf(&b, "%*d→%s\n", w, offset+i+1, line)
	}
	if hasMore {
		fmt.Fprintf(&b, "\n[more lines below; pass offset=%d to continue]\n", offset+len(collected))
	}
	return b.String(), nil
}
