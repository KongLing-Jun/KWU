package diagnostics

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const (
	FailureMarkerKey = "failure_marker"
	FailureMarker    = "failure"
	SlogSourceKey    = "journal_source"
	SlogEventKey     = "journal_event"

	maxQueue             = 256
	maxLineBytes         = 2 << 10
	maxCurrentBytes      = 2 << 20
	maxJournalBytes      = 10 << 20
	maxErrorSessions     = 100
	retentionPeriod      = 7 * 24 * time.Hour
	dedupeWindow         = 30 * time.Second
	maxDedupeEntries     = 1024
	lockWait             = 250 * time.Millisecond
	closeDrainPeriod     = 500 * time.Millisecond
	housekeepingInterval = time.Hour
	lockFilename         = ".errors.lock"
	truncatedMarker      = " [truncated] "
)

var (
	phonePattern        = regexp.MustCompile(`\b(?:1[3-9]\d{9}|\d{3}[ .-]\d{3}[ .-]\d{4})\b`)
	emailPattern        = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	secretPattern       = regexp.MustCompile(`(?i)\b(api[_-]?key|access[_-]?token|refresh[_-]?token|id[_-]?token|authorization|secret|password|passwd|pwd|token)\b\s*[:=]\s*(?:Bearer\s+)?['"]?[^'"\s,;|]+['"]?`)
	bearerPattern       = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	jwtPattern          = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b`)
	explicitKeyPattern  = regexp.MustCompile(`\b(?:sk|rk)-(?:proj-)?[A-Za-z0-9_-]{12,}\b`)
	longHexPattern      = regexp.MustCompile(`\b[0-9a-fA-F]{32,}\b`)
	longTokenPattern    = regexp.MustCompile(`\b[A-Za-z0-9_+/=-]{48,}\b`)
	environmentPattern  = regexp.MustCompile(`\b[A-Z][A-Z0-9_]*(?:API[_-]?KEY|ACCESS[_-]?KEY|PRIVATE[_-]?KEY|SECRET|TOKEN|PASSWORD|PASSWD|PWD)[A-Z0-9_]*\b`)
	absolutePathPattern = regexp.MustCompile(`(?i)(?:[A-Za-z]:[\\/]|\\\\|/(?:users|home|var|tmp|private|workspace|appdata|programdata)(?:[\\/]|$))[^\s"'<>;,]+|/(?:[^/\s"'<>;,]+/)+[^/\s"'<>;,]+`)
	legacyOpaquePattern = regexp.MustCompile(`(?i)\b(prompt|tool[_ -]?input|request[_ -]?body|response[_ -]?body|headers?|details)\b\s*[:=]\s*.*`)
	operationPattern    = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)
	codePattern         = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,80}$`)
	logIDPattern        = regexp.MustCompile(`^[a-z0-9]{6}-[0-9a-z]{4,6}$`)
	partPattern         = regexp.MustCompile(`^(.*)\.\d{3}\.log$`)
	legacyPattern       = regexp.MustCompile(`(?i)\.jsonl$`)
)

var opaqueLabels = []string{
	"authorization", "request_body", "request body", "response_body", "response body",
	"headers", "header", "details", "prompt", "tool_input", "tool input", "file_content", "file content",
}

// OwnsLogID reports whether value was issued by this startup journal session.
func (j *Journal) OwnsLogID(value string) bool {
	value = strings.TrimSpace(value)
	if j == nil || !logIDPattern.MatchString(value) {
		return false
	}
	sequenceText, ok := strings.CutPrefix(value, j.sessionID+"-")
	if !ok {
		return false
	}
	sequence, err := strconv.ParseUint(sequenceText, 36, 64)
	return err == nil && sequence > 0 && sequenceText == leftPadBase36(sequence, 4) && sequence <= j.sequence.Load()
}

type Frame struct {
	Category string `json:"category"`
	Ordinal  int    `json:"ordinal"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

// Event is an in-memory diagnostic envelope. Cause remains worker-local and is
// never returned to a renderer or encoded as a structured payload.
type Event struct {
	SchemaVersion   int       `json:"schema_version"`
	OccurredAt      time.Time `json:"occurred_at"`
	Severity        string    `json:"severity"`
	Source          string    `json:"source"`
	Event           string    `json:"event"`
	Scope           string    `json:"scope,omitempty"`
	Code            string    `json:"code,omitempty"`
	DiagnosticID    string    `json:"diagnostic_id,omitempty"`
	Operation       string    `json:"operation,omitempty"`
	CallsiteID      uint64    `json:"callsite_id,omitempty"`
	RequestID       string    `json:"request_id,omitempty"`
	HTTPStatus      int       `json:"http_status"`
	Retryable       bool      `json:"retryable"`
	ErrorType       string    `json:"error_type,omitempty"`
	CauseType       string    `json:"cause_type,omitempty"`
	Summary         string    `json:"summary,omitempty"`
	StackFrames     []Frame   `json:"stack_frames,omitempty"`
	ComponentFrames []Frame   `json:"component_frames,omitempty"`
	Phase           string    `json:"phase,omitempty"`
	StateBefore     string    `json:"state_before,omitempty"`
	StateAfter      string    `json:"state_after,omitempty"`
	Outcome         string    `json:"outcome,omitempty"`
	AccessPresent   bool      `json:"access_present"`
	RefreshPresent  bool      `json:"refresh_present"`
	AccessExpiry    time.Time `json:"access_expiry,omitempty"`
	RefreshExpiry   time.Time `json:"refresh_expiry,omitempty"`
	AppVersion      string    `json:"app_version,omitempty"`
	Channel         string    `json:"channel,omitempty"`
	BuildRevision   string    `json:"build_revision,omitempty"`
	EnvironmentID   string    `json:"environment_id,omitempty"`
	OS              string    `json:"os,omitempty"`
	Arch            string    `json:"arch,omitempty"`
	PID             int       `json:"pid,omitempty"`
	Cause           error     `json:"-"`
	UserVisible     bool      `json:"-"`
}

type Metadata struct {
	AppVersion, Channel, BuildRevision, EnvironmentID string
}

type journalLockAcquire func(string, time.Duration) (*journalLock, bool)
type journalOptions struct {
	now            func() time.Time
	metadata       Metadata
	ops            journalFileOps
	lockAcquire    journalLockAcquire
	recordObserver func()
}
type Option func(*journalOptions)

func WithClock(now func() time.Time) Option {
	return func(o *journalOptions) {
		if now != nil {
			o.now = now
		}
	}
}
func WithMetadata(metadata Metadata) Option { return func(o *journalOptions) { o.metadata = metadata } }
func withFileOps(ops journalFileOps) Option { return func(o *journalOptions) { o.ops = ops } }
func withLockAcquire(acquire journalLockAcquire) Option {
	return func(o *journalOptions) {
		if acquire != nil {
			o.lockAcquire = acquire
		}
	}
}
func withRecordObserver(observer func()) Option {
	return func(o *journalOptions) { o.recordObserver = observer }
}

type journalFileOps struct {
	mkdirAll  func(string, os.FileMode) error
	chmodPath func(string, os.FileMode) error
	readDir   func(string) ([]os.DirEntry, error)
	stat      func(string) (os.FileInfo, error)
	openFile  func(string, int, os.FileMode) (*os.File, error)
	remove    func(string) error
	rename    func(string, string) error
	chmodFile func(*os.File, os.FileMode) error
	seek      func(*os.File, int64, int) (int64, error)
	write     func(*os.File, []byte) (int, error)
	sync      func(*os.File) error
	truncate  func(*os.File, int64) error
	close     func(*os.File) error
}

func defaultJournalFileOps() journalFileOps {
	return journalFileOps{
		mkdirAll: os.MkdirAll, chmodPath: os.Chmod, readDir: os.ReadDir, stat: os.Stat,
		openFile: os.OpenFile, remove: os.Remove, rename: os.Rename,
		chmodFile: func(f *os.File, m os.FileMode) error { return f.Chmod(m) },
		seek:      func(f *os.File, o int64, w int) (int64, error) { return f.Seek(o, w) },
		write:     func(f *os.File, p []byte) (int, error) { return f.Write(p) },
		sync:      func(f *os.File) error { return f.Sync() },
		truncate:  func(f *os.File, s int64) error { return f.Truncate(s) },
		close:     func(f *os.File) error { return f.Close() },
	}
}

type dedupeRecord struct {
	at  time.Time
	seq uint64
}
type queuedEvent struct {
	event Event
	ready chan struct{}
}
type Journal struct {
	path           string
	queue          chan *queuedEvent
	stop           chan struct{}
	done           chan struct{}
	closed         atomic.Bool
	inflight       atomic.Int64
	closeOnce      sync.Once
	sequence       atomic.Uint64
	now            func() time.Time
	metadata       Metadata
	ops            journalFileOps
	lockAcquire    journalLockAcquire
	recordObserver func()
	startupAt      time.Time
	sessionID      string
	part           int
	partSize       int64
	dedupe         map[string]dedupeRecord
	dedupeSeq      uint64
}

func New(path string, options ...Option) *Journal {
	configured := journalOptions{now: time.Now, ops: defaultJournalFileOps(), lockAcquire: acquireJournalLock}
	for _, option := range options {
		if option != nil {
			option(&configured)
		}
	}
	if configured.ops.mkdirAll == nil {
		configured.ops = defaultJournalFileOps()
	}
	if configured.lockAcquire == nil {
		configured.lockAcquire = acquireJournalLock
	}
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if strings.TrimSpace(path) == "" || cleanPath == "." {
		cleanPath = ""
	}
	startupAt := configured.now()
	j := &Journal{
		path: cleanPath, queue: make(chan *queuedEvent, maxQueue), stop: make(chan struct{}), done: make(chan struct{}),
		now: configured.now, metadata: configured.metadata, ops: configured.ops, lockAcquire: configured.lockAcquire,
		recordObserver: configured.recordObserver, startupAt: startupAt, sessionID: newSessionID(), dedupe: make(map[string]dedupeRecord),
	}
	go j.run()
	return j
}

// RecordWithID does only atomic bookkeeping, short-ID projection, and a
// nonblocking queue select. Queue/full/closed rejection returns no UI ID.
func (j *Journal) RecordWithID(event Event) string {
	if j == nil || j.path == "" || j.closed.Load() {
		return ""
	}
	j.inflight.Add(1)
	defer j.inflight.Add(-1)
	if j.closed.Load() {
		return ""
	}
	select {
	case <-j.stop:
		return ""
	default:
	}
	queued := &queuedEvent{ready: make(chan struct{})}
	select {
	case j.queue <- queued:
		if j.recordObserver != nil {
			j.recordObserver()
		}
		id := j.nextLogID()
		event.DiagnosticID = id
		queued.event = event
		close(queued.ready)
		return id
	default:
		return ""
	}
}

func (j *Journal) Record(event Event) bool { return j.RecordWithID(event) != "" }
func (j *Journal) nextLogID() string       { return j.sessionID + "-" + leftPadBase36(j.sequence.Add(1), 4) }

func (j *Journal) Close() {
	if j == nil {
		return
	}
	j.closeOnce.Do(func() { j.closed.Store(true); close(j.stop) })
	select {
	case <-j.done:
	case <-time.After(closeDrainPeriod):
	}
}

func (j *Journal) run() {
	defer close(j.done)
	if j.path == "" {
		return
	}
	j.housekeep()
	ticker := time.NewTicker(housekeepingInterval)
	defer ticker.Stop()
	for {
		select {
		case queued := <-j.queue:
			<-queued.ready
			j.process(queued.event)
		case <-ticker.C:
			j.housekeep()
		case <-j.stop:
			j.drain()
			return
		}
	}
}

func (j *Journal) drain() {
	deadline := time.Now().Add(closeDrainPeriod)
	for time.Now().Before(deadline) {
		select {
		case queued := <-j.queue:
			<-queued.ready
			j.process(queued.event)
		default:
			if j.inflight.Load() == 0 {
				return
			}
			runtime.Gosched()
		}
	}
}

func (j *Journal) process(event Event) {
	event = j.normalizeEvent(event)
	cause, deepest := eventCause(event)
	if !event.UserVisible && j.suppressDuplicate(event.Operation, cause, event.OccurredAt) {
		return
	}
	line := formatLine(event, cause, deepest)
	if line != "" {
		j.writeLine([]byte(line))
	}
}

func (j *Journal) suppressDuplicate(operation, cause string, at time.Time) bool {
	key := operation + "\x00" + cause
	if previous, ok := j.dedupe[key]; ok && at.Sub(previous.at) >= 0 && at.Sub(previous.at) < dedupeWindow {
		return true
	}
	j.dedupeSeq++
	j.dedupe[key] = dedupeRecord{at: at, seq: j.dedupeSeq}
	if len(j.dedupe) > maxDedupeEntries {
		oldestKey := ""
		var oldest dedupeRecord
		for key, record := range j.dedupe {
			if oldestKey == "" || record.seq < oldest.seq {
				oldestKey, oldest = key, record
			}
		}
		delete(j.dedupe, oldestKey)
	}
	return false
}

func (j *Journal) normalizeEvent(event Event) Event {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = j.now()
	} else {
		event.OccurredAt = event.OccurredAt.Local()
	}
	if strings.EqualFold(strings.TrimSpace(event.Severity), "warn") || strings.EqualFold(strings.TrimSpace(event.Severity), "warning") {
		event.Severity = "WARN"
	} else {
		event.Severity = "ERROR"
	}
	event.Operation = normalizeOperation(event)
	event.Code = normalizeToken(event.Code, codePattern)
	event.RequestID = normalizeToken(scrubJournalText(event.RequestID), operationPattern)
	if event.HTTPStatus < 100 || event.HTTPStatus > 599 {
		event.HTTPStatus = 0
	}
	event.Phase = normalizeText(event.Phase, 80)
	event.StateBefore = normalizeText(event.StateBefore, 80)
	event.StateAfter = normalizeText(event.StateAfter, 80)
	event.Outcome = normalizeText(event.Outcome, 80)
	return event
}

func normalizeOperation(event Event) string {
	for _, value := range []string{event.Operation, event.Scope, event.Event, event.Source} {
		value = strings.TrimSpace(value)
		if operationPattern.MatchString(value) {
			return value
		}
	}
	return "unknown"
}
func normalizeToken(value string, pattern *regexp.Regexp) string {
	value = strings.TrimSpace(value)
	if pattern.MatchString(value) {
		return value
	}
	return ""
}
func normalizeText(value string, max int) string {
	return clipUTF8(flattenWhitespace(scrubJournalText(value)), max)
}

func eventCause(event Event) (string, string) {
	if event.Cause != nil {
		cause := concreteErrorType(event.Cause) + ": " + event.Cause.Error()
		if context := flattenWhitespace(scrubJournalText(event.Summary)); context != "" {
			cause = context + ": " + cause
		}
		cause = flattenWhitespace(scrubJournalText(cause))
		deepest := flattenWhitespace(scrubJournalText(deepestCauseText(event.Cause)))
		if cause != "" {
			return cause, deepest
		}
	}
	for _, value := range []string{event.Summary, event.ErrorType, event.Event, "error"} {
		value = flattenWhitespace(scrubJournalText(value))
		if value != "" {
			return value, value
		}
	}
	return "error", "error"
}

func concreteErrorType(err error) string {
	if err == nil {
		return "error"
	}
	return fmt.Sprintf("%T", err)
}

func deepestCauseText(err error) string {
	if err == nil {
		return ""
	}
	type multiUnwrapper interface{ Unwrap() []error }
	if multi, ok := err.(multiUnwrapper); ok {
		children := multi.Unwrap()
		for index := len(children) - 1; index >= 0; index-- {
			if value := deepestCauseText(children[index]); value != "" {
				return value
			}
		}
	}
	if child := errors.Unwrap(err); child != nil {
		if value := deepestCauseText(child); value != "" {
			return value
		}
	}
	return err.Error()
}

func formatLine(event Event, cause, deepest string) string {
	prefix := event.OccurredAt.Format("15:04:05.000") + " " + event.Severity + " " + event.DiagnosticID + " " + event.Operation
	if event.Code != "" {
		prefix += " [" + event.Code + "]"
	}
	line := prefix + " | " + cause
	if context := presentContext(event); context != "" {
		line += " | " + context
	}
	return capLine(line, prefix+" | ", deepest) + "\n"
}

func presentContext(event Event) string {
	items := make([]string, 0, 10)
	appendValue := func(key, value string) {
		if value != "" {
			items = append(items, key+"="+value)
		}
	}
	appendValue("request_id", event.RequestID)
	if event.HTTPStatus != 0 {
		appendValue("status", strconv.Itoa(event.HTTPStatus))
	}
	if event.Retryable {
		appendValue("retryable", "true")
	}
	appendValue("phase", event.Phase)
	appendValue("before", event.StateBefore)
	appendValue("after", event.StateAfter)
	appendValue("outcome", event.Outcome)
	if event.CallsiteID != 0 {
		appendValue("callsite", strconv.FormatUint(event.CallsiteID, 36))
	}
	return strings.Join(items, " ")
}

func capLine(line, prefix, deepest string) string {
	limit := maxLineBytes - 1
	if len(line) <= limit {
		return line
	}
	deepest = flattenWhitespace(deepest)
	tail := tailUTF8(deepest, limit/3)
	startBudget := limit - len(truncatedMarker) - len(tail)
	if startBudget < len(prefix) {
		tail = tailUTF8(deepest, limit-len(prefix)-len(truncatedMarker))
		startBudget = limit - len(truncatedMarker) - len(tail)
	}
	return clipUTF8(line, startBudget) + truncatedMarker + tail
}
func flattenWhitespace(value string) string { return strings.Join(strings.Fields(value), " ") }
func clipUTF8(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	value = value[:max]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[:len(value)-1]
	}
	return value
}
func tailUTF8(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	value = value[len(value)-max:]
	for !utf8.ValidString(value) && len(value) > 0 {
		value = value[1:]
	}
	return value
}

func scrubJournalText(value string) string {
	value = redactOpaqueValues(value)
	value = bearerPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = secretPattern.ReplaceAllStringFunc(value, func(match string) string {
		separator := strings.IndexAny(match, ":=")
		if separator < 0 {
			return "[redacted-credential]"
		}
		label := strings.ToLower(strings.TrimSpace(match[:separator]))
		return label + "=[redacted-" + strings.ReplaceAll(label, "_", "-") + "]"
	})
	value = emailPattern.ReplaceAllString(value, "[redacted-email]")
	value = phonePattern.ReplaceAllString(value, "[redacted-phone]")
	value = jwtPattern.ReplaceAllString(value, "[redacted-jwt]")
	value = explicitKeyPattern.ReplaceAllString(value, "[redacted-key]")
	value = longHexPattern.ReplaceAllString(value, "[redacted-token]")
	return longTokenPattern.ReplaceAllString(value, "[redacted-token]")
}

// ScrubSensitiveText preserves the established crash/feedback privacy
// contract. The local error journal uses scrubJournalText so its private,
// on-device lines may retain useful local file paths as required by R2.
func ScrubSensitiveText(value string) string {
	value = absolutePathPattern.ReplaceAllString(value, "[redacted-path]")
	value = legacyOpaquePattern.ReplaceAllString(value, "${1}=[redacted]")
	value = bearerPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = secretPattern.ReplaceAllString(value, "${1}=[redacted]")
	value = environmentPattern.ReplaceAllString(value, "[redacted-env]")
	value = emailPattern.ReplaceAllString(value, "[redacted-email]")
	value = phonePattern.ReplaceAllString(value, "[redacted-phone]")
	value = jwtPattern.ReplaceAllString(value, "[redacted-jwt]")
	value = explicitKeyPattern.ReplaceAllString(value, "[redacted-key]")
	value = longHexPattern.ReplaceAllString(value, "[redacted-hex]")
	return longTokenPattern.ReplaceAllString(value, "[redacted-token]")
}

func redactOpaqueValues(value string) string {
	var result strings.Builder
	remaining := value
	for remaining != "" {
		start, label := nextOpaqueLabel(strings.ToLower(remaining))
		if start < 0 {
			result.WriteString(remaining)
			break
		}
		separator := start + len(label)
		for separator < len(remaining) && (remaining[separator] == ' ' || remaining[separator] == '\t') {
			separator++
		}
		if separator >= len(remaining) || (remaining[separator] != ':' && remaining[separator] != '=') {
			result.WriteString(remaining[:start+len(label)])
			remaining = remaining[start+len(label):]
			continue
		}
		end := len(remaining)
		for _, delimiter := range []string{";", "|", "\n", "\r"} {
			if offset := strings.Index(remaining[separator+1:], delimiter); offset >= 0 && separator+1+offset < end {
				end = separator + 1 + offset
			}
		}
		result.WriteString(remaining[:separator])
		result.WriteString("=[redacted]")
		remaining = remaining[end:]
	}
	return result.String()
}
func nextOpaqueLabel(lower string) (int, string) {
	best, label := -1, ""
	for _, candidate := range opaqueLabels {
		if index := strings.Index(lower, candidate); index >= 0 && (best < 0 || index < best) {
			best, label = index, candidate
		}
	}
	return best, label
}

func (j *Journal) writeLine(payload []byte) bool {
	if len(payload) == 0 || len(payload) > maxLineBytes {
		return false
	}
	if err := j.ops.mkdirAll(j.path, 0o700); err != nil {
		return false
	}
	_ = j.ops.chmodPath(j.path, 0o700)
	lock, ok := j.lockAcquire(j.path, lockWait)
	if !ok {
		return false
	}
	defer lock.Close()
	if !j.cleanupFiles(j.now(), int64(len(payload))) {
		return false
	}
	if j.partSize+int64(len(payload)) > maxCurrentBytes {
		j.part++
		j.partSize = 0
	}
	path := j.sessionPartPath()
	if err := j.ops.mkdirAll(filepath.Dir(path), 0o700); err != nil {
		return false
	}
	_ = j.ops.chmodPath(filepath.Dir(path), 0o700)
	file, err := j.ops.openFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return false
	}
	_ = j.ops.chmodFile(file, 0o600)
	offset, err := j.ops.seek(file, 0, io.SeekEnd)
	if err != nil || offset+int64(len(payload)) > maxCurrentBytes {
		_ = j.ops.close(file)
		return false
	}
	written, writeErr := j.ops.write(file, payload)
	if writeErr != nil || written != len(payload) || j.ops.sync(file) != nil {
		_ = j.ops.truncate(file, offset)
		_ = j.ops.sync(file)
		_ = j.ops.close(file)
		return false
	}
	_ = j.ops.close(file)
	j.partSize = offset + int64(len(payload))
	return true
}

func (j *Journal) housekeep() {
	if _, err := j.ops.stat(j.path); err != nil {
		return
	}
	lock, ok := j.lockAcquire(j.path, lockWait)
	if !ok {
		return
	}
	defer lock.Close()
	_ = j.cleanupFiles(j.now(), 0)
}

func (j *Journal) sessionPartPath() string {
	dir := filepath.Join(j.path, j.startupAt.Format("2006-01-02"))
	base := j.startupAt.Format("150405") + "-" + j.sessionID
	if j.part == 0 {
		return filepath.Join(dir, base+".log")
	}
	return filepath.Join(dir, fmt.Sprintf("%s.%03d.log", base, j.part))
}

type retainedFile struct {
	path    string
	info    os.FileInfo
	session string
}

func (j *Journal) cleanupFiles(now time.Time, required int64) bool {
	files, err := j.retainedFiles(j.path)
	if errors.Is(err, os.ErrNotExist) {
		return required <= maxJournalBytes
	}
	if err != nil {
		return false
	}
	for index := range files {
		if files[index].info.ModTime().Before(now.Add(-retentionPeriod)) && j.ops.remove(files[index].path) == nil {
			files[index].info = nil
		}
	}
	files = compactRetained(files)
	sessions := groupSessions(files)
	currentSession := filepath.Clean(filepath.Join(j.path, j.startupAt.Format("2006-01-02"), j.startupAt.Format("150405")+"-"+j.sessionID))
	currentExists := false
	for _, session := range sessions {
		if filepath.Clean(session.key) == currentSession {
			currentExists = true
			break
		}
	}
	maxExistingSessions := maxErrorSessions
	if required > 0 && !currentExists {
		maxExistingSessions--
	}
	for len(sessions) > maxExistingSessions {
		oldest := sessions[0]
		for _, file := range oldest.files {
			_ = j.ops.remove(file.path)
		}
		sessions = sessions[1:]
		files = removeSession(files, oldest.key)
	}
	sort.Slice(files, func(a, b int) bool {
		if files[a].info.ModTime().Equal(files[b].info.ModTime()) {
			return files[a].path < files[b].path
		}
		return files[a].info.ModTime().Before(files[b].info.ModTime())
	})
	var total int64
	for _, file := range files {
		total += file.info.Size()
	}
	currentPrefix := filepath.Clean(filepath.Join(j.path, j.startupAt.Format("2006-01-02"), j.startupAt.Format("150405")+"-"+j.sessionID))
	for total+required > maxJournalBytes {
		removed := false
		for index, file := range files {
			if strings.HasPrefix(filepath.Clean(file.path), currentPrefix) {
				continue
			}
			if j.ops.remove(file.path) != nil {
				continue
			}
			total -= file.info.Size()
			files = append(files[:index], files[index+1:]...)
			removed = true
			break
		}
		if !removed {
			return total+required <= maxJournalBytes
		}
	}
	return true
}

func (j *Journal) retainedFiles(dir string) ([]retainedFile, error) {
	entries, err := j.ops.readDir(dir)
	if err != nil {
		return nil, err
	}
	var files []retainedFile
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			nested, nestedErr := j.retainedFiles(path)
			if nestedErr == nil {
				files = append(files, nested...)
			}
			continue
		}
		if entry.Name() == lockFilename || (!strings.HasSuffix(strings.ToLower(entry.Name()), ".log") && !legacyPattern.MatchString(entry.Name())) {
			continue
		}
		info, statErr := j.ops.stat(path)
		if statErr != nil {
			continue
		}
		session := ""
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".log") {
			base := strings.TrimSuffix(entry.Name(), ".log")
			if match := partPattern.FindStringSubmatch(entry.Name()); len(match) == 2 {
				base = match[1]
			}
			session = filepath.Join(filepath.Dir(path), base)
		}
		files = append(files, retainedFile{path: path, info: info, session: session})
	}
	return files, nil
}
func compactRetained(files []retainedFile) []retainedFile {
	kept := files[:0]
	for _, file := range files {
		if file.info != nil {
			kept = append(kept, file)
		}
	}
	return kept
}

type retainedSession struct {
	key   string
	at    time.Time
	files []retainedFile
}

func groupSessions(files []retainedFile) []retainedSession {
	byKey := make(map[string]*retainedSession)
	for _, file := range files {
		if file.session == "" || file.info.Size() == 0 {
			continue
		}
		group := byKey[file.session]
		if group == nil {
			group = &retainedSession{key: file.session, at: file.info.ModTime()}
			byKey[file.session] = group
		}
		if file.info.ModTime().Before(group.at) {
			group.at = file.info.ModTime()
		}
		group.files = append(group.files, file)
	}
	sessions := make([]retainedSession, 0, len(byKey))
	for _, session := range byKey {
		sessions = append(sessions, *session)
	}
	sort.Slice(sessions, func(a, b int) bool {
		if sessions[a].at.Equal(sessions[b].at) {
			return sessions[a].key < sessions[b].key
		}
		return sessions[a].at.Before(sessions[b].at)
	})
	return sessions
}
func removeSession(files []retainedFile, session string) []retainedFile {
	kept := files[:0]
	for _, file := range files {
		if file.session != session {
			kept = append(kept, file)
		}
	}
	return kept
}

func newSessionID() string {
	var value [4]byte
	if _, err := rand.Read(value[:]); err == nil {
		return leftPadBase36(uint64(binary.LittleEndian.Uint32(value[:]))%2176782336, 6)
	}
	return leftPadBase36(uint64(time.Now().UnixNano())%2176782336, 6)
}
func leftPadBase36(value uint64, width int) string {
	encoded := strconv.FormatUint(value, 36)
	if len(encoded) < width {
		encoded = strings.Repeat("0", width-len(encoded)) + encoded
	}
	return encoded
}

func NewHandler(delegate slog.Handler, journal *Journal) slog.Handler {
	if delegate == nil {
		delegate = slog.NewTextHandler(io.Discard, nil)
	}
	return &fanoutHandler{delegate: delegate, journal: journal}
}

type fanoutHandler struct {
	delegate slog.Handler
	journal  *Journal
	attrs    []slog.Attr
}

func (h *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.delegate.Enabled(ctx, level) || level >= slog.LevelWarn
}
func (h *fanoutHandler) Handle(ctx context.Context, record slog.Record) error {
	var delegateErr error
	if h.delegate.Enabled(ctx, record.Level) {
		delegateErr = h.delegate.Handle(ctx, record)
	}
	if h.journal != nil {
		if event, ok := eventFromRecord(record, h.attrs); ok {
			h.journal.Record(event)
		}
	}
	return delegateErr
}
func (h *fanoutHandler) EnqueueAuthFailure(attrs []slog.Attr) bool {
	if h == nil || h.journal == nil {
		return false
	}
	values := make(map[string]slog.Value)
	collectAttrs(values, append(append([]slog.Attr(nil), h.attrs...), attrs...))
	event, ok := authFailureEvent(values)
	return ok && h.journal.Record(event)
}
func (h *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := &fanoutHandler{delegate: h.delegate.WithAttrs(attrs), journal: h.journal, attrs: append([]slog.Attr(nil), h.attrs...)}
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}
func (h *fanoutHandler) WithGroup(name string) slog.Handler {
	return &fanoutHandler{delegate: h.delegate.WithGroup(name), journal: h.journal, attrs: append([]slog.Attr(nil), h.attrs...)}
}

func eventFromRecord(record slog.Record, inherited []slog.Attr) (Event, bool) {
	attrs := append([]slog.Attr(nil), inherited...)
	record.Attrs(func(attr slog.Attr) bool { attrs = append(attrs, attr); return true })
	values := make(map[string]slog.Value)
	collectAttrs(values, attrs)
	if event, ok := authFailureEvent(values); ok {
		return event, true
	}
	if record.Level < slog.LevelError && !(record.Level >= slog.LevelWarn && valueString(values, FailureMarkerKey) == FailureMarker) {
		return Event{}, false
	}
	severity, operation := "error", "go.error"
	if record.Level < slog.LevelError {
		severity, operation = "warn", valueString(values, "scope")
	}
	return Event{OccurredAt: record.Time, Severity: severity, Source: "go", Event: "slog.failure", Scope: operation, Operation: operation, Code: valueString(values, "code"), CallsiteID: stableCallsiteID(record.PC), Summary: record.Message, Cause: valueError(values, "err"), ErrorType: "Error"}, true
}
func authFailureEvent(values map[string]slog.Value) (Event, bool) {
	if valueString(values, SlogSourceKey) != "cloud" || valueString(values, SlogEventKey) != "auth.failure" {
		return Event{}, false
	}
	return Event{Severity: "error", Source: "cloud", Event: "auth.failure", Scope: "cloud.auth", Operation: "cloud.auth", Code: valueString(values, "code"), RequestID: valueString(values, "request_id"), HTTPStatus: valueInt(values, "http_status"), Retryable: valueBool(values, "retryable"), Phase: valueString(values, "phase"), StateBefore: valueString(values, "state_before"), StateAfter: valueString(values, "state_after"), Outcome: valueString(values, "outcome"), Summary: "authentication failure", Cause: valueError(values, "err")}, true
}
func stableCallsiteID(pc uintptr) uint64 {
	if pc == 0 {
		return 0
	}
	function := runtime.FuncForPC(pc)
	if function == nil {
		return 0
	}
	_, line := function.FileLine(pc)
	hash := uint64(1469598103934665603)
	for _, value := range []byte(function.Name() + "\x00" + strconv.Itoa(line)) {
		hash ^= uint64(value)
		hash *= 1099511628211
	}
	return hash
}
func collectAttrs(values map[string]slog.Value, attrs []slog.Attr) {
	for _, attr := range attrs {
		if attr.Equal(slog.Attr{}) || attr.Key == "" {
			continue
		}
		if attr.Value.Kind() == slog.KindGroup {
			collectAttrs(values, attr.Value.Group())
			continue
		}
		if _, exists := values[attr.Key]; !exists {
			values[attr.Key] = attr.Value
		}
	}
}
func valueString(values map[string]slog.Value, key string) string {
	value := values[key]
	if value.Kind() == slog.KindString {
		return value.String()
	}
	return ""
}
func valueError(values map[string]slog.Value, key string) error {
	value := values[key]
	if value.Kind() == slog.KindAny {
		if err, ok := value.Any().(error); ok {
			return err
		}
	}
	return nil
}
func valueInt(values map[string]slog.Value, key string) int {
	value := values[key]
	switch value.Kind() {
	case slog.KindInt64:
		return int(value.Int64())
	case slog.KindUint64:
		return int(value.Uint64())
	default:
		return 0
	}
}
func valueBool(values map[string]slog.Value, key string) bool {
	value := values[key]
	return value.Kind() == slog.KindBool && value.Bool()
}

var _ slog.Handler = (*fanoutHandler)(nil)
