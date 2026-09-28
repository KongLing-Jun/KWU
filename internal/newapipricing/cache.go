package newapipricing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"kwu/internal/config"
	"kwu/internal/fileutil"
	"kwu/internal/modelbilling"
)

const (
	CacheFormatVersion = 1
	defaultFreshFor    = 12 * time.Hour
	defaultStaleFor    = 24 * time.Hour
	pricingDependency  = "NEWAPI-PRICING-01"
	statusDependency   = "NEWAPI-STATUS-01"
)

type SourceAuthenticity string

const (
	SourceReady    SourceAuthenticity = "READY"
	SourceMockOnly SourceAuthenticity = "MOCK_ONLY"
)

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type SourceEvidence struct {
	DependencyID  string             `json:"dependency_id"`
	Authenticity  SourceAuthenticity `json:"authenticity"`
	SourceVersion *string            `json:"source_version,omitempty"`
	SourceHash    *string            `json:"source_hash,omitempty"`
	FetchedAt     time.Time          `json:"fetched_at"`
}

type SnapshotProvenance struct {
	Pricing SourceEvidence `json:"pricing"`
	Status  SourceEvidence `json:"status"`
}

type CachedSnapshot struct {
	Snapshot   modelbilling.PricingSnapshot `json:"snapshot"`
	Provenance SnapshotProvenance           `json:"provenance"`
}

type RefreshResult struct {
	CachedSnapshot CachedSnapshot
	Catalog        *Catalog
}

type RefreshRequest struct {
	Source       string
	Endpoint     string
	Currency     string
	Transport    Transport
	StatusSource StatusSource
}

type cacheFile struct {
	Version    int                          `json:"version"`
	Snapshot   modelbilling.PricingSnapshot `json:"snapshot"`
	Provenance SnapshotProvenance           `json:"provenance"`
}

type refreshPolicy struct {
	FreshFor time.Duration
	StaleFor time.Duration
}

type refreshFlight struct {
	done      chan struct{}
	cancel    context.CancelFunc
	accepting bool
	waiters   int
	result    RefreshResult
	err       error
}

type SnapshotCache struct {
	dir       string
	clock     Clock
	policy    refreshPolicy
	writeFile func(string, []byte, os.FileMode) error
	encode    func(cacheFile) ([]byte, error)

	mu      sync.Mutex
	entries map[string]cacheFile
	flights map[string]*refreshFlight

	onClosingFlightWait func()
}

var sourceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

func DefaultCacheDir() string {
	root := config.CacheDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, "pricing")
}

func NewDefaultCache() (*SnapshotCache, error) {
	dir := DefaultCacheDir()
	if dir == "" {
		return nil, contractError("pricing cache directory is unavailable")
	}
	return NewSnapshotCache(dir, nil)
}

func NewSnapshotCache(dir string, clock Clock) (*SnapshotCache, error) {
	return newSnapshotCache(dir, clock, refreshPolicy{FreshFor: defaultFreshFor, StaleFor: defaultStaleFor})
}

// DefaultPricingStateAt classifies fetched data with the shared default cache windows.
func DefaultPricingStateAt(fetchedAt, now time.Time) modelbilling.PricingState {
	return pricingStateAt(now, fetchedAt.Add(defaultFreshFor), fetchedAt.Add(defaultStaleFor))
}

func newSnapshotCache(dir string, clock Clock, policy refreshPolicy) (*SnapshotCache, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, contractError("pricing cache directory is required")
	}
	if policy.FreshFor <= 0 || policy.StaleFor <= policy.FreshFor {
		return nil, contractError("pricing cache refresh policy is invalid")
	}
	if clock == nil {
		clock = systemClock{}
	}
	return &SnapshotCache{
		dir: dir, clock: clock, policy: policy,
		writeFile: fileutil.AtomicWriteFileStrict, encode: encodeCacheFile,
		entries: make(map[string]cacheFile), flights: make(map[string]*refreshFlight),
	}, nil
}

func (c *SnapshotCache) Read(source string) (CachedSnapshot, error) {
	path, err := c.path(source)
	if err != nil {
		return CachedSnapshot{}, err
	}
	c.mu.Lock()
	record, ok := c.entries[source]
	if !ok {
		record, err = c.readFile(path, source)
		if err == nil {
			c.entries[source] = cloneCacheFile(record)
		}
	}
	c.mu.Unlock()
	if err != nil {
		return CachedSnapshot{}, unavailableError("read pricing cache for %s: %v", source, err)
	}
	return c.currentView(record), nil
}

func (c *SnapshotCache) Refresh(ctx context.Context, request RefreshRequest) (CachedSnapshot, error) {
	result, err := c.RefreshWithCatalog(ctx, request)
	return result.CachedSnapshot, err
}

func (c *SnapshotCache) RefreshWithCatalog(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	if err := ctx.Err(); err != nil {
		return RefreshResult{}, err
	}
	if err := validateRefreshRequest(request); err != nil {
		return RefreshResult{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return RefreshResult{}, err
		}
		flight, closing := c.joinOrStartRefresh(ctx, request)
		if !closing {
			return c.awaitRefresh(ctx, request.Source, flight)
		}
		if err := c.waitForClosingFlight(ctx, flight); err != nil {
			return RefreshResult{}, err
		}
	}
}

func (c *SnapshotCache) joinOrStartRefresh(ctx context.Context, request RefreshRequest) (*refreshFlight, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	flight := c.flights[request.Source]
	if flight != nil && !flight.accepting {
		return flight, true
	}
	if flight == nil {
		sharedContext, cancel := context.WithCancel(context.WithoutCancel(ctx))
		flight = &refreshFlight{done: make(chan struct{}), cancel: cancel, accepting: true}
		c.flights[request.Source] = flight
		go c.runRefresh(sharedContext, request, flight)
	}
	flight.waiters++
	return flight, false
}

func (c *SnapshotCache) awaitRefresh(ctx context.Context, source string, flight *refreshFlight) (RefreshResult, error) {
	select {
	case <-ctx.Done():
		c.releaseWaiter(source, flight)
		return RefreshResult{}, ctx.Err()
	case <-flight.done:
		c.releaseWaiter(source, flight)
		return cloneRefreshResult(flight.result), flight.err
	}
}

func (c *SnapshotCache) waitForClosingFlight(ctx context.Context, flight *refreshFlight) error {
	if c.onClosingFlightWait != nil {
		c.onClosingFlightWait()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-flight.done:
		return nil
	}
}

func (c *SnapshotCache) runRefresh(ctx context.Context, request RefreshRequest, flight *refreshFlight) {
	defer flight.cancel()
	result, err := c.refresh(ctx, request)
	c.mu.Lock()
	flight.result = cloneRefreshResult(result)
	flight.err = err
	delete(c.flights, request.Source)
	close(flight.done)
	c.mu.Unlock()
}

func (c *SnapshotCache) releaseWaiter(source string, flight *refreshFlight) {
	c.mu.Lock()
	flight.waiters--
	if flight.waiters == 0 && c.flights[source] == flight {
		flight.accepting = false
		flight.cancel()
	}
	c.mu.Unlock()
}

func (c *SnapshotCache) refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	record, catalog, err := c.fetch(ctx, request)
	if err == nil {
		err = ctx.Err()
		if err == nil {
			err = c.persist(record)
		}
	}
	if err == nil {
		return RefreshResult{CachedSnapshot: c.currentView(record), Catalog: &catalog}, nil
	}
	if fallback, readErr := c.Read(request.Source); readErr == nil {
		return RefreshResult{CachedSnapshot: fallback}, err
	}
	return RefreshResult{}, unavailableFrom(err)
}

func (c *SnapshotCache) fetch(ctx context.Context, request RefreshRequest) (cacheFile, Catalog, error) {
	raw, err := request.Transport.Fetch(ctx, request.Endpoint)
	if err != nil {
		return cacheFile{}, Catalog{}, err
	}
	status, err := LoadStatus(ctx, request.StatusSource)
	if err != nil {
		return cacheFile{}, Catalog{}, err
	}
	fetchedAt := c.clock.Now().UTC()
	provenance := buildProvenance(status, fetchedAt)
	header := SnapshotHeader{
		SnapshotID: "pending", Source: request.Source, SourceHash: "pending", Currency: request.Currency,
		ExchangeRate: &status.USDExchangeRate, RechargePrice: &status.RechargePrice,
		FetchedAt: fetchedAt, FreshUntil: fetchedAt.Add(c.policy.FreshFor),
		StaleUntil: fetchedAt.Add(c.policy.StaleFor), State: modelbilling.PricingFresh,
	}
	catalog, snapshot, err := (Adapter{}).Adapt(raw, header)
	if err != nil {
		return cacheFile{}, Catalog{}, err
	}
	provenance.Pricing.SourceVersion = cloneString(snapshot.SourceVersion)
	hash, err := ContentHash(snapshot, provenance)
	if err != nil {
		return cacheFile{}, Catalog{}, err
	}
	snapshot.SourceHash = hash
	snapshot.SnapshotID = BuildSnapshotID(snapshot.Source, hash)
	record := cacheFile{Version: CacheFormatVersion, Snapshot: snapshot, Provenance: provenance}
	if err := validateCacheFile(record, c.policy, request.Source); err != nil {
		return cacheFile{}, Catalog{}, err
	}
	return record, catalog, nil
}

func buildProvenance(status modelbilling.PricingSourceStatus, fetchedAt time.Time) SnapshotProvenance {
	return SnapshotProvenance{
		Pricing: SourceEvidence{DependencyID: pricingDependency, Authenticity: SourceReady, FetchedAt: fetchedAt},
		Status: SourceEvidence{
			DependencyID: statusDependency, Authenticity: SourceMockOnly,
			SourceVersion: cloneString(status.SourceVersion), SourceHash: cloneString(status.SourceHash),
			FetchedAt: status.FetchedAt,
		},
	}
}

func (c *SnapshotCache) persist(record cacheFile) error {
	if err := validateCacheFile(record, c.policy, record.Snapshot.Source); err != nil {
		return err
	}
	data, err := c.encode(record)
	if err != nil {
		return fmt.Errorf("encode pricing cache: %w", err)
	}
	decoded, err := decodeCacheFile(data)
	if err != nil {
		return fmt.Errorf("verify encoded pricing cache: %w", err)
	}
	if err := validateCacheFile(decoded, c.policy, record.Snapshot.Source); err != nil {
		return fmt.Errorf("verify encoded pricing cache: %w", err)
	}
	path, err := c.path(record.Snapshot.Source)
	if err != nil {
		return err
	}
	if err := c.writeFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write pricing cache: %w", err)
	}
	c.mu.Lock()
	c.entries[record.Snapshot.Source] = cloneCacheFile(record)
	c.mu.Unlock()
	return nil
}

func (c *SnapshotCache) readFile(path, source string) (cacheFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{}, err
	}
	record, err := decodeCacheFile(data)
	if err != nil {
		return cacheFile{}, err
	}
	if err := validateCacheFile(record, c.policy, source); err != nil {
		return cacheFile{}, err
	}
	return record, nil
}

func (c *SnapshotCache) currentView(record cacheFile) CachedSnapshot {
	value := cloneCachedSnapshot(CachedSnapshot{Snapshot: record.Snapshot, Provenance: record.Provenance})
	value.Snapshot.State = pricingStateAt(c.clock.Now(), value.Snapshot.FreshUntil, value.Snapshot.StaleUntil)
	return value
}

func pricingStateAt(now, freshUntil, staleUntil time.Time) modelbilling.PricingState {
	switch {
	case now.Before(freshUntil):
		return modelbilling.PricingFresh
	case now.Before(staleUntil):
		return modelbilling.PricingStaleUsable
	default:
		return modelbilling.PricingStaleBlocked
	}
}

func (c *SnapshotCache) path(source string) (string, error) {
	if !sourceIDPattern.MatchString(source) {
		return "", contractError("pricing source must be a public stable identifier")
	}
	return filepath.Join(c.dir, source+".snapshot.json"), nil
}

func validateRefreshRequest(request RefreshRequest) error {
	if !sourceIDPattern.MatchString(request.Source) {
		return contractError("pricing source must be a public stable identifier")
	}
	if strings.TrimSpace(request.Endpoint) == "" || strings.TrimSpace(request.Currency) == "" {
		return contractError("pricing endpoint and currency are required")
	}
	if request.Transport == nil || request.StatusSource == nil {
		return contractError("pricing transport and status source are required")
	}
	return nil
}

func encodeCacheFile(record cacheFile) ([]byte, error) {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func decodeCacheFile(data []byte) (cacheFile, error) {
	var record cacheFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil {
		return cacheFile{}, contractError("decode pricing cache: %v", err)
	}
	var trailer any
	if err := decoder.Decode(&trailer); err != io.EOF {
		if err == nil {
			return cacheFile{}, contractError("pricing cache contains multiple JSON values")
		}
		return cacheFile{}, contractError("decode pricing cache trailer: %v", err)
	}
	if record.Version != CacheFormatVersion {
		return cacheFile{}, contractError("unsupported pricing cache version %d", record.Version)
	}
	return record, nil
}

func unavailableFrom(cause error) error {
	retryable := true
	var domainErr *modelbilling.Error
	if errors.As(cause, &domainErr) {
		retryable = domainErr.Retryable
	}
	return &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable, Message: fmt.Sprintf("pricing refresh failed without a legal cache: %v", cause), Retryable: retryable}
}

func unavailableError(format string, args ...any) error {
	return &modelbilling.Error{Code: modelbilling.ErrorPricingUnavailable, Message: fmt.Sprintf(format, args...), Retryable: true}
}
