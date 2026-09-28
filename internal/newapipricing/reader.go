package newapipricing

import "context"

// Reader is the single B1 read/refresh boundary shared by the model
// marketplace and B2 price preparation.
type Reader struct {
	cache *SnapshotCache
}

func NewReader(cache *SnapshotCache) (*Reader, error) {
	if cache == nil {
		return nil, contractError("pricing snapshot cache is required")
	}
	return &Reader{cache: cache}, nil
}

// Read is cache-only. It never refreshes or accesses a transport/status source.
func (r *Reader) Read(source string) (CachedSnapshot, error) {
	if r == nil || r.cache == nil {
		return CachedSnapshot{}, contractError("pricing reader is unavailable")
	}
	return r.cache.Read(source)
}

// Refresh is the explicit network path and preserves the Catalog produced by
// SnapshotCache.RefreshWithCatalog.
func (r *Reader) Refresh(ctx context.Context, request RefreshRequest) (RefreshResult, error) {
	if r == nil || r.cache == nil {
		return RefreshResult{}, contractError("pricing reader is unavailable")
	}
	return r.cache.RefreshWithCatalog(ctx, request)
}
