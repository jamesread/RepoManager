package main

import (
	"os"
	"strings"
)

const defaultListen = ":8080"

// ListenAddr returns the bind address: $PORT if set, otherwise :8080.
// PORT may be a bare port ("8080") or a full address (":8080" / "0.0.0.0:8080").
func ListenAddr() string {
	if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
		return normalizePort(port)
	}
	return defaultListen
}

func normalizePort(port string) string {
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}
