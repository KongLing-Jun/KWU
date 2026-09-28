package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kwu/internal/provider"
)

func TestHeartbeatDoesNotRefreshSemanticProgressAndRetriesOnce(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flush(w)
		writeHeartbeats(r.Context(), w, 10*time.Millisecond)
	}))
	defer srv.Close()

	c := newTimeoutTestClient(t, srv.URL)
	c.idleTimeout = time.Second
	c.progressTimeout = 60 * time.Millisecond
	c.requestTimeout = time.Second

	var got error
	for chunk := range mustTestStream(t, c) {
		if chunk.Type == provider.ChunkError {
			got = chunk.Err
		}
	}
	if !errors.Is(got, errStreamNoProgress) {
		t.Fatalf("stream error = %v, want semantic no-progress timeout", got)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want initial request plus exactly one safe replay", requests.Load())
	}
}

func TestSemanticProgressRefreshesProgressWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flush(w)
		for _, text := range []string{"one", "two", "three", "four"} {
			_, _ = fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", text)
			flush(w)
			time.Sleep(75 * time.Millisecond)
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		flush(w)
	}))
	defer srv.Close()

	c := newTimeoutTestClient(t, srv.URL)
	c.idleTimeout = time.Second
	c.progressTimeout = 200 * time.Millisecond
	c.requestTimeout = 2 * time.Second

	var text strings.Builder
	for chunk := range mustTestStream(t, c) {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("progressing stream failed: %v", chunk.Err)
		}
		if chunk.Type == provider.ChunkText {
			text.WriteString(chunk.Text)
		}
	}
	if text.String() != "onetwothreefour" {
		t.Fatalf("text = %q, want all semantic progress", text.String())
	}
}

func TestNoProgressAfterOutputDoesNotReplay(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n")
		flush(w)
		writeHeartbeats(r.Context(), w, 10*time.Millisecond)
	}))
	defer srv.Close()

	c := newTimeoutTestClient(t, srv.URL)
	c.idleTimeout = time.Second
	c.progressTimeout = 60 * time.Millisecond
	c.requestTimeout = time.Second

	var got error
	for chunk := range mustTestStream(t, c) {
		if chunk.Type == provider.ChunkError {
			got = chunk.Err
		}
	}
	var interrupted *provider.StreamInterruptedError
	if !errors.As(got, &interrupted) || !errors.Is(got, errStreamNoProgress) {
		t.Fatalf("stream error = %v, want interrupted semantic timeout", got)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no replay after output", requests.Load())
	}
}

func TestRequestWallClockTimeoutRetriesOnceWithoutOutput(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		flush(w)
		writeHeartbeats(r.Context(), w, 10*time.Millisecond)
	}))
	defer srv.Close()

	c := newTimeoutTestClient(t, srv.URL)
	c.idleTimeout = time.Second
	c.progressTimeout = time.Second
	c.requestTimeout = 60 * time.Millisecond

	var got error
	for chunk := range mustTestStream(t, c) {
		if chunk.Type == provider.ChunkError {
			got = chunk.Err
		}
	}
	if !errors.Is(got, errStreamRequestTimeout) {
		t.Fatalf("stream error = %v, want request wall-clock timeout", got)
	}
	if requests.Load() != 2 {
		t.Fatalf("requests = %d, want initial request plus exactly one safe replay", requests.Load())
	}
}

func TestFinishReasonGraceCompletesHeartbeatOnlyTail(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":\"stop\"}]}\n\n")
		flush(w)
		writeHeartbeats(r.Context(), w, 10*time.Millisecond)
	}))
	defer srv.Close()

	c := newTimeoutTestClient(t, srv.URL)
	c.idleTimeout = time.Second
	c.progressTimeout = time.Second
	c.requestTimeout = time.Second
	c.finishGrace = 60 * time.Millisecond

	var text strings.Builder
	for chunk := range mustTestStream(t, c) {
		if chunk.Type == provider.ChunkError {
			t.Fatalf("finish_reason grace should complete cleanly: %v", chunk.Err)
		}
		if chunk.Type == provider.ChunkText {
			text.WriteString(chunk.Text)
		}
	}
	if text.String() != "done" {
		t.Fatalf("text = %q, want done", text.String())
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want no replay after finish_reason", requests.Load())
	}
}

func newTimeoutTestClient(t *testing.T, baseURL string) *client {
	t.Helper()
	p, err := New(provider.Config{Name: "openai", BaseURL: baseURL, Model: "test", APIKey: "k"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p.(*client)
}

func mustTestStream(t *testing.T, c *client) <-chan provider.Chunk {
	t.Helper()
	ch, err := c.Stream(context.Background(), provider.Request{Messages: []provider.Message{{Role: provider.RoleUser, Content: "hi"}}})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	return ch
}

func writeHeartbeats(ctx context.Context, w http.ResponseWriter, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flush(w)
		}
	}
}
