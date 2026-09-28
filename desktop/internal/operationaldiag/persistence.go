package operationaldiag

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	defaultPersistenceQueueCapacity = 256
	defaultTerminalReserve          = 32
	defaultMaxFileBytes             = 10 << 20
	defaultMaxFiles                 = 5
)

type persistenceCommand struct {
	close bool
	done  chan persistenceResult
}

type persistenceResult struct {
	ordinal uint64
	err     error
}

type persistentWriter struct {
	directory       string
	runID           string
	maxFileBytes    int64
	maxFiles        int
	terminalReserve int
	records         chan Record
	commands        chan persistenceCommand
	done            chan struct{}
	counters        *counterState
	committed       atomic.Uint64
	closed          atomic.Bool
	closeOnce       sync.Once
	beforeWrite     func()

	file     *os.File
	buffer   *bufio.Writer
	fileSize int64
	sequence int
}

func newPersistentWriter(options PersistentOptions, counters *counterState) (*persistentWriter, error) {
	directory := strings.TrimSpace(options.Directory)
	if directory == "" || !filepath.IsAbs(directory) {
		return nil, errors.New("operational evidence directory must be absolute")
	}
	runID := normalizeOpaqueID(options.RunID)
	if runID == "" {
		return nil, errors.New("operational evidence run id is invalid")
	}
	queueCapacity := options.QueueCapacity
	if queueCapacity < 2 {
		queueCapacity = defaultPersistenceQueueCapacity
	}
	reserve := options.TerminalReserve
	if reserve < 1 {
		reserve = defaultTerminalReserve
	}
	if reserve >= queueCapacity {
		reserve = queueCapacity / 4
	}
	maxFileBytes := options.MaxFileBytes
	if maxFileBytes < 1024 {
		maxFileBytes = defaultMaxFileBytes
	}
	maxFiles := options.MaxFiles
	if maxFiles < 1 {
		maxFiles = defaultMaxFiles
	}
	directory = filepath.Join(filepath.Clean(directory), runID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create operational evidence directory: %w", err)
	}
	w := &persistentWriter{
		directory: directory, runID: runID, maxFileBytes: maxFileBytes, maxFiles: maxFiles,
		terminalReserve: reserve, records: make(chan Record, queueCapacity), commands: make(chan persistenceCommand),
		done: make(chan struct{}), counters: counters,
		beforeWrite: options.beforeWrite,
	}
	committed, truncated, err := recoverPersistentOrdinal(directory)
	if err != nil {
		return nil, err
	}
	if committed > 0 {
		w.committed.Store(committed)
		w.counters.persistenceRecovered.Add(1)
	}
	if truncated {
		w.counters.persistenceTailTruncated.Add(1)
	}
	if err := w.openNextFile(); err != nil {
		return nil, err
	}
	go w.run()
	return w, nil
}

func (w *persistentWriter) Enqueue(record Record) {
	if w == nil {
		return
	}
	if w.closed.Load() {
		w.counters.persistenceDropped.Add(1)
		return
	}
	terminal := isTerminalRecord(record)
	if !terminal && len(w.records) >= cap(w.records)-w.terminalReserve {
		w.counters.persistenceQueueFull.Add(1)
		w.counters.persistenceDropped.Add(1)
		return
	}
	select {
	case w.records <- record:
		w.counters.persistenceEnqueued.Add(1)
	default:
		w.counters.persistenceQueueFull.Add(1)
		w.counters.persistenceDropped.Add(1)
	}
}

func (w *persistentWriter) run() {
	defer close(w.done)
	for {
		select {
		case record := <-w.records:
			w.writeRecord(record)
		case command := <-w.commands:
			w.drain()
			err := w.sync()
			if command.close {
				err = errors.Join(err, w.closeFile())
				w.closed.Store(true)
			}
			if err == nil {
				w.counters.persistenceFlushed.Add(1)
			}
			command.done <- persistenceResult{ordinal: w.committed.Load(), err: err}
			if command.close {
				return
			}
		}
	}
}

func (w *persistentWriter) drain() {
	for {
		select {
		case record := <-w.records:
			w.writeRecord(record)
		default:
			return
		}
	}
}

func (w *persistentWriter) writeRecord(record Record) {
	if w.beforeWrite != nil {
		w.beforeWrite()
	}
	payload, err := json.Marshal(record)
	if err == nil && w.fileSize > 0 && w.fileSize+int64(len(payload))+1 > w.maxFileBytes {
		err = w.rotate()
	}
	if err == nil {
		_, err = w.buffer.Write(payload)
	}
	if err == nil {
		err = w.buffer.WriteByte('\n')
	}
	if err != nil {
		w.counters.persistenceWriteFailed.Add(1)
		return
	}
	w.fileSize += int64(len(payload)) + 1
	w.committed.Store(record.Ordinal)
	w.counters.persistenceWritten.Add(1)
}

func (w *persistentWriter) Flush(ctx context.Context) (uint64, error) {
	return w.command(ctx, false)
}

func (w *persistentWriter) Close(ctx context.Context) error {
	var resultErr error
	w.closeOnce.Do(func() {
		_, resultErr = w.command(ctx, true)
	})
	return resultErr
}

func (w *persistentWriter) command(ctx context.Context, closeWriter bool) (uint64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	done := make(chan persistenceResult, 1)
	select {
	case w.commands <- persistenceCommand{close: closeWriter, done: done}:
	case <-w.done:
		return w.committed.Load(), nil
	case <-ctx.Done():
		return w.committed.Load(), ctx.Err()
	}
	select {
	case result := <-done:
		return result.ordinal, result.err
	case <-ctx.Done():
		return w.committed.Load(), ctx.Err()
	}
}

func (w *persistentWriter) CommittedOrdinal() uint64 {
	if w == nil {
		return 0
	}
	return w.committed.Load()
}

func (w *persistentWriter) sync() error {
	if w.buffer == nil || w.file == nil {
		return nil
	}
	if err := w.buffer.Flush(); err != nil {
		w.counters.persistenceWriteFailed.Add(1)
		return err
	}
	if err := w.file.Sync(); err != nil {
		w.counters.persistenceWriteFailed.Add(1)
		return err
	}
	return nil
}

func (w *persistentWriter) rotate() error {
	if err := w.closeFile(); err != nil {
		return err
	}
	w.counters.persistenceRotated.Add(1)
	return w.openNextFile()
}

func (w *persistentWriter) closeFile() error {
	if w.file == nil {
		return nil
	}
	err := w.sync()
	closeErr := w.file.Close()
	w.file, w.buffer = nil, nil
	return errors.Join(err, closeErr)
}

func (w *persistentWriter) openNextFile() error {
	sequences, err := evidenceFileSequences(w.directory)
	if err != nil {
		return err
	}
	if len(sequences) > 0 {
		w.sequence = sequences[len(sequences)-1]
	}
	w.sequence++
	path := filepath.Join(w.directory, evidenceFileName(w.sequence))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open operational evidence file: %w", err)
	}
	w.file, w.buffer, w.fileSize = file, bufio.NewWriterSize(file, 64<<10), 0
	sequences = append(sequences, w.sequence)
	for len(sequences) > w.maxFiles {
		oldest := sequences[0]
		sequences = sequences[1:]
		if err := os.Remove(filepath.Join(w.directory, evidenceFileName(oldest))); err != nil && !errors.Is(err, os.ErrNotExist) {
			w.counters.persistenceWriteFailed.Add(1)
		}
	}
	return nil
}

func evidenceFileSequences(directory string) ([]int, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}
	sequences := make([]int, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "events-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		value := strings.TrimSuffix(strings.TrimPrefix(name, "events-"), ".jsonl")
		sequence, err := strconv.Atoi(value)
		if err == nil && sequence > 0 {
			sequences = append(sequences, sequence)
		}
	}
	sort.Ints(sequences)
	return sequences, nil
}

func evidenceFileName(sequence int) string { return fmt.Sprintf("events-%06d.jsonl", sequence) }

func recoverPersistentOrdinal(directory string) (uint64, bool, error) {
	sequences, err := evidenceFileSequences(directory)
	if err != nil {
		return 0, false, err
	}
	var highest uint64
	truncated := false
	for _, sequence := range sequences {
		payload, err := os.ReadFile(filepath.Join(directory, evidenceFileName(sequence)))
		if err != nil {
			return 0, truncated, err
		}
		for _, line := range strings.Split(string(payload), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var record Record
			if err := json.Unmarshal([]byte(line), &record); err != nil {
				truncated = true
				continue
			}
			if record.Ordinal > highest {
				highest = record.Ordinal
			}
		}
	}
	return highest, truncated, nil
}

func isTerminalRecord(record Record) bool {
	kind := string(record.Kind)
	for _, suffix := range []string{".completed", ".failed", ".cancelled", ".closed", ".resolved", ".rejected", ".committed", ".exited", ".recovery_required", ".interrupted", ".usage_missing"} {
		if strings.HasSuffix(kind, suffix) {
			return true
		}
	}
	return false
}
