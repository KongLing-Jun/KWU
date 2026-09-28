package connector

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DevFixedFixture is deliberately wire-only. The caller supplies a complete
// raw Catalog envelope and an immutable URL-path -> ZIP byte mapping. This
// helper never owns connector metadata and never parses, installs, or registers
// capabilities.
type DevFixedFixture struct {
	Catalog  []byte
	Packages map[string][]byte
}

type DevFixedServer struct {
	listener net.Listener
	server   *http.Server
	baseURL  string
	once     sync.Once
}

func StartDevFixedServer(listener net.Listener, fixture DevFixedFixture) (*DevFixedServer, error) {
	if listener == nil || len(fixture.Catalog) == 0 {
		return nil, errors.New("connector dev-fixed: listener and raw catalog are required")
	}
	address := listener.Addr().String()
	if !strings.HasPrefix(address, "127.0.0.1:") {
		return nil, errors.New("connector dev-fixed: listener must bind 127.0.0.1")
	}
	catalog := append([]byte(nil), fixture.Catalog...)
	packages := make(map[string][]byte, len(fixture.Packages))
	for path, raw := range fixture.Packages {
		if !strings.HasPrefix(path, "/") || !strings.HasSuffix(path, "/package.zip") || len(raw) == 0 {
			return nil, errors.New("connector dev-fixed: invalid immutable package mapping")
		}
		packages[path] = append([]byte(nil), raw...)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(catalog)
	})
	for path, raw := range packages {
		path, raw := path, raw
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Cache-Control", "public, immutable")
			w.Header().Set("Content-Length", stringValue(len(raw)))
			_, _ = w.Write(raw)
		})
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	out := &DevFixedServer{listener: listener, server: server, baseURL: "http://" + address}
	go func() { _ = server.Serve(listener) }()
	return out, nil
}

func (s *DevFixedServer) Source() CatalogSource {
	return HTTPSource{URL: s.baseURL + "/catalog.json", Client: strictHTTPClient()}
}

func (s *DevFixedServer) BaseURL() string { return s.baseURL }

func (s *DevFixedServer) Close() error {
	var err error
	s.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err = s.server.Shutdown(ctx)
	})
	return err
}

func stringValue(value int) string {
	if value == 0 {
		return "0"
	}
	var buffer [32]byte
	i := len(buffer)
	for value > 0 {
		i--
		buffer[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[i:])
}
