//go:build testdiag

package testdiag

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPprofServerIsLoopbackOnlyAndCloses(t *testing.T) {
	diagnostics, err := StartPprof()
	if err != nil {
		t.Fatal(err)
	}

	parsed, err := url.Parse(diagnostics.URL())
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Hostname() != "127.0.0.1" {
		t.Fatalf("diagnostics host = %q, want loopback", parsed.Hostname())
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/goroutine?debug=1"} {
		response, err := client.Get(diagnostics.URL() + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read/close %s: %v / %v", path, readErr, closeErr)
		}
		if response.StatusCode != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, response.StatusCode)
		}
		if len(body) == 0 {
			t.Fatalf("GET %s returned an empty body", path)
		}
		if path != "/debug/pprof/" && !strings.Contains(string(body), "goroutine profile") {
			t.Fatalf("GET %s did not return a goroutine profile", path)
		}
	}

	closeContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := diagnostics.Close(closeContext); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := diagnostics.Close(closeContext); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	if _, err := client.Get(diagnostics.URL() + "/debug/pprof/"); err == nil {
		t.Fatal("diagnostics endpoint remained reachable after Close")
	}
}
