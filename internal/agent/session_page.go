package agent

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"kwu/internal/provider"
)

var (
	// ErrSessionGenerationChanged tells a paged reader to discard its cursor and
	// restart from the tail because the transcript was rewritten.
	ErrSessionGenerationChanged = errors.New("session generation changed")
	// ErrSessionPageUnsupported identifies legacy event tapes that require their
	// compatibility projector instead of provider-message JSONL paging.
	ErrSessionPageUnsupported = errors.New("session format does not support direct paging")
)

// LoadSessionPage reads at most one bounded provider-message window directly
// from the JSONL tail. before is a byte offset returned by the previous page;
// generation prevents a cursor from being reused after compaction or rewrite.
func LoadSessionPage(path string, before, limit int, expectedGeneration string) (messages []provider.Message, start int, hasMore bool, generation string, err error) {
	if limit <= 0 {
		limit = 200
	}
	f, info, generation, err := openSessionPage(path, expectedGeneration)
	if err != nil {
		return nil, 0, false, generation, err
	}
	defer f.Close()
	end := before
	if end <= 0 || int64(end) > info.Size() {
		end = int(info.Size())
	}
	reversed, oldestStart, err := readSessionMessagesBackward(f, path, int64(end), limit)
	if err != nil {
		return nil, 0, false, generation, err
	}
	reverseMessages(reversed)
	currentGeneration, err := validateSessionPageGeneration(path, generation)
	if err != nil {
		return nil, 0, false, currentGeneration, err
	}
	return reversed, int(oldestStart), oldestStart > 0, generation, nil
}

func openSessionPage(path, expectedGeneration string) (*os.File, os.FileInfo, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, "", err
	}
	generation := sessionFileGeneration(info)
	if expectedGeneration != "" && expectedGeneration != generation {
		f.Close()
		return nil, nil, generation, ErrSessionGenerationChanged
	}
	return f, info, generation, nil
}

func readSessionMessagesBackward(file *os.File, path string, cursor int64, limit int) ([]provider.Message, int64, error) {
	reversed := make([]provider.Message, 0, limit)
	oldestStart := cursor
	for cursor > 0 {
		line, lineStart, ok, err := readPreviousJSONLine(file, cursor)
		if err != nil {
			return nil, 0, err
		}
		if !ok {
			break
		}
		cursor = lineStart
		message, empty, err := decodePagedSessionMessage(path, line)
		if err != nil {
			return nil, 0, err
		}
		if empty {
			continue
		}
		reversed = append(reversed, message)
		oldestStart = lineStart
		if len(reversed) >= limit && message.Role != provider.RoleTool {
			break
		}
	}
	return reversed, oldestStart, nil
}

func decodePagedSessionMessage(path string, line []byte) (provider.Message, bool, error) {
	if len(bytes.TrimSpace(line)) == 0 {
		return provider.Message{}, true, nil
	}
	var envelope struct {
		Kind string `json:"kind"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return provider.Message{}, false, fmt.Errorf("decode %s page: %w", path, err)
	}
	if envelope.Kind != "" || envelope.Type != "" {
		return provider.Message{}, false, ErrSessionPageUnsupported
	}
	var message provider.Message
	if err := json.Unmarshal(line, &message); err != nil {
		return provider.Message{}, false, fmt.Errorf("decode %s page: %w", path, err)
	}
	if !pagedSessionRole(message.Role) {
		return provider.Message{}, false, ErrSessionPageUnsupported
	}
	return message, false, nil
}

func reverseMessages(messages []provider.Message) {
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
}

func validateSessionPageGeneration(path, expected string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	current := sessionFileGeneration(info)
	if current != expected {
		return current, ErrSessionGenerationChanged
	}
	return current, nil
}

func sessionFileGeneration(info os.FileInfo) string {
	return fmt.Sprintf("%d:%d", info.Size(), info.ModTime().UnixNano())
}

func pagedSessionRole(role provider.Role) bool {
	switch role {
	case provider.RoleSystem, provider.RoleUser, provider.RoleAssistant, provider.RoleTool:
		return true
	default:
		return false
	}
}

func readPreviousJSONLine(file *os.File, end int64) ([]byte, int64, bool, error) {
	end, err := trimTrailingLineEnd(file, end)
	if err != nil {
		return nil, 0, false, err
	}
	if end == 0 {
		return nil, 0, false, nil
	}

	const blockSize = int64(64 * 1024)
	position := end
	var reverseChunks [][]byte
	for position > 0 {
		start := position - blockSize
		if start < 0 {
			start = 0
		}
		block := make([]byte, position-start)
		if _, err := file.ReadAt(block, start); err != nil && !errors.Is(err, io.EOF) {
			return nil, 0, false, err
		}
		if newline := bytes.LastIndexByte(block, '\n'); newline >= 0 {
			lineStart := start + int64(newline) + 1
			line := joinReverseChunks(block[newline+1:], reverseChunks)
			return bytes.TrimSuffix(line, []byte{'\r'}), lineStart, true, nil
		}
		reverseChunks = append(reverseChunks, block)
		position = start
	}
	return bytes.TrimSuffix(joinReverseChunks(nil, reverseChunks), []byte{'\r'}), 0, true, nil
}

func trimTrailingLineEnd(file *os.File, end int64) (int64, error) {
	for end > 0 {
		var trailing [1]byte
		if _, err := file.ReadAt(trailing[:], end-1); err != nil {
			return 0, err
		}
		if trailing[0] != '\n' && trailing[0] != '\r' {
			break
		}
		end--
	}
	return end, nil
}

func joinReverseChunks(prefix []byte, reverseChunks [][]byte) []byte {
	line := append([]byte(nil), prefix...)
	for index := len(reverseChunks) - 1; index >= 0; index-- {
		line = append(line, reverseChunks[index]...)
	}
	return line
}
