// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

//go:build windows

package storagehelper

import (
	"fmt"
	"net"

	"github.com/Microsoft/go-winio"
)

func createListener(endpoint string) (net.Listener, error) {
	listener, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on named pipe: %w", err)
	}
	return listener, nil
}
