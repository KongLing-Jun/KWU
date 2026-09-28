package connector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

func strictHTTPClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type HTTPSource struct {
	URL    string
	Client *http.Client
}

func (s HTTPSource) Fetch(ctx context.Context) ([]byte, error) {
	if s.URL == "" {
		return nil, errors.New("catalog: empty source URL")
	}
	client := s.Client
	if client == nil {
		client = strictHTTPClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("catalog: create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("catalog: fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog: unexpected HTTP status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("catalog: read: %w", err)
	}
	if int64(len(raw)) > MaxCatalogBytes {
		return nil, errors.New("catalog: response too large")
	}
	return raw, nil
}

type DownloadedPackage struct {
	Path   string
	Size   int64
	SHA256 string
}

type Downloader struct {
	Client *http.Client
}

func (d Downloader) Download(ctx context.Context, item CatalogItem, tempRoot string) (result DownloadedPackage, err error) {
	client := d.Client
	if client == nil {
		client = strictHTTPClient()
	}
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		return result, fmt.Errorf("connector download: create temporary directory: %w", err)
	}
	f, err := os.CreateTemp(tempRoot, ".connector-*.zip")
	if err != nil {
		return result, fmt.Errorf("connector download: create temporary file: %w", err)
	}
	result.Path = f.Name()
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(result.Path)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.Package.URL, nil)
	if err != nil {
		return result, fmt.Errorf("connector download: create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return result, fmt.Errorf("connector download: request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return result, fmt.Errorf("connector download: unexpected HTTP status %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxPackageBytes || (resp.ContentLength >= 0 && resp.ContentLength != item.Package.SizeBytes) {
		return result, errors.New("connector download: invalid content length")
	}
	hash := newSHA256Writer()
	written, err := io.Copy(io.MultiWriter(f, hash), io.LimitReader(resp.Body, MaxPackageBytes+1))
	if err != nil {
		return result, fmt.Errorf("connector download: read package: %w", err)
	}
	if written > MaxPackageBytes {
		return result, errors.New("connector download: package too large")
	}
	if written != item.Package.SizeBytes {
		return result, errors.New("connector download: size mismatch")
	}
	result.Size = written
	result.SHA256 = hash.SumHex()
	if result.SHA256 != item.Package.SHA256 {
		return result, errors.New("connector download: sha256 mismatch")
	}
	if err := f.Sync(); err != nil {
		return result, fmt.Errorf("connector download: sync temporary file: %w", err)
	}
	if err := f.Close(); err != nil {
		return result, fmt.Errorf("connector download: close temporary file: %w", err)
	}
	if filepath.Ext(result.Path) != ".zip" {
		return result, errors.New("connector download: invalid temporary filename")
	}
	keep = true
	return result, nil
}
