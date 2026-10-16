// Package config loads the bootstrap server's runtime settings.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	DefaultListenAddr = "127.0.0.1:8080"
	DefaultDataDir    = "../shared/data"
)

type Config struct {
	ListenAddr string
	// DatabaseURL is optional; when empty the server runs diagnostics only.
	DatabaseURL string
	// DataDir is shared/data; the default fits running from server/.
	DataDir string
}

// Load reads the process environment. No dotenv file is loaded automatically.
func Load() (Config, error) {
	addr := strings.TrimSpace(os.Getenv("JIANGHU_LISTEN_ADDR"))
	if addr == "" {
		addr = DefaultListenAddr
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid JIANGHU_LISTEN_ADDR: %w", err)
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return Config{}, fmt.Errorf("JIANGHU_LISTEN_ADDR port must be between 1 and 65535")
	}
	dataDir := strings.TrimSpace(os.Getenv("JIANGHU_DATA_DIR"))
	if dataDir == "" {
		dataDir = DefaultDataDir
	}
	return Config{ListenAddr: addr, DatabaseURL: strings.TrimSpace(os.Getenv("JIANGHU_DATABASE_URL")), DataDir: dataDir}, nil
}
