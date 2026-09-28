package expert

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	SourceSession = "session"
	SourceRequest = "request"
	SourceConfig  = "config"
	SourceNone    = "none"

	// ExplicitNoneID is an internal override used when a resumed legacy session
	// has no expert sidecar. It intentionally cannot pass the public expert id
	// validator, so it cannot collide with a real catalog entry.
	ExplicitNoneID = "__none__"
)

type Selection struct {
	ID       string
	Name     string
	Source   string
	Selected bool
	Expert   Expert
}

type State struct {
	SelectedID         string `json:"selectedId"`
	SelectedName       string `json:"selectedName"`
	Source             string `json:"source"`
	CanSwitch          bool   `json:"canSwitch"`
	RequiresNewSession bool   `json:"requiresNewSession"`
}

func ResolveSelection(catalog *Catalog, sessionID, requestID, configID string) (Selection, error) {
	if strings.TrimSpace(sessionID) == ExplicitNoneID {
		return Selection{Source: SourceNone}, nil
	}
	candidates := []struct {
		source string
		id     string
	}{
		{SourceSession, sessionID},
		{SourceRequest, requestID},
		{SourceConfig, configID},
	}
	for _, candidate := range candidates {
		id := strings.TrimSpace(candidate.id)
		if id == "" {
			continue
		}
		ex, err := catalog.Resolve(id)
		if err != nil {
			return Selection{}, err
		}
		return Selection{ID: ex.ID, Name: ex.Name, Source: candidate.source, Selected: true, Expert: ex}, nil
	}
	return Selection{Source: SourceNone}, nil
}

func (s Selection) State(canSwitch bool) State {
	state := State{
		Source:             s.Source,
		CanSwitch:          canSwitch,
		RequiresNewSession: !canSwitch,
	}
	if state.Source == "" {
		state.Source = SourceNone
	}
	if s.Selected {
		state.SelectedID = s.ID
		state.SelectedName = s.Name
	}
	return state
}

func (s *Store) database(ctx context.Context) (*sql.DB, func(), error) {
	if !s.persistent {
		db, err := s.open()
		if err != nil {
			return nil, func() {}, err
		}
		if err := s.ensureInitialized(ctx, db); err != nil {
			db.Close()
			return nil, func() {}, err
		}
		return db, func() { _ = db.Close() }, nil
	}

	s.dbMu.Lock()
	defer s.dbMu.Unlock()
	if s.db != nil {
		return s.db, func() {}, nil
	}
	db, err := s.open()
	if err != nil {
		return nil, func() {}, err
	}
	if err := s.ensureInitialized(ctx, db); err != nil {
		db.Close()
		return nil, func() {}, err
	}
	s.db = db
	return db, func() {}, nil
}

func (s *Store) open() (*sql.DB, error) {
	if s.path == "" {
		return nil, fmt.Errorf("expert store path is required")
	}
	if dir := filepath.Dir(s.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create expert store dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", s.path)
	if err != nil {
		return nil, fmt.Errorf("open expert store: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		db.Close()
		return nil, fmt.Errorf("configure expert store: %w", err)
	}
	return db, nil
}

func (s *Store) ensureInitialized(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db)
	if err != nil {
		return err
	}
	if !exists {
		return initialize(ctx, db, s.builtins)
	}
	if err := ensureSchemaColumns(ctx, db); err != nil {
		return err
	}
	count, err := tableRowCount(ctx, db)
	if err != nil {
		return err
	}
	if count == 0 {
		if err := seedExistingTable(ctx, db, s.builtins); err != nil {
			return err
		}
	}
	if err := syncBuiltinDefinitions(ctx, db, s.builtins); err != nil {
		return err
	}
	return migrateStoredUserData(ctx, db)
}
