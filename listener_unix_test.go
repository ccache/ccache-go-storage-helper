// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

//go:build !windows

package storagehelper

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCreateListenerRefusesRegularFile(t *testing.T) {
	endpoint := filepath.Join(t.TempDir(), "endpoint")
	if err := os.WriteFile(endpoint, []byte("keep me"), 0600); err != nil {
		t.Fatal(err)
	}

	listener, err := createListener(endpoint)
	if listener != nil {
		listener.Close()
		t.Fatal("createListener unexpectedly returned a listener")
	}
	if err == nil || !strings.Contains(err.Error(), "non-socket") {
		t.Fatalf("createListener error = %v, want non-socket error", err)
	}
	contents, err := os.ReadFile(endpoint)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(contents) != "keep me" {
		t.Errorf("file contents = %q, want %q", contents, "keep me")
	}
}

func TestCreateListenerReplacesStaleSocket(t *testing.T) {
	endpoint := filepath.Join(t.TempDir(), "endpoint")
	fd, err := syscall.Socket(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrUnix{Name: endpoint}); err != nil {
		syscall.Close(fd)
		t.Fatal(err)
	}
	if err := syscall.Close(fd); err != nil {
		t.Fatal(err)
	}

	listener, err := createListener(endpoint)
	if err != nil {
		t.Fatalf("createListener: %v", err)
	}
	defer listener.Close()
	info, err := os.Lstat(endpoint)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Errorf("endpoint mode = %v, want socket", info.Mode())
	}
}
