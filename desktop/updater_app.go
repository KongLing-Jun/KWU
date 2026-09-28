package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kwu/desktop/internal/update"
)

// updater_app.go is the auto-updater's bound command surface 鈥?the App methods the
// frontend calls 鈥?mirroring settings_app.go's "one file per concern" split. The
// transport-free logic lives in updater.go; this file is the Wails glue: it streams
// download progress as "updater:progress" events and routes macOS to the manual
// download path unless the macOS build was Developer ID signed and notarized.

// Version returns the build version injected via -ldflags (see main.go). The
// frontend displays it; CheckUpdate compares against it.
func (a *App) Version() string { return version }

// CheckUpdate fetches the manifest and reports whether a newer
// build is available for this platform. Safe to call on startup: a network error
// surfaces in UpdateInfo.Err rather than failing, so the UI can stay quiet.
func (a *App) CheckUpdate() (*UpdateInfo, error) {
	c, err := httpClient()
	if err != nil {
		return &UpdateInfo{
			Current:       version,
			Channel:       channel,
			CanSelfUpdate: false,
			ManualOnly:    true,
			ManualKind:    manualUpdateKind(),
			DownloadURL:   downloadPage(),
			Err:           err.Error(),
		}, nil
	}
	m, err := fetchManifest(a.reqCtx(), c)
	if err != nil {
		return &UpdateInfo{
			Current:       version,
			Channel:       channel,
			CanSelfUpdate: false,
			ManualOnly:    true,
			ManualKind:    manualUpdateKind(),
			DownloadURL:   downloadPage(),
			Err:           err.Error(),
		}, nil
	}
	info := evaluate(version, m)
	if info.Available {
		_, hasManifestAsset := m.Asset()
		var release *cloudReleaseMetadata
		baseURL, baseErr := resolveCloudBaseURL(version, os.Getenv(cloudBaseURLEnv), cloudDevelopmentBaseURL, cloudProductionBaseURL)
		if baseErr == nil {
			cloudCtx, cloudCancel := context.WithTimeout(a.reqCtx(), cloudReleaseTimeout)
			release, _ = fetchCloudReleaseMetadata(cloudCtx, c, baseURL, runtime.GOOS, runtime.GOARCH)
			cloudCancel()
		}
		reconcileUpdateAvailability(&info, version, hasManifestAsset, release)
	}
	return &info, nil
}

// OpenDownloadPage opens the releases page in the browser 鈥?the macOS manual-update
// path and a fallback link elsewhere.
func (a *App) OpenDownloadPage() {
	page := downloadPage()
	if c, err := httpClient(); err == nil {
		if m, err := fetchManifest(a.reqCtx(), c); err == nil && m.DownloadPage != "" {
			page = m.DownloadPage
		}
	}
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, page)
	}
}

// OpenUpdateDownload opens a release-specific manual download. Only absolute
// HTTPS URLs can leave the app; missing or malformed metadata falls back to the
// existing GitHub releases page.
func (a *App) OpenUpdateDownload(rawURL string) {
	if a.ctx != nil {
		wruntime.BrowserOpenURL(a.ctx, safeUpdateDownloadURL(rawURL))
	}
}

// DownloadUpdate downloads, verifies, and caches the latest build. Installation is
// deliberately a separate user action so the UI can show "downloaded" before the
// app quits to finish the update.
func (a *App) DownloadUpdate() (*UpdateDownloadResult, error) {
	if !canSelfUpdate() {
		a.OpenDownloadPage()
		return nil, nil
	}
	c, err := httpClient()
	if err != nil {
		return nil, a.failUpdate(err)
	}
	verified, err := downloadUpdateFromSources(a.reqCtx(), c, releaseSources(), manifestSourceTimeout, func(ctx context.Context, asset update.Asset) ([]byte, error) {
		a.emitProgress("downloading", 0, asset.Size, "")
		return a.downloadVerify(ctx, asset)
	})
	if err != nil {
		return nil, a.failUpdate(err)
	}
	meta, err := saveCachedUpdate(verified.Version, verified.Asset, verified.Data)
	if err != nil {
		return nil, a.failUpdate(err)
	}
	a.emitProgress("downloaded", meta.Size, meta.Size, "")
	return &UpdateDownloadResult{
		Version: meta.Version,
		Channel: meta.Channel,
		Path:    meta.Path,
		Size:    meta.Size,
		SHA256:  meta.SHA256,
	}, nil
}

// InstallUpdate applies the cached, verified update and then exits/relaunches.
func (a *App) InstallUpdate() error {
	if !canSelfUpdate() {
		a.OpenDownloadPage()
		return nil
	}
	meta, err := applyCurrentCachedUpdate(
		a.reqCtx(),
		httpClient,
		releaseAuthorizationSources(),
		manifestSourceTimeout,
		func(meta *cachedUpdate, data []byte) error {
			a.emitProgress("installing", meta.Size, meta.Size, "")
			switch runtime.GOOS {
			case "windows":
				return applyWindowsFile(meta.Path)
			case "darwin":
				return applyMac(meta.Path)
			case "linux":
				return applyLinux(data)
			default:
				return fmt.Errorf("self-update unsupported on %s", runtime.GOOS)
			}
		},
	)
	if err != nil {
		return a.failUpdate(err)
	}

	a.emitProgress("done", meta.Size, meta.Size, "")

	// Persist the conversation and stop subprocesses before handing off (same as
	// shutdown). On Linux the binary is now replaced, so relaunch it; on Windows and
	// macOS the installer/helper we launched takes over once we exit.
	a.shutdown(a.ctx)
	if runtime.GOOS == "linux" {
		_ = relaunch()
	}
	os.Exit(0)
	return nil
}

// ApplyUpdate is kept for older frontend bindings and tests. New UI code uses the
// explicit download 鈫?install split.
func (a *App) ApplyUpdate() error {
	if _, err := a.DownloadUpdate(); err != nil {
		return err
	}
	return a.InstallUpdate()
}

// downloadVerify downloads the asset (streaming progress), verifies its minisign
// signature against the embedded public key, then its sha256. It returns the
// verified bytes and never touches disk on a bad signature.
func (a *App) downloadVerify(ctx context.Context, asset update.Asset) ([]byte, error) {
	c, err := httpClient()
	if err != nil {
		return nil, err
	}
	v4, _ := httpClientIPv4() // best-effort IPv4 fallback; nil just means retries reuse c
	data, err := download(ctx, c, v4, asset.URL, asset.Size, func(rcv, total int64) {
		a.emitProgress("downloading", rcv, total, "")
	})
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}
	a.emitProgress("verifying", asset.Size, asset.Size, "")
	sigCtx, cancel := context.WithTimeout(ctx, manifestSourceTimeout)
	defer cancel()
	sig, err := fetchBytes(sigCtx, c, asset.Sig)
	if err != nil {
		return nil, fmt.Errorf("fetch signature: %w", err)
	}
	if err := update.Verify(data, sig); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}
	if err := checkSHA256(data, asset.SHA256); err != nil {
		return nil, fmt.Errorf("verify sha256: %w", err)
	}
	return data, nil
}

// reqCtx is the context for updater HTTP calls 鈥?the Wails context once startup has
// run, else Background (CheckUpdate may, in theory, be reached before startup).
func (a *App) reqCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) emitProgress(phase string, received, total int64, errMsg string) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "updater:progress", updateProgress{
		Phase: phase, Received: received, Total: total, Err: errMsg,
	})
}

// failUpdate emits an error progress event and returns the error to the caller.
func (a *App) failUpdate(err error) error {
	a.emitProgress("error", 0, 0, err.Error())
	return err
}
