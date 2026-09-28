package agent

import (
	"os"
	"path/filepath"
	"strings"
)

func legacyEventNames(entries []os.DirEntry) map[string]bool {
	hasEvents := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && strings.HasSuffix(name, ".events.jsonl") {
			hasEvents[strings.TrimSuffix(name, ".events.jsonl")] = true
		}
	}
	return hasEvents
}

func importPreferredLegacyJSONL(srcDir, dest, base string, eventsInfo os.FileInfo) bool {
	jsonlPath := filepath.Join(srcDir, base+".jsonl")
	jsonlInfo, err := os.Stat(jsonlPath)
	if err != nil || !isMessageFormat(jsonlPath) {
		return false
	}
	if eventsInfo != nil && jsonlInfo.ModTime().Before(eventsInfo.ModTime()) {
		return false
	}
	if err := transformAndCopyJsonl(jsonlPath, dest); err != nil {
		return false
	}
	preserveLegacyModTime(dest, eventsInfo)
	return true
}

func importLegacyJSONLBackups(entries []os.DirEntry, srcDir, globalDest string, hasEvents map[string]bool, projectDir func(string) string) int {
	imported := 0
	for _, entry := range entries {
		if importLegacyJSONLBackup(entry, srcDir, globalDest, hasEvents, projectDir) {
			imported++
		}
	}
	return imported
}
