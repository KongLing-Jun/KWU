// Package transcript persists the UI event stream independently of model history.
package transcript

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct{ Path string }

type Entry struct {
	Sequence int64           `json:"sequence"`
	Payload  json.RawMessage `json:"payload"`
}

type Page struct {
	JournalID string          `json:"journalId"`
	Baseline  json.RawMessage `json:"baseline,omitempty"`
	Through   int64           `json:"through"`
	Next      int64           `json:"next"`
	HasMore   bool            `json:"hasMore"`
	Entries   []Entry         `json:"entries"`
}

func (s Store) open() (*sql.DB, error) {
	if s.Path == "" {
		return nil, errors.New("empty transcript database path")
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return nil, err
	}
	path := filepath.ToSlash(s.Path)
	if filepath.VolumeName(s.Path) != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	u := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", u.String()+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS transcripts (session TEXT PRIMARY KEY, id TEXT NOT NULL UNIQUE, baseline BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS transcript_events (
			seq INTEGER PRIMARY KEY AUTOINCREMENT,
			journal_id TEXT NOT NULL REFERENCES transcripts(id) ON DELETE CASCADE,
			event_id TEXT NOT NULL, payload BLOB NOT NULL,
			UNIQUE(journal_id, event_id))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			db.Close()
			return nil, err
		}
	}
	return db, nil
}

// Begin keeps the first baseline. Subsequent turns append to the same journal,
// so display-only intermediate output remains available after a later turn.
func (s Store) Begin(session, id string, baseline json.RawMessage) error {
	if session == "" || id == "" || !json.Valid(baseline) {
		return errors.New("invalid transcript identity or baseline")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	// Only an existing session is an idempotent begin. An ID owned by another
	// session must fail instead of silently leaving this session without a journal.
	_, err = db.Exec(`INSERT INTO transcripts(session, id, baseline) VALUES (?, ?, ?)
		ON CONFLICT(session) DO NOTHING`, session, id, []byte(baseline))
	return err
}

// Append commits before returning its delivery cursor. SQLite assigns the order
// atomically; event identity makes retries idempotent without application locks.
func (s Store) Append(session, eventID string, payload json.RawMessage) (string, int64, error) {
	if eventID == "" || !json.Valid(payload) {
		return "", 0, errors.New("invalid transcript event")
	}
	db, err := s.open()
	if err != nil {
		return "", 0, err
	}
	defer db.Close()
	var id string
	var seq int64
	// Return the cursor in the same statement that persists the event. A reset
	// between a separate INSERT and SELECT could otherwise lose its identity.
	// The conflict update preserves the original payload and delivery cursor.
	err = db.QueryRow(`INSERT INTO transcript_events(journal_id, event_id, payload)
		SELECT id, ?, ? FROM transcripts WHERE session = ?
		ON CONFLICT(journal_id, event_id) DO UPDATE SET event_id = excluded.event_id
		RETURNING journal_id, seq`, eventID, []byte(payload), session).Scan(&id, &seq)
	return id, seq, err
}

// Read uses a fixed upper cursor across pages. Appends during recovery are
// delivered live and merged by the frontend after the snapshot is installed.
// A zero through cursor starts a new snapshot, including incremental recovery
// after an already displayed sequence; only initial recovery reads the baseline.
func (s Store) Read(session, journalID string, after, through int64) (*Page, error) {
	if _, err := os.Stat(s.Path); errors.Is(err, os.ErrNotExist) {
		if journalID != "" {
			return nil, errors.New("transcript changed during recovery")
		}
		return nil, nil
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var page Page
	err = tx.QueryRow(`SELECT id FROM transcripts WHERE session = ?`, session).Scan(&page.JournalID)
	if errors.Is(err, sql.ErrNoRows) {
		if journalID != "" {
			return nil, errors.New("transcript changed during recovery")
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if journalID != "" && journalID != page.JournalID {
		return nil, errors.New("transcript changed during recovery")
	}
	if after == 0 {
		var baseline []byte
		if err := tx.QueryRow(`SELECT baseline FROM transcripts WHERE session = ?`, session).Scan(&baseline); err != nil {
			return nil, err
		}
		page.Baseline = baseline
	}
	if through == 0 {
		if err := tx.QueryRow(`SELECT COALESCE(MAX(seq), 0) FROM transcript_events WHERE journal_id = ?`, page.JournalID).Scan(&through); err != nil {
			return nil, err
		}
	}
	page.Through, page.Next = through, after
	rows, err := tx.Query(`SELECT seq, payload FROM transcript_events WHERE journal_id = ? AND seq > ? AND seq <= ? ORDER BY seq LIMIT 501`, page.JournalID, after, through)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page.Entries = []Entry{}
	for rows.Next() {
		if len(page.Entries) == 500 {
			page.HasMore = true
			break
		}
		var entry Entry
		if err := rows.Scan(&entry.Sequence, &entry.Payload); err != nil {
			return nil, err
		}
		page.Entries = append(page.Entries, entry)
		page.Next = entry.Sequence
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &page, nil
}

func (s Store) Reset(session string) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`DELETE FROM transcripts WHERE session = ?`, session); err != nil {
		return fmt.Errorf("reset transcript: %w", err)
	}
	return nil
}
