// Package temporal provides the process-local, deterministic source of current
// time used by model context, tools, and local commands. It never infers time or
// timezone from network location.
package temporal

import (
	"fmt"
	"strings"
	"sync"
	"time"
	_ "time/tzdata"
)

// Clock makes time-dependent behavior deterministic in tests.
type Clock interface {
	Now() time.Time
}

// ClockFunc adapts a function to Clock.
type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time { return f() }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// Source describes which timezone candidate won resolution.
type Source string

const (
	SourceOverride Source = "override"
	SourceConfig   Source = "config"
	SourceLocal    Source = "local"
	SourceUTC      Source = "utc"
)

// Options configures a Service. Override is supplied by a host/client and wins
// over Configured. Local and Clock are injectable for tests.
type Options struct {
	Override   string
	Configured string
	Local      *time.Location
	Clock      Clock
	Warn       func(string)
}

// Snapshot is one internally consistent reading of the clock.
type Snapshot struct {
	Local       time.Time
	UTC         time.Time
	Date        string
	ClockTime   string
	Weekday     time.Weekday
	WeekdayISO  int
	Timezone    string
	UTCOffset   string
	UnixSeconds int64
}

// ContextBlock returns compact date-level facts for a new model turn. Exact
// wall-clock time stays behind current_datetime.
func (s Snapshot) ContextBlock() string {
	return fmt.Sprintf(
		`<temporal_context date=%q weekday=%q weekday_iso=%q timezone=%q utc_offset=%q/>`,
		s.Date,
		s.Weekday.String(),
		fmt.Sprintf("%d", s.WeekdayISO),
		s.Timezone,
		s.UTCOffset,
	)
}

// Service resolves a user timezone once and converts every clock reading through
// it. SetOverride may update the client/session timezone without rebuilding the
// model session.
type Service struct {
	mu         sync.RWMutex
	clock      Clock
	configured string
	local      *time.Location
	warn       func(string)
	location   *time.Location
	name       string
	source     Source
}

// New constructs a Service using override > configured > local > UTC.
func New(opts Options) *Service {
	clock := opts.Clock
	if clock == nil {
		clock = systemClock{}
	}
	local := opts.Local
	if local == nil {
		local = time.Local
	}
	s := &Service{clock: clock, configured: strings.TrimSpace(opts.Configured), local: local, warn: opts.Warn}
	s.resolve(strings.TrimSpace(opts.Override))
	return s
}

// SetOverride replaces the client/session timezone. An empty value removes the
// override and falls back to configured/local resolution. Invalid values leave
// the current resolution unchanged.
func (s *Service) SetOverride(name string) error {
	name = strings.TrimSpace(name)
	if name != "" {
		if _, err := time.LoadLocation(name); err != nil {
			return fmt.Errorf("invalid timezone %q: %w", name, err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolveLocked(name)
	return nil
}

// Timezone returns the resolved timezone label and its source.
func (s *Service) Timezone() (string, Source) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name, s.source
}

// Snapshot reads the clock once so every returned field describes the same
// instant.
func (s *Service) Snapshot() Snapshot {
	s.mu.RLock()
	clock, loc, name := s.clock, s.location, s.name
	s.mu.RUnlock()
	now := clock.Now().In(loc)
	_, offset := now.Zone()
	return Snapshot{
		Local:       now,
		UTC:         now.UTC(),
		Date:        now.Format("2006-01-02"),
		ClockTime:   now.Format("15:04:05"),
		Weekday:     now.Weekday(),
		WeekdayISO:  weekdayISO(now.Weekday()),
		Timezone:    name,
		UTCOffset:   formatOffset(offset),
		UnixSeconds: now.Unix(),
	}
}

func (s *Service) resolve(override string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resolveLocked(override)
}

func (s *Service) resolveLocked(override string) {
	for _, candidate := range []struct {
		name   string
		source Source
	}{
		{strings.TrimSpace(override), SourceOverride},
		{s.configured, SourceConfig},
	} {
		if candidate.name == "" {
			continue
		}
		loc, err := time.LoadLocation(candidate.name)
		if err == nil {
			s.location, s.name, s.source = loc, candidate.name, candidate.source
			return
		}
		if s.warn != nil {
			s.warn(fmt.Sprintf("invalid timezone %q; falling back", candidate.name))
		}
	}
	if s.local != nil {
		name := strings.TrimSpace(s.local.String())
		if name == "" {
			name = "Local"
		}
		s.location, s.name, s.source = s.local, name, SourceLocal
		return
	}
	s.location, s.name, s.source = time.UTC, "UTC", SourceUTC
}

func weekdayISO(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func formatOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d:%02d", sign, seconds/3600, seconds%3600/60)
}
