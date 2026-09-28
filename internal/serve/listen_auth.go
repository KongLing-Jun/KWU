package serve

import (
	"fmt"
	"net"
	"strings"
)

func (s *Server) validateListenAuth(addr string) error {
	if s.AuthMode() != "none" || listenAddrIsLoopback(addr) {
		return nil
	}
	return fmt.Errorf("serve auth is required when listening on non-loopback address %q; set --auth token or --auth password, or bind to 127.0.0.1", addr)
}

func listenAddrIsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return true
	}
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
