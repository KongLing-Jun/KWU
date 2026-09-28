package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"kwu/internal/config"
	"kwu/internal/fileutil"
)

const (
	heartbeatMigrationVersion        = "heartbeat-v1"
	heartbeatMigrationDirName        = "migrations"
	heartbeatMigrationMarkerFileName = "heartbeat-v1.json"

	HeartbeatMigrationStatusNotFound        HeartbeatMigrationStatus = "not_found"
	HeartbeatMigrationStatusMigrated        HeartbeatMigrationStatus = "migrated"
	HeartbeatMigrationStatusAlreadyMigrated HeartbeatMigrationStatus = "already_migrated"
	HeartbeatMigrationStatusSourceChanged   HeartbeatMigrationStatus = "source_changed"
	HeartbeatMigrationStatusMarkerFailed    HeartbeatMigrationStatus = "marker_failed"
	HeartbeatMigrationStatusFailed          HeartbeatMigrationStatus = "failed"
)

type HeartbeatMigrationStatus string

type HeartbeatMigrationRunResult struct {
	Status       HeartbeatMigrationStatus        `json:"status"`
	Notice       string                          `json:"notice,omitempty"`
	SourcePath   string                          `json:"sourcePath,omitempty"`
	SourceHash   string                          `json:"sourceHash,omitempty"`
	TaskIDs      map[string]string               `json:"taskIds,omitempty"`
	Skipped      []HeartbeatMigrationSkippedTask `json:"skipped,omitempty"`
	ErrorSummary string                          `json:"errorSummary,omitempty"`
	Marker       *HeartbeatMigrationMarker       `json:"marker,omitempty"`
}

type HeartbeatMigrationMarker struct {
	Version      string                          `json:"version"`
	Status       HeartbeatMigrationStatus        `json:"status,omitempty"`
	SourcePath   string                          `json:"sourcePath"`
	SourceHash   string                          `json:"sourceHash"`
	MigratedAt   time.Time                       `json:"migratedAt"`
	TaskIDs      map[string]string               `json:"taskIds"`
	Skipped      []HeartbeatMigrationSkippedTask `json:"skipped,omitempty"`
	ErrorSummary string                          `json:"errorSummary"`
}

func MigrateHeartbeatIfNeeded(ctx context.Context, store *Store) (HeartbeatMigrationRunResult, error) {
	if err := ctx.Err(); err != nil {
		return heartbeatMigrationFailed("", "", err), err
	}
	if store == nil {
		err := errors.New("automation: migration store is required")
		return heartbeatMigrationFailed("", "", err), err
	}

	sourcePath := filepath.Join(config.MemoryUserDir(), legacyHeartbeatFileName)
	sourceBytes, err := os.ReadFile(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return HeartbeatMigrationRunResult{Status: HeartbeatMigrationStatusNotFound, SourcePath: sourcePath}, nil
	}
	if err != nil {
		return heartbeatMigrationFailed(sourcePath, "", err), err
	}
	sourceHash := heartbeatMigrationHash(sourceBytes)

	if marker, err := readHeartbeatMigrationMarker(store); err == nil {
		if marker.SourceHash == sourceHash {
			if marker.Status == HeartbeatMigrationStatusFailed {
				return HeartbeatMigrationRunResult{
					Status:       HeartbeatMigrationStatusFailed,
					SourcePath:   sourcePath,
					SourceHash:   sourceHash,
					TaskIDs:      marker.TaskIDs,
					Skipped:      marker.Skipped,
					ErrorSummary: marker.ErrorSummary,
					Marker:       &marker,
				}, nil
			}
			return HeartbeatMigrationRunResult{
				Status:       HeartbeatMigrationStatusAlreadyMigrated,
				SourcePath:   sourcePath,
				SourceHash:   sourceHash,
				TaskIDs:      marker.TaskIDs,
				Skipped:      marker.Skipped,
				ErrorSummary: marker.ErrorSummary,
				Marker:       &marker,
			}, nil
		}
		return HeartbeatMigrationRunResult{
			Status:       HeartbeatMigrationStatusSourceChanged,
			Notice:       "legacy heartbeat file changed after migration; manage tasks in the automation task list",
			SourcePath:   sourcePath,
			SourceHash:   sourceHash,
			TaskIDs:      marker.TaskIDs,
			Skipped:      marker.Skipped,
			ErrorSummary: marker.ErrorSummary,
			Marker:       &marker,
		}, nil
	}

	converted, err := decodeLegacyHeartbeatMigration(sourcePath, sourceBytes)
	if err != nil {
		return heartbeatMigrationFailed(sourcePath, sourceHash, err), err
	}
	if len(converted.Tasks) == 0 && len(converted.Skipped) > 0 {
		summary := heartbeatMigrationSkippedSummary(converted.Skipped)
		marker := HeartbeatMigrationMarker{
			Version:      heartbeatMigrationVersion,
			Status:       HeartbeatMigrationStatusFailed,
			SourcePath:   sourcePath,
			SourceHash:   sourceHash,
			MigratedAt:   time.Now().UTC(),
			TaskIDs:      map[string]string{},
			Skipped:      converted.Skipped,
			ErrorSummary: summary,
		}
		_ = writeHeartbeatMigrationMarker(store, marker)
		return HeartbeatMigrationRunResult{
			Status:       HeartbeatMigrationStatusFailed,
			SourcePath:   sourcePath,
			SourceHash:   sourceHash,
			Skipped:      converted.Skipped,
			ErrorSummary: summary,
			Marker:       &marker,
		}, nil
	}
	existing, err := store.LoadTasks()
	if err != nil {
		return heartbeatMigrationFailed(sourcePath, sourceHash, err), err
	}
	tasks, taskIDs := mergeHeartbeatMigrationTasks(existing, converted)
	if err := ctx.Err(); err != nil {
		return heartbeatMigrationFailed(sourcePath, sourceHash, err), err
	}
	if err := store.SaveTasks(tasks); err != nil {
		return heartbeatMigrationFailed(sourcePath, sourceHash, err), err
	}

	marker := HeartbeatMigrationMarker{
		Version:      heartbeatMigrationVersion,
		Status:       HeartbeatMigrationStatusMigrated,
		SourcePath:   sourcePath,
		SourceHash:   sourceHash,
		MigratedAt:   time.Now().UTC(),
		TaskIDs:      taskIDs,
		Skipped:      converted.Skipped,
		ErrorSummary: heartbeatMigrationSkippedSummary(converted.Skipped),
	}
	if err := writeHeartbeatMigrationMarker(store, marker); err != nil {
		result := HeartbeatMigrationRunResult{
			Status:       HeartbeatMigrationStatusMarkerFailed,
			SourcePath:   sourcePath,
			SourceHash:   sourceHash,
			TaskIDs:      taskIDs,
			Skipped:      converted.Skipped,
			ErrorSummary: err.Error(),
			Marker:       &marker,
		}
		return result, nil
	}
	return HeartbeatMigrationRunResult{
		Status:       HeartbeatMigrationStatusMigrated,
		SourcePath:   sourcePath,
		SourceHash:   sourceHash,
		TaskIDs:      taskIDs,
		Skipped:      converted.Skipped,
		ErrorSummary: marker.ErrorSummary,
		Marker:       &marker,
	}, nil
}

func (s *service) MigrateHeartbeatIfNeeded(ctx context.Context) (HeartbeatMigrationRunResult, error) {
	result, err := MigrateHeartbeatIfNeeded(ctx, s.store)
	s.migrationMu.Lock()
	s.heartbeatMigration = result
	s.migrationMu.Unlock()
	return result, err
}

func (s *service) MigrationStatus(ctx context.Context) (HeartbeatMigrationRunResult, error) {
	if err := ctx.Err(); err != nil {
		return HeartbeatMigrationRunResult{}, err
	}
	s.migrationMu.Lock()
	defer s.migrationMu.Unlock()
	if s.heartbeatMigration.Status == "" {
		return HeartbeatMigrationRunResult{Status: HeartbeatMigrationStatusNotFound}, nil
	}
	return s.heartbeatMigration, nil
}

func mergeHeartbeatMigrationTasks(existing []Task, converted HeartbeatMigrationResult) ([]Task, map[string]string) {
	out := append([]Task(nil), existing...)
	seen := make(map[string]bool, len(existing)+len(converted.Tasks))
	for _, task := range existing {
		seen[task.ID] = true
	}
	taskIDs := make(map[string]string, len(converted.Tasks))
	for i, task := range converted.Tasks {
		legacyID := task.ID
		if i < len(converted.Metadata) && converted.Metadata[i].LegacyID != "" {
			legacyID = converted.Metadata[i].LegacyID
		}
		taskIDs[legacyID] = task.ID
		if seen[task.ID] {
			continue
		}
		seen[task.ID] = true
		out = append(out, task)
	}
	return out, taskIDs
}

func readHeartbeatMigrationMarker(store *Store) (HeartbeatMigrationMarker, error) {
	b, err := os.ReadFile(heartbeatMigrationMarkerPath(store))
	if err != nil {
		return HeartbeatMigrationMarker{}, err
	}
	var marker HeartbeatMigrationMarker
	if err := json.Unmarshal(b, &marker); err != nil {
		return HeartbeatMigrationMarker{}, fmt.Errorf("read heartbeat migration marker: %w", err)
	}
	return marker, nil
}

func writeHeartbeatMigrationMarker(store *Store, marker HeartbeatMigrationMarker) error {
	b, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return fmt.Errorf("write heartbeat migration marker: encode: %w", err)
	}
	b = append(b, '\n')
	if err := fileutil.AtomicWriteFile(heartbeatMigrationMarkerPath(store), b, 0o600); err != nil {
		return fmt.Errorf("write heartbeat migration marker: %w", err)
	}
	return nil
}

func heartbeatMigrationMarkerPath(store *Store) string {
	return filepath.Join(store.dir, heartbeatMigrationDirName, heartbeatMigrationMarkerFileName)
}

func heartbeatMigrationHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func heartbeatMigrationFailed(sourcePath, sourceHash string, err error) HeartbeatMigrationRunResult {
	return HeartbeatMigrationRunResult{
		Status:       HeartbeatMigrationStatusFailed,
		SourcePath:   sourcePath,
		SourceHash:   sourceHash,
		ErrorSummary: err.Error(),
	}
}

func heartbeatMigrationSkippedSummary(skipped []HeartbeatMigrationSkippedTask) string {
	if len(skipped) == 0 {
		return ""
	}
	if len(skipped) == 1 {
		return "skipped 1 legacy heartbeat task: " + skipped[0].Reason
	}
	return fmt.Sprintf("skipped %d legacy heartbeat tasks", len(skipped))
}
