// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

//go:build !windows

package storagehelper

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testStorage struct {
	putKey       []byte
	putSize      int64
	putOverwrite bool
	putPrefix    []byte
}

func (s *testStorage) Exists(key []byte) (bool, error) {
	return bytes.Equal(key, []byte("exists")), nil
}

func (s *testStorage) Get(key []byte) (io.ReadCloser, int64, bool, error) {
	switch string(key) {
	case "found":
		return io.NopCloser(strings.NewReader("value")), int64(len("value")), true, nil
	case "missing":
		return nil, 0, false, nil
	default:
		return nil, 0, false, nil
	}
}

func (s *testStorage) Put(key []byte, value io.Reader, size int64, overwrite bool) (bool, error) {
	s.putKey = append([]byte(nil), key...)
	s.putSize = size
	s.putOverwrite = overwrite
	buffer := make([]byte, 1)
	n, err := value.Read(buffer)
	if err != nil && err != io.EOF {
		return false, err
	}
	s.putPrefix = append([]byte(nil), buffer[:n]...)
	return true, nil
}

func (s *testStorage) Remove(key []byte) (bool, error) {
	return bytes.Equal(key, []byte("remove")), nil
}

func TestServerProtocol(t *testing.T) {
	storage := &testStorage{}
	endpoint := filepath.Join(t.TempDir(), "storage-helper.sock")
	server := NewServer(
		&Config{IPCEndpoint: endpoint, Diagnostics: []string{"diagnostic"}},
		"test helper 0.1.0",
		storage,
		NewLogger(""),
	)
	runErr := make(chan error, 1)
	go func() { runErr <- server.Run() }()

	conn := dialTestServer(t, endpoint)
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	if got := readTestByte(t, reader); got != ProtocolVersion {
		t.Fatalf("protocol version = %#x, want %#x", got, ProtocolVersion)
	}
	if got := readTestByte(t, reader); got != 3 {
		t.Fatalf("capability count = %d, want 3", got)
	}
	for _, want := range []byte{CapabilityGetPutRemove, CapabilityInfo, CapabilityExists} {
		if got := readTestByte(t, reader); got != want {
			t.Fatalf("capability = %#x, want %#x", got, want)
		}
	}

	writeTestRequest(t, writer, RequestGet, []byte("found"))
	if got := readTestByte(t, reader); got != ResponseOK {
		t.Fatalf("GET response = %#x, want %#x", got, ResponseOK)
	}
	if got := readTestUint64(t, reader); got != uint64(len("value")) {
		t.Fatalf("GET length = %d, want %d", got, len("value"))
	}
	if got := readTestBytes(t, reader, len("value")); string(got) != "value" {
		t.Fatalf("GET value = %q, want %q", got, "value")
	}

	writeTestRequest(t, writer, RequestGet, []byte("missing"))
	if got := readTestByte(t, reader); got != ResponseNoop {
		t.Fatalf("missing GET response = %#x, want %#x", got, ResponseNoop)
	}

	if err := writer.WriteByte(RequestPut); err != nil {
		t.Fatal(err)
	}
	writeTestKey(t, writer, []byte("put"))
	if err := writer.WriteByte(PutFlagOverwrite); err != nil {
		t.Fatal(err)
	}
	writeTestUint64(t, writer, uint64(len("payload")))
	if _, err := writer.WriteString("payload"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := readTestByte(t, reader); got != ResponseOK {
		t.Fatalf("PUT response = %#x, want %#x", got, ResponseOK)
	}
	if got, want := string(storage.putKey), "put"; got != want {
		t.Errorf("PUT key = %q, want %q", got, want)
	}
	if got, want := storage.putSize, int64(len("payload")); got != want {
		t.Errorf("PUT size = %d, want %d", got, want)
	}
	if !storage.putOverwrite {
		t.Error("PUT overwrite = false, want true")
	}
	if got, want := string(storage.putPrefix), "p"; got != want {
		t.Errorf("PUT prefix = %q, want %q", got, want)
	}

	writeTestRequest(t, writer, RequestExists, []byte("exists"))
	if got := readTestByte(t, reader); got != ResponseOK {
		t.Fatalf("EXISTS response = %#x, want %#x", got, ResponseOK)
	}
	if got := readTestByte(t, reader); got != 1 {
		t.Fatalf("EXISTS value = %d, want 1", got)
	}

	writeTestRequest(t, writer, RequestRemove, []byte("remove"))
	if got := readTestByte(t, reader); got != ResponseOK {
		t.Fatalf("REMOVE response = %#x, want %#x", got, ResponseOK)
	}

	if err := writer.WriteByte(RequestInfo); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := readTestMsg(t, reader); got != "test helper 0.1.0" {
		t.Fatalf("INFO identity = %q", got)
	}
	if got := readTestByte(t, reader); got != 1 {
		t.Fatalf("INFO diagnostic count = %d, want 1", got)
	}
	if got := readTestMsg(t, reader); got != "diagnostic" {
		t.Fatalf("INFO diagnostic = %q", got)
	}

	if err := writer.WriteByte(RequestStop); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
	if got := readTestByte(t, reader); got != ResponseOK {
		t.Fatalf("STOP response = %#x, want %#x", got, ResponseOK)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Server.Run: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Server.Run did not return after STOP")
	}
}

func dialTestServer(t *testing.T, endpoint string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", endpoint)
		if err == nil {
			return conn
		}
		if _, statErr := os.Stat(endpoint); statErr != nil && !os.IsNotExist(statErr) {
			t.Fatalf("Stat endpoint: %v", statErr)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for server listener")
	return nil
}

func writeTestRequest(t *testing.T, writer *bufio.Writer, request byte, key []byte) {
	t.Helper()
	if err := writer.WriteByte(request); err != nil {
		t.Fatal(err)
	}
	writeTestKey(t, writer, key)
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}
}

func writeTestKey(t *testing.T, writer *bufio.Writer, key []byte) {
	t.Helper()
	if err := writer.WriteByte(byte(len(key))); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(key); err != nil {
		t.Fatal(err)
	}
}

func writeTestUint64(t *testing.T, writer *bufio.Writer, value uint64) {
	t.Helper()
	var buffer [8]byte
	binary.NativeEndian.PutUint64(buffer[:], value)
	if _, err := writer.Write(buffer[:]); err != nil {
		t.Fatal(err)
	}
}

func readTestByte(t *testing.T, reader *bufio.Reader) byte {
	t.Helper()
	value, err := reader.ReadByte()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func readTestUint64(t *testing.T, reader *bufio.Reader) uint64 {
	t.Helper()
	var buffer [8]byte
	if _, err := io.ReadFull(reader, buffer[:]); err != nil {
		t.Fatal(err)
	}
	return binary.NativeEndian.Uint64(buffer[:])
}

func readTestBytes(t *testing.T, reader *bufio.Reader, length int) []byte {
	t.Helper()
	buffer := make([]byte, length)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		t.Fatal(err)
	}
	return buffer
}

func readTestMsg(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	return string(readTestBytes(t, reader, int(readTestByte(t, reader))))
}
