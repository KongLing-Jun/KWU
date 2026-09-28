package connector

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"kwu/internal/fileutil"
)

type ActiveVersion struct {
	ConnectorID string `json:"connector_id"`
	Version     string `json:"version"`
	SHA256      string `json:"sha256"`
}

type InstallResult struct {
	Active   ActiveVersion
	Path     string
	Manifest Manifest
}

type installReceipt struct {
	ConnectorID string               `json:"connector_id"`
	Version     string               `json:"version"`
	SHA256      string               `json:"sha256"`
	Files       []installReceiptFile `json:"files"`
}

type installReceiptFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type Installer struct {
	Root   string
	Policy RuntimePolicy
}

func (i Installer) Install(ctx context.Context, item CatalogItem, downloaded DownloadedPackage) (result InstallResult, err error) {
	if i.Root == "" {
		return result, errors.New("connector install: empty package root")
	}
	if err := verifyDownloadedPackage(downloaded, item); err != nil {
		return result, err
	}
	reader, err := zip.OpenReader(downloaded.Path)
	if err != nil {
		return result, fmt.Errorf("connector install: invalid ZIP: %w", err)
	}
	defer reader.Close()
	inspected, err := inspectPackageZip(&reader.Reader)
	if err != nil {
		return result, err
	}
	manifest, err := parseManifests(inspected, item, i.Policy)
	if err != nil {
		return result, err
	}
	connectorRoot := filepath.Join(i.Root, item.ID)
	versionRoot := filepath.Join(connectorRoot, "packages", item.Version)
	active, found, err := loadActive(connectorRoot)
	if err != nil {
		return result, err
	}
	if found {
		if active.ConnectorID == item.ID && active.Version == item.Version && active.SHA256 == item.Package.SHA256 {
			if err := verifyInstallReceipt(connectorRoot, versionRoot, active); err != nil {
				return result, err
			}
			installed, loadErr := LoadInstalledManifest(versionRoot, active, i.Policy)
			if loadErr != nil {
				return result, loadErr
			}
			return InstallResult{Active: active, Path: versionRoot, Manifest: installed}, nil
		}
		comparison := compareVersions(item.Version, active.Version)
		if comparison < 0 {
			return result, errors.New("connector install: automatic downgrade is forbidden")
		}
		if comparison == 0 {
			return result, errors.New("connector install: same version has different content")
		}
	}
	if _, statErr := os.Stat(versionRoot); statErr == nil {
		return result, errors.New("connector install: orphaned version directory exists")
	} else if !os.IsNotExist(statErr) {
		return result, fmt.Errorf("connector install: inspect version directory: %w", statErr)
	}
	packagesRoot := filepath.Dir(versionRoot)
	if err := os.MkdirAll(packagesRoot, 0o700); err != nil {
		return result, fmt.Errorf("connector install: create packages root: %w", err)
	}
	tempRoot, err := os.MkdirTemp(packagesRoot, ".install-*")
	if err != nil {
		return result, fmt.Errorf("connector install: create temporary directory: %w", err)
	}
	keepVersion := false
	receiptPath := ""
	defer func() {
		_ = os.RemoveAll(tempRoot)
		if err != nil && keepVersion {
			_ = os.RemoveAll(versionRoot)
			if receiptPath != "" {
				_ = os.Remove(receiptPath)
			}
		}
	}()
	if err = extractInspectedZip(ctx, inspected, tempRoot); err != nil {
		return result, err
	}
	if err = os.Rename(tempRoot, versionRoot); err != nil {
		return result, fmt.Errorf("connector install: activate version directory: %w", err)
	}
	keepVersion = true
	active = ActiveVersion{ConnectorID: item.ID, Version: item.Version, SHA256: item.Package.SHA256}
	receipt, err := buildInstallReceipt(versionRoot, active)
	if err != nil {
		return result, err
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		return result, fmt.Errorf("connector install: encode receipt: %w", err)
	}
	receiptPath = installReceiptPath(connectorRoot, item.Version)
	if err = os.MkdirAll(filepath.Dir(receiptPath), 0o700); err != nil {
		return result, fmt.Errorf("connector install: create receipt root: %w", err)
	}
	if err = fileutil.AtomicWriteFileStrict(receiptPath, receiptRaw, 0o600); err != nil {
		return result, fmt.Errorf("connector install: write receipt: %w", err)
	}
	raw, err := json.Marshal(active)
	if err != nil {
		return result, fmt.Errorf("connector install: encode active version: %w", err)
	}
	if err = fileutil.AtomicWriteFileStrict(filepath.Join(connectorRoot, "active.json"), raw, 0o600); err != nil {
		_ = os.Remove(receiptPath)
		return result, fmt.Errorf("connector install: write active version: %w", err)
	}
	keepVersion = false
	return InstallResult{Active: active, Path: versionRoot, Manifest: manifest}, nil
}

func (i Installer) Active(connectorID string) (ActiveVersion, string, bool, error) {
	root := filepath.Join(i.Root, connectorID)
	active, found, err := loadActive(root)
	if err != nil || !found {
		return active, "", found, err
	}
	if active.ConnectorID != connectorID || !validLowerSHA256(active.SHA256) {
		return ActiveVersion{}, "", false, errors.New("connector install: invalid active version")
	}
	version, err := parseNormalizedSemver(active.Version)
	if err != nil || version != active.Version {
		return ActiveVersion{}, "", false, errors.New("connector install: invalid active version")
	}
	path := filepath.Join(root, "packages", active.Version)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ActiveVersion{}, "", false, nil
		}
		return ActiveVersion{}, "", false, err
	}
	if !info.IsDir() {
		return ActiveVersion{}, "", false, errors.New("connector install: active version is not a directory")
	}
	return active, path, true, nil
}

func (i Installer) ActiveManifest(connectorID string) (ActiveVersion, string, Manifest, bool, error) {
	active, packagePath, found, err := i.Active(connectorID)
	if err != nil || !found {
		return active, packagePath, Manifest{}, found, err
	}
	if err := verifyInstallReceipt(filepath.Join(i.Root, connectorID), packagePath, active); err != nil {
		return ActiveVersion{}, "", Manifest{}, false, err
	}
	manifest, err := LoadInstalledManifest(packagePath, active, i.Policy)
	if err != nil {
		return ActiveVersion{}, "", Manifest{}, false, err
	}
	return active, packagePath, manifest, true, nil
}

func installReceiptPath(connectorRoot, version string) string {
	return filepath.Join(connectorRoot, "receipts", version+".json")
}

func buildInstallReceipt(packageRoot string, active ActiveVersion) (installReceipt, error) {
	receipt := installReceipt{ConnectorID: active.ConnectorID, Version: active.Version, SHA256: active.SHA256}
	err := filepath.WalkDir(packageRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("connector install: active package contains a symlink")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("connector install: active package contains an invalid file")
		}
		relative, err := filepath.Rel(packageRoot, filePath)
		if err != nil {
			return err
		}
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		hash := newSHA256Writer()
		written, copyErr := io.Copy(hash, io.LimitReader(file, MaxPackageUncompressedBytes+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil || written != info.Size() || written > MaxPackageUncompressedBytes {
			return errors.New("connector install: hash active package file")
		}
		receipt.Files = append(receipt.Files, installReceiptFile{Path: filepath.ToSlash(relative), Size: info.Size(), SHA256: hash.SumHex()})
		return nil
	})
	if err != nil {
		return installReceipt{}, fmt.Errorf("connector install: build receipt: %w", err)
	}
	if len(receipt.Files) == 0 {
		return installReceipt{}, errors.New("connector install: build receipt: package is empty")
	}
	return receipt, nil
}

func verifyInstallReceipt(connectorRoot, packageRoot string, active ActiveVersion) error {
	raw, err := os.ReadFile(installReceiptPath(connectorRoot, active.Version))
	if err != nil {
		return errors.New("connector install: active package receipt is unavailable")
	}
	var want installReceipt
	if decodeStrict(raw, &want) != nil || want.ConnectorID != active.ConnectorID || want.Version != active.Version || want.SHA256 != active.SHA256 {
		return errors.New("connector install: active package receipt is invalid")
	}
	got, err := buildInstallReceipt(packageRoot, active)
	if err != nil || len(got.Files) != len(want.Files) {
		return errors.New("connector install: active package content is damaged")
	}
	for index := range got.Files {
		if got.Files[index] != want.Files[index] {
			return errors.New("connector install: active package content is damaged")
		}
	}
	return nil
}

func verifyDownloadedPackage(downloaded DownloadedPackage, item CatalogItem) error {
	info, err := os.Stat(downloaded.Path)
	if err != nil {
		return fmt.Errorf("connector install: inspect downloaded package: %w", err)
	}
	if info.Size() != item.Package.SizeBytes || downloaded.Size != item.Package.SizeBytes || downloaded.SHA256 != item.Package.SHA256 {
		return errors.New("connector install: downloaded package metadata mismatch")
	}
	f, err := os.Open(downloaded.Path)
	if err != nil {
		return fmt.Errorf("connector install: open downloaded package: %w", err)
	}
	defer f.Close()
	hash := newSHA256Writer()
	written, err := io.Copy(hash, io.LimitReader(f, MaxPackageBytes+1))
	if err != nil || written != item.Package.SizeBytes || written > MaxPackageBytes || hash.SumHex() != item.Package.SHA256 {
		return errors.New("connector install: package size or sha256 mismatch")
	}
	return nil
}

func extractInspectedZip(ctx context.Context, inspected inspectedZip, target string) error {
	for _, file := range inspected.files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		name, err := normalizeZipPath(file.Name)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, filepath.FromSlash(name))
		if !strings.HasPrefix(filepath.Clean(destination), filepath.Clean(target)+string(os.PathSeparator)) {
			return errors.New("connector install: path escapes target")
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o700); err != nil {
				return fmt.Errorf("connector install: create directory: %w", err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return fmt.Errorf("connector install: create parent directory: %w", err)
		}
		src, err := file.Open()
		if err != nil {
			return fmt.Errorf("connector install: open ZIP entry: %w", err)
		}
		dst, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = src.Close()
			return fmt.Errorf("connector install: create output file: %w", err)
		}
		written, copyErr := io.Copy(dst, io.LimitReader(src, int64(file.UncompressedSize64)+1))
		closeErr := dst.Sync()
		if err := dst.Close(); closeErr == nil {
			closeErr = err
		}
		_ = src.Close()
		if copyErr != nil || closeErr != nil || written != int64(file.UncompressedSize64) {
			return errors.New("connector install: extract file failed")
		}
	}
	return nil
}

func loadActive(connectorRoot string) (ActiveVersion, bool, error) {
	raw, err := os.ReadFile(filepath.Join(connectorRoot, "active.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return ActiveVersion{}, false, nil
		}
		return ActiveVersion{}, false, fmt.Errorf("connector install: read active version: %w", err)
	}
	var active ActiveVersion
	if err := decodeStrict(raw, &active); err != nil {
		return ActiveVersion{}, false, fmt.Errorf("connector install: decode active version: %w", err)
	}
	return active, true, nil
}
