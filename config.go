// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Attribute is a backend-specific key-value attribute supplied by ccache.
type Attribute struct {
	// Key is the attribute name without the leading @ character.
	Key string
	// Value is the attribute value.
	Value string
}

// Config contains settings common to all storage helpers. Backends interpret
// URL and Attributes themselves and append any INFO diagnostics to Diagnostics.
type Config struct {
	// IPCEndpoint is the Unix socket path or Windows named-pipe name on which
	// the helper listens.
	IPCEndpoint string
	// URL is the unparsed CRSH_URL value.
	URL string
	// IdleTimeout is the duration after which an idle helper will exit. Zero
	// disables the idle timeout.
	IdleTimeout time.Duration
	// Attributes contains the backend-specific attributes supplied by ccache
	// (order preserved).
	Attributes []Attribute
	// Diagnostics are sent in response to an INFO request.
	Diagnostics []string
}

// ParseConfig parses the CRSH_* environment variables common to all storage
// helpers. It validates CRSH_URL, CRSH_IDLE_TIMEOUT and CRSH_NUM_ATTR, but
// deliberately leaves URL and Attributes for the backend to interpret.
func ParseConfig(logger *Logger) (*Config, error) {
	ipcEndpoint := os.Getenv("CRSH_IPC_ENDPOINT")
	if runtime.GOOS == "windows" {
		ipcEndpoint = `\\.\pipe\` + ipcEndpoint
	}
	logger.Logf("IPC endpoint: %s", ipcEndpoint)

	cfg := &Config{IPCEndpoint: ipcEndpoint}

	cfg.URL = os.Getenv("CRSH_URL")
	if cfg.URL == "" {
		return nil, fmt.Errorf("CRSH_URL not set")
	}

	idleTimeout := os.Getenv("CRSH_IDLE_TIMEOUT")
	if idleTimeout == "" {
		idleTimeout = "0"
	}
	timeoutSecs, err := strconv.Atoi(idleTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_IDLE_TIMEOUT: %w", err)
	}
	if timeoutSecs < 0 {
		return nil, fmt.Errorf("invalid CRSH_IDLE_TIMEOUT: %d", timeoutSecs)
	}
	cfg.IdleTimeout = time.Duration(timeoutSecs) * time.Second
	logger.Logf("Idle timeout: %s", cfg.IdleTimeout)

	numAttr := os.Getenv("CRSH_NUM_ATTR")
	if numAttr == "" {
		numAttr = "0"
	}
	n, err := strconv.Atoi(numAttr)
	if err != nil {
		return nil, fmt.Errorf("invalid CRSH_NUM_ATTR: %w", err)
	}
	if n < 0 {
		return nil, fmt.Errorf("invalid CRSH_NUM_ATTR: %d", n)
	}
	for i := 0; i < n; i++ {
		attribute := Attribute{
			Key:   os.Getenv(fmt.Sprintf("CRSH_ATTR_KEY_%d", i)),
			Value: os.Getenv(fmt.Sprintf("CRSH_ATTR_VALUE_%d", i)),
		}
		logger.Logf("Attribute: %s=%s", attribute.Key, attribute.Value)
		cfg.Attributes = append(cfg.Attributes, attribute)
	}

	return cfg, nil
}
