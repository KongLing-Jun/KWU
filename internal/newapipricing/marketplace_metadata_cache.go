package newapipricing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/fileutil"
)

type MarketplaceMetadataCache struct {
	dir       string
	writeFile func(string, []byte, os.FileMode) error
}

func NewMarketplaceMetadataCache(dir string) (*MarketplaceMetadataCache, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, contractError("marketplace metadata cache directory is required")
	}
	return &MarketplaceMetadataCache{dir: dir, writeFile: fileutil.AtomicWriteFileStrict}, nil
}

func (c *MarketplaceMetadataCache) Read(source string, cached CachedSnapshot) (MarketplaceMetadataFile, error) {
	if c == nil {
		return MarketplaceMetadataFile{}, metadataError(MetadataUnavailable, errors.New("marketplace metadata cache is unavailable"))
	}
	path, err := c.path(source)
	if err != nil {
		return MarketplaceMetadataFile{}, metadataError(MetadataUnavailable, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MarketplaceMetadataFile{}, metadataError(MetadataUnavailable, err)
	}
	metadata, err := validatedMarketplaceMetadata(data)
	if err == nil {
		err = validateMarketplaceMetadataIdentity(source, cached, metadata)
	}
	if err != nil {
		return MarketplaceMetadataFile{}, metadataError(MetadataUnavailable, err)
	}
	return metadata, nil
}

func validatedMarketplaceMetadata(data []byte) (MarketplaceMetadataFile, error) {
	metadata, err := decodeMarketplaceMetadata(data)
	if err != nil {
		return MarketplaceMetadataFile{}, err
	}
	if err := validateMarketplaceMetadata(metadata); err != nil {
		return MarketplaceMetadataFile{}, err
	}
	return metadata, nil
}

func validateMarketplaceMetadataIdentity(source string, cached CachedSnapshot, metadata MarketplaceMetadataFile) error {
	if source != cached.Snapshot.Source || metadata.SourceID != cached.Snapshot.Source ||
		metadata.SnapshotID != cached.Snapshot.SnapshotID || metadata.SourceHash != cached.Snapshot.SourceHash {
		return contractError("marketplace metadata identity does not match pricing snapshot")
	}
	return nil
}

func (c *MarketplaceMetadataCache) Write(metadata MarketplaceMetadataFile) error {
	if c == nil {
		return metadataError(MetadataWriteFailed, errors.New("marketplace metadata cache is unavailable"))
	}
	if err := validateMarketplaceMetadata(metadata); err != nil {
		return metadataError(MetadataWriteFailed, err)
	}
	data, err := encodeMarketplaceMetadata(metadata)
	if err == nil {
		var decoded MarketplaceMetadataFile
		decoded, err = decodeMarketplaceMetadata(data)
		if err == nil {
			err = validateMarketplaceMetadata(decoded)
		}
	}
	if err != nil {
		return metadataError(MetadataWriteFailed, err)
	}
	path, err := c.path(metadata.SourceID)
	if err == nil {
		err = c.writeFile(path, data, 0o600)
	}
	if err != nil {
		return metadataError(MetadataWriteFailed, err)
	}
	return nil
}

func (c *MarketplaceMetadataCache) path(source string) (string, error) {
	if !sourceIDPattern.MatchString(source) {
		return "", contractError("marketplace metadata source must be a public stable identifier")
	}
	return filepath.Join(c.dir, source+".marketplace.json"), nil
}

func encodeMarketplaceMetadata(metadata MarketplaceMetadataFile) ([]byte, error) {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode marketplace metadata: %w", err)
	}
	return append(data, '\n'), nil
}

func decodeMarketplaceMetadata(data []byte) (MarketplaceMetadataFile, error) {
	var metadata MarketplaceMetadataFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return MarketplaceMetadataFile{}, contractError("decode marketplace metadata: %v", err)
	}
	var trailer any
	if err := decoder.Decode(&trailer); err != io.EOF {
		if err == nil {
			return MarketplaceMetadataFile{}, contractError("marketplace metadata contains multiple JSON values")
		}
		return MarketplaceMetadataFile{}, contractError("decode marketplace metadata trailer: %v", err)
	}
	return metadata, nil
}
