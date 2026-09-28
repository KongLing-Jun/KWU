package expert

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func holdUserStore(t *testing.T, path string, access, share uint32) windows.Handle {
	t.Helper()
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(pointer, access, share, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	return handle
}

func seedUserStore(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "experts.json")
	if err := ResetUserExpertStore(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUserStoreReadRecoversAfterSharingViolation(t *testing.T) {
	path := seedUserStore(t)
	handle := holdUserStore(t, path, windows.GENERIC_READ, 0)
	if _, err := os.ReadFile(path); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		windows.CloseHandle(handle)
		t.Fatalf("ordinary read = %v; want sharing violation", err)
	}
	closed := make(chan error, 1)
	go func() {
		time.Sleep(120 * time.Millisecond)
		closed <- windows.CloseHandle(handle)
	}()
	t.Cleanup(func() {
		if err := <-closed; err != nil {
			t.Error(err)
		}
	})
	if _, err := LoadUserExperts(path); err != nil {
		t.Fatalf("read after transient contention: %v", err)
	}
}

func TestUserStoreReadAllowsDeleteAccessHolder(t *testing.T) {
	path := seedUserStore(t)
	handle := holdUserStore(t, path, windows.DELETE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
	defer windows.CloseHandle(handle)
	if _, err := os.ReadFile(path); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
		t.Fatalf("ordinary read = %v; want missing-delete-sharing conflict", err)
	}
	if _, err := LoadUserExperts(path); err != nil {
		t.Fatalf("read with compatible delete sharing: %v", err)
	}
}

func TestUserStoreWritePreservesDestinationWhenRenameBlocked(t *testing.T) {
	path := seedUserStore(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	handle := holdUserStore(t, path, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE)
	writeErr := SaveExpertLibraryState(path, ExpertLibraryState{FavoriteIDs: []string{"kept"}})
	if err := windows.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	if writeErr == nil {
		t.Error("write succeeded despite blocked replacement")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("failed replacement changed original: %q, %v", after, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary cleanup: %v, %v", entries, err)
	}
}

func TestUserStoreReadReturnsPersistentSharingViolation(t *testing.T) {
	path := seedUserStore(t)
	handle := holdUserStore(t, path, windows.GENERIC_READ, 0)
	defer windows.CloseHandle(handle)
	started := time.Now()
	_, err := LoadUserExperts(path)
	var pathErr *os.PathError
	if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !errors.As(err, &pathErr) || pathErr.Path != path {
		t.Fatalf("persistent contention error = %v", err)
	}
	if elapsed := time.Since(started); elapsed < 600*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("read retry duration = %v", elapsed)
	}
}

func TestUserStoreReadRetriesByteRangeContention(t *testing.T) {
	for _, release := range []bool{true, false} {
		t.Run(fmt.Sprintf("release=%v", release), func(t *testing.T) {
			path := seedUserStore(t)
			payload := []byte(`{"version":2,"experts":[]}` + strings.Repeat(" ", 8192))
			if err := os.WriteFile(path, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			handle := holdUserStore(t, path, windows.GENERIC_READ|windows.GENERIC_WRITE,
				windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE)
			defer windows.CloseHandle(handle)
			overlap := windows.Overlapped{Offset: 4096}
			if err := windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlap); err != nil {
				t.Fatal(err)
			}
			if _, err := readUserStoreAttempt(path); !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
				t.Fatalf("read-stage contention = %v", err)
			}
			if release {
				closed := make(chan error, 1)
				go func() {
					time.Sleep(120 * time.Millisecond)
					closed <- windows.UnlockFileEx(handle, 0, 1, 0, &overlap)
				}()
				defer func() {
					if err := <-closed; err != nil {
						t.Error(err)
					}
				}()
			}
			started := time.Now()
			got, err := readUserStoreBytes(path)
			if release && (err != nil || !bytes.Equal(got, payload)) {
				t.Fatalf("recovered read = %d bytes, %v; want exact payload", len(got), err)
			}
			if !release && (got != nil || !errors.Is(err, windows.ERROR_LOCK_VIOLATION)) {
				t.Fatalf("persistent read-stage error = %v, bytes = %d", err, len(got))
			}
			if time.Since(started) > 5*time.Second {
				t.Fatal("read-stage retries exceeded budget")
			}
		})
	}
}

func TestUserStoreReadPathsAndHandleLifetime(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, path := range []string{
		filepath.Join("relative space", "\u4e13\u5bb6.json"),
		filepath.Join(strings.Repeat("a", 140), strings.Repeat("b", 140), "experts.json"),
	} {
		t.Run(fmt.Sprintf("path-length-%d", len(path)), func(t *testing.T) {
			if err := ResetUserExpertStore(path); err != nil {
				t.Fatal(err)
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{path, abs, `\\?\` + abs} {
				if _, err := LoadUserExperts(name); err != nil {
					t.Fatalf("read path %q: %v", name, err)
				}
			}
			file, err := openUserStoreRead(path)
			if err != nil {
				t.Fatal(err)
			}
			var flags uint32
			getHandleInformation := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")
			ok, _, callErr := getHandleInformation.Call(file.Fd(), uintptr(unsafe.Pointer(&flags)))
			file.Close()
			if ok == 0 || flags&windows.HANDLE_FLAG_INHERIT != 0 {
				t.Fatalf("read handle inheritance = %d, %v", flags, callErr)
			}
			handle := holdUserStore(t, `\\?\`+abs, windows.GENERIC_READ, 0)
			windows.CloseHandle(handle)
		})
	}
}

func TestUserStoreWriterRecoversAfterSharedReaderCloses(t *testing.T) {
	path := seedUserStore(t)
	file, err := openUserStoreRead(path)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() {
		time.Sleep(120 * time.Millisecond)
		closed <- file.Close()
	}()
	t.Cleanup(func() {
		if err := <-closed; err != nil {
			t.Error(err)
		}
	})
	if err := SaveExpertLibraryState(path, ExpertLibraryState{FavoriteIDs: []string{"updated"}}); err != nil {
		t.Fatalf("replace after shared reader closes: %v", err)
	}
	state, err := LoadExpertLibraryState(path)
	if err != nil || len(state.FavoriteIDs) != 1 || state.FavoriteIDs[0] != "updated" {
		t.Fatalf("published state = %+v, %v", state, err)
	}
}

func TestUserStoreConcurrentPublication(t *testing.T) {
	path := seedUserStore(t)
	states := []userStoreFile{
		{LibraryState: ExpertLibraryState{FavoriteIDs: []string{"small"}}},
		{LibraryState: ExpertLibraryState{FavoriteIDs: []string{strings.Repeat("large", 4000)}}},
	}
	var snapshots [][]byte
	for _, state := range states {
		if err := saveUserStoreFile(path, state); err != nil {
			t.Fatal(err)
		}
		body, err := readUserStoreBytes(path)
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, body)
	}
	start := make(chan struct{})
	results := make(chan error, 5)
	for worker := 0; worker < 5; worker++ {
		go func() {
			<-start
			results <- exerciseUserStorePublication(path, worker, states, snapshots)
		}()
	}
	close(start)
	for worker := 0; worker < 5; worker++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

func exerciseUserStorePublication(path string, worker int, states []userStoreFile, snapshots [][]byte) error {
	for iteration := 0; iteration < 80; iteration++ {
		if worker < 2 {
			if err := saveUserStoreFile(path, states[worker]); err != nil {
				return fmt.Errorf("writer %d: %w", worker, err)
			}
			continue
		}
		body, err := readUserStoreBytes(path)
		if err != nil {
			return fmt.Errorf("reader %d: %w", worker, err)
		}
		if !bytes.Equal(body, snapshots[0]) && !bytes.Equal(body, snapshots[1]) {
			return fmt.Errorf("reader %d saw a partial or mixed document", worker)
		}
	}
	return nil
}
