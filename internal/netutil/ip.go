package netutil

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP resolves the client IP, matching PHP mainLib::getIP behavior.
func ClientIP(r *http.Request) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}

	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" && isCloudflareIP(remote) {
		return cfIP
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" && isLocalhost(remote) {
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	return remote
}

func isLocalhost(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

