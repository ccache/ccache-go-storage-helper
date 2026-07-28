// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

//go:build !windows

package storagehelper

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

func createListener(endpoint string) (net.Listener, error) {
	info, err := os.Lstat(endpoint)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return nil, fmt.Errorf("refusing to remove non-socket endpoint: %s", endpoint)
		}
		if err := os.Remove(endpoint); err != nil {
			return nil, fmt.Errorf("failed to remove existing socket: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to inspect existing endpoint: %w", err)
	}
	oldMask := syscall.Umask(0077)
	defer syscall.Umask(oldMask)
	listener, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on Unix socket: %w", err)
	}
	return listener, nil
}
