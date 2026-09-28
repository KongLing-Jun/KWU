//go:build testdiag

// Package testdiag exposes diagnostics only to builds that explicitly opt in
// with the testdiag build tag.
package testdiag

import (
	"context"
	"errors"
	"net"
	"net/http"
	httppprof "net/http/pprof"
	"sync"
	"time"
)

// Server is a loopback-only pprof server owned by a test harness.
type Server struct {
	listener net.Listener
	server   *http.Server
	done     chan error

	closeOnce sync.Once
	closeErr  error
}

// StartPprof starts pprof on an OS-assigned loopback port.
func StartPprof() (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", httppprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", httppprof.Trace)
	for _, name := range []string{"allocs", "block", "goroutine", "heap", "mutex", "threadcreate"} {
		mux.Handle("/debug/pprof/"+name, httppprof.Handler(name))
	}

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	diagnostics := &Server{
		listener: listener,
		server:   httpServer,
		done:     make(chan error, 1),
	}
	go func() {
		serveErr := httpServer.Serve(listener)
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		diagnostics.done <- serveErr
		close(diagnostics.done)
	}()
	return diagnostics, nil
}

// URL returns the loopback base URL selected for this test process.
func (s *Server) URL() string {
	return "http://" + s.listener.Addr().String()
}

// Close shuts down the diagnostics server and waits for its serving goroutine.
func (s *Server) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		shutdownErr := s.server.Shutdown(ctx)
		if shutdownErr != nil {
			_ = s.server.Close()
		}
		serveErr := <-s.done
		s.closeErr = errors.Join(shutdownErr, serveErr)
	})
	return s.closeErr
}
