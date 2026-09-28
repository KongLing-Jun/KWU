package connector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type ScopeConfirmation func(context.Context, OwnerScope) error

type Service struct {
	mode       CatalogMode
	source     CatalogSource
	downloader Downloader
	installer  Installer
	states     StateStore
	tempRoot   string
	closer     interface{ Close() error }

	mu           sync.RWMutex
	operationMu  sync.Mutex
	catalog      Catalog
	catalogReady bool
	catalogErr   error
	refreshing   atomic.Bool
}

var ErrCatalogRefreshInProgress = errors.New("connector catalog refresh is in progress")

type ServiceOptions struct {
	Mode       CatalogMode
	Source     CatalogSource
	Downloader Downloader
	Installer  Installer
	StateStore StateStore
	TempRoot   string
	Closer     interface{ Close() error }
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Source == nil || options.Installer.Root == "" || options.StateStore.SharedRoot == "" || options.TempRoot == "" {
		return nil, errors.New("connector service: incomplete options")
	}
	return &Service{
		mode: options.Mode, source: options.Source, downloader: options.Downloader, installer: options.Installer,
		states: options.StateStore, tempRoot: options.TempRoot, closer: options.Closer,
	}, nil
}

func (s *Service) Start(ctx context.Context) error {
	_, err := s.RefreshCatalog(ctx)
	return err
}

func (s *Service) Close() error {
	if s == nil || s.closer == nil {
		return nil
	}
	return s.closer.Close()
}

func (s *Service) RefreshCatalog(ctx context.Context) (Catalog, error) {
	return s.refreshCatalog(ctx, false)
}

// RecoverCatalog does not refetch a ready catalog observed by another caller.
func (s *Service) RecoverCatalog(ctx context.Context) (Catalog, error) {
	return s.refreshCatalog(ctx, true)
}

func (s *Service) refreshCatalog(ctx context.Context, onlyMissing bool) (Catalog, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return Catalog{}, err
	}
	if !s.refreshing.CompareAndSwap(false, true) {
		return Catalog{}, ErrCatalogRefreshInProgress
	}
	defer s.refreshing.Store(false)
	if onlyMissing {
		if catalog, err := s.Catalog(); err == nil {
			return catalog, nil
		}
	}
	return s.loadCatalog(ctx)
}

func (s *Service) loadCatalog(ctx context.Context) (Catalog, error) {
	raw, err := s.source.Fetch(ctx)
	if err != nil {
		s.mu.Lock()
		s.catalogErr = err
		s.mu.Unlock()
		return Catalog{}, err
	}
	catalog, err := ParseCatalog(raw, s.mode)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		s.mu.Lock()
		s.catalogErr = err
		s.mu.Unlock()
		return Catalog{}, err
	}
	s.mu.Lock()
	s.catalog = catalog
	s.catalogReady = true
	s.catalogErr = nil
	s.mu.Unlock()
	return cloneCatalog(catalog), nil
}

func (s *Service) Catalog() (Catalog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.catalogReady {
		if s.catalogErr != nil {
			return Catalog{}, s.catalogErr
		}
		return Catalog{}, errors.New("connector service: catalog is unavailable")
	}
	return cloneCatalog(s.catalog), nil
}

func (s *Service) Install(ctx context.Context, connectorID string, scope OwnerScope, confirm ScopeConfirmation) (Snapshot, error) {
	if err := scope.Validate(); err != nil {
		return Snapshot{}, err
	}
	s.operationMu.Lock()
	defer s.operationMu.Unlock()
	s.mu.RLock()
	item, ok := findCatalogItem(s.catalog, connectorID)
	ready := s.catalogReady
	s.mu.RUnlock()
	if !ready {
		return Snapshot{}, errors.New("connector service: a fresh catalog is required")
	}
	if !ok {
		return Snapshot{}, fmt.Errorf("connector service: connector %q is not in the catalog", connectorID)
	}
	if !item.Compatible {
		return Snapshot{}, fmt.Errorf("connector service: connector is blocked: %s", item.BlockedReason)
	}
	active, _, _, activeFound, activeErr := s.installer.ActiveManifest(connectorID)
	if activeErr != nil {
		return Snapshot{}, activeErr
	}
	preserveConnected := false
	if activeFound {
		owner, ownerFound, ownerErr := s.states.LoadOwner(scope, connectorID)
		if ownerErr != nil {
			return Snapshot{}, ownerErr
		}
		preserveConnected = ownerFound && owner.Version == active.Version && owner.SHA256 == active.SHA256 && owner.Status == StatusConnected
	}
	if activeFound {
		comparison := compareVersions(item.Version, active.Version)
		if comparison < 0 {
			return s.snapshot(connectorID, scope)
		}
		if comparison == 0 {
			if item.Package.SHA256 != active.SHA256 {
				return Snapshot{}, errors.New("connector service: current version content conflicts with active")
			}
			return s.snapshot(connectorID, scope)
		}
	}
	if err := s.states.WriteInstall(connectorID, StatusInstalling, nil); err != nil {
		return Snapshot{}, err
	}
	downloaded, err := s.downloader.Download(ctx, item, s.tempRoot)
	if err != nil {
		if activeFound {
			_ = s.states.ClearInstall(connectorID)
			return s.snapshot(connectorID, scope)
		}
		if stateErr := s.failInstall(connectorID, err); stateErr != nil {
			return Snapshot{}, stateErr
		}
		return s.snapshot(connectorID, scope)
	}
	defer os.Remove(downloaded.Path)
	installed, err := s.installer.Install(ctx, item, downloaded)
	if err != nil {
		if activeFound {
			_ = s.states.ClearInstall(connectorID)
			return s.snapshot(connectorID, scope)
		}
		if stateErr := s.failInstall(connectorID, err); stateErr != nil {
			return Snapshot{}, stateErr
		}
		return s.snapshot(connectorID, scope)
	}
	if err := s.states.ClearInstall(connectorID); err != nil {
		return Snapshot{}, err
	}
	if confirm != nil {
		if err := confirm(ctx, scope); err != nil {
			return Snapshot{}, err
		}
	}
	ownerStatus := StatusAuthorizationNeeded
	if preserveConnected {
		ownerStatus = StatusConnected
	}
	if err := s.states.WriteOwner(scope, connectorID, installed.Active, ownerStatus); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{ConnectorID: connectorID, Status: ownerStatus, Version: installed.Active.Version, SHA256: installed.Active.SHA256, PackagePath: installed.Path}, nil
}

func (s *Service) Manifest(connectorID string) (ActiveVersion, string, Manifest, error) {
	active, packagePath, manifest, found, err := s.installer.ActiveManifest(connectorID)
	if err != nil {
		return ActiveVersion{}, "", Manifest{}, err
	}
	if !found {
		return ActiveVersion{}, "", Manifest{}, errors.New("connector service: connector is not installed")
	}
	return active, packagePath, manifest, nil
}

func (s *Service) CatalogItem(connectorID string) (CatalogItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.catalogReady {
		return CatalogItem{}, errors.New("connector service: catalog is unavailable")
	}
	item, ok := findCatalogItem(s.catalog, connectorID)
	if !ok {
		return CatalogItem{}, fmt.Errorf("connector service: connector %q is not in the catalog", connectorID)
	}
	return cloneCatalogItem(item), nil
}

func (s *Service) Snapshot(connectorID string, scope OwnerScope) (Snapshot, error) {
	return s.snapshot(connectorID, scope)
}

func (s *Service) snapshot(connectorID string, scope OwnerScope) (Snapshot, error) {
	active, packagePath, found, err := s.installer.Active(connectorID)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		install, found, err := s.states.LoadInstall(connectorID)
		if err != nil {
			return Snapshot{}, err
		}
		if found {
			return Snapshot{ConnectorID: connectorID, Status: install.Status, Error: cloneSafeError(install.Error)}, nil
		}
		return Snapshot{ConnectorID: connectorID, Status: StatusNotInstalled}, nil
	}
	owner, found, err := s.states.LoadOwner(scope, connectorID)
	if err != nil {
		return Snapshot{}, err
	}
	status := StatusAuthorizationNeeded
	if found && owner.Version == active.Version && owner.SHA256 == active.SHA256 {
		status = owner.Status
	}
	var safeErr *SafeError
	if found && owner.Version == active.Version && owner.SHA256 == active.SHA256 {
		safeErr = cloneSafeError(owner.Error)
	}
	return Snapshot{ConnectorID: connectorID, Status: status, Version: active.Version, SHA256: active.SHA256, Error: safeErr, PackagePath: packagePath}, nil
}

// SetOwnerStatus records a managed connector's owner-scoped runtime state.
func (s *Service) SetOwnerStatus(scope OwnerScope, connectorID string, status Status, safeErr *SafeError) (Snapshot, error) {
	if err := scope.Validate(); err != nil {
		return Snapshot{}, err
	}
	active, packagePath, found, err := s.installer.Active(connectorID)
	if err != nil {
		return Snapshot{}, err
	}
	if !found {
		return Snapshot{}, errors.New("connector service: connector is not installed")
	}
	if err := s.states.WriteOwnerWithError(scope, connectorID, active, status, safeErr); err != nil {
		return Snapshot{}, err
	}
	return Snapshot{ConnectorID: connectorID, Status: status, Version: active.Version, SHA256: active.SHA256, Error: cloneSafeError(safeErr), PackagePath: packagePath}, nil
}

func (s *Service) failInstall(connectorID string, cause error) error {
	return s.states.WriteInstall(connectorID, StatusFailed, safeInstallError(cause))
}

func findCatalogItem(catalog Catalog, connectorID string) (CatalogItem, bool) {
	for _, item := range catalog.Items {
		if item.ID == connectorID {
			return item, true
		}
	}
	return CatalogItem{}, false
}

func cloneCatalog(catalog Catalog) Catalog {
	copy := catalog
	copy.Items = append([]CatalogItem(nil), catalog.Items...)
	for i := range copy.Items {
		copy.Items[i].Examples = append([]string(nil), catalog.Items[i].Examples...)
		copy.Items[i].Capabilities = append([]string(nil), catalog.Items[i].Capabilities...)
	}
	return copy
}

func cloneCatalogItem(item CatalogItem) CatalogItem {
	item.Examples = append([]string(nil), item.Examples...)
	item.Capabilities = append([]string(nil), item.Capabilities...)
	return item
}

func DefaultRoots(cacheRoot string) (packages, temp string) {
	root := filepath.Join(cacheRoot, "connectors")
	return root, filepath.Join(root, "tmp")
}
