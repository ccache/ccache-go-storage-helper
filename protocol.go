// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

// Storage is implemented by a storage backend. Keys are opaque byte strings
// supplied by ccache.
type Storage interface {
	// Exists reports whether key is present.
	Exists(key []byte) (present bool, err error)
	// Get looks up key. The body return value supplies the stored value, size is
	// the number of bytes in that value and found reports whether key exists.
	// When found is false, Server ignores body and size. When found is true, body
	// must supply the complete value (Server closes it after reading). A negative
	// size means that the size is unknown, so Server buffers the value before
	// responding.
	Get(key []byte) (body io.ReadCloser, size int64, found bool, err error)
	// Put stores value under key. value is valid only for the duration of the
	// call and contains at most size bytes. Server drains any unread bytes when
	// Put returns. If overwrite is false it is only a performance hint: a backend
	// may still overwrite an existing value. Put reports whether it stored a
	// value.
	Put(key []byte, value io.Reader, size int64, overwrite bool) (bool, error)
	// Remove removes key and reports whether it was present.
	Remove(key []byte) (present bool, err error)
}

const (
	// ProtocolVersion is the ccache remote storage helper protocol version.
	ProtocolVersion = 0x01

	// CapabilityGetPutRemove identifies the GET, PUT and REMOVE capability.
	CapabilityGetPutRemove = 0x00
	// CapabilityInfo identifies the INFO capability.
	CapabilityInfo = 0x01
	// CapabilityExists identifies the EXISTS capability.
	CapabilityExists = 0x02

	// RequestGet identifies a GET request.
	RequestGet = 0x00
	// RequestPut identifies a PUT request.
	RequestPut = 0x01
	// RequestRemove identifies a REMOVE request.
	RequestRemove = 0x02
	// RequestStop identifies a STOP request.
	RequestStop = 0x03
	// RequestInfo identifies an INFO request.
	RequestInfo = 0x04
	// RequestExists identifies an EXISTS request.
	RequestExists = 0x05

	// ResponseOK identifies a successful response.
	ResponseOK = 0x00
	// ResponseNoop identifies a successful response with no operation performed.
	ResponseNoop = 0x01
	// ResponseErr identifies an error response.
	ResponseErr = 0x02

	// PutFlagOverwrite requests that PUT overwrite an existing value.
	PutFlagOverwrite = 0x01
)

func writeGreeting(w io.Writer) error {
	caps := [...]byte{CapabilityGetPutRemove, CapabilityInfo, CapabilityExists}
	if err := writeByte(w, ProtocolVersion); err != nil {
		return err
	}
	if err := writeByte(w, uint8(len(caps))); err != nil {
		return err
	}
	_, err := w.Write(caps[:])
	return err
}

func readKey(r io.Reader) ([]byte, error) {
	keyLen, err := readByte(r)
	if err != nil {
		return nil, err
	}
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, err
	}
	return key, nil
}

func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}

func readByte(r io.Reader) (byte, error) {
	var buf [1]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return buf[0], nil
}

func writeUint64(w io.Writer, value uint64) error {
	var buf [8]byte
	binary.NativeEndian.PutUint64(buf[:], value)
	_, err := w.Write(buf[:])
	return err
}

func readUint64(r io.Reader) (uint64, error) {
	var buf [8]byte
	if _, err := io.ReadFull(r, buf[:]); err != nil {
		return 0, err
	}
	return binary.NativeEndian.Uint64(buf[:]), nil
}

func writeMsg(w io.Writer, msg string) error {
	msg = strings.ToValidUTF8(msg, "\uFFFD")
	if len(msg) > 255 {
		length := 255
		for !utf8.RuneStart(msg[length]) {
			length--
		}
		msg = msg[:length]
	}
	if err := writeByte(w, uint8(len(msg))); err != nil {
		return err
	}
	_, err := io.WriteString(w, msg)
	return err
}

func writeErr(w io.Writer, msg string) error {
	if err := writeByte(w, ResponseErr); err != nil {
		return err
	}
	return writeMsg(w, msg)
}

func handleExists(r io.Reader, w io.Writer, storage Storage, logger *Logger) error {
	key, err := readKey(r)
	if err != nil {
		return err
	}
	logger.Logf("EXISTS request for key %x", key)
	found, err := storage.Exists(key)
	if err != nil {
		logger.Logf("EXISTS error: %v", err)
		return writeErr(w, err.Error())
	}
	logger.Logf("EXISTS result: %v", found)
	if err := writeByte(w, ResponseOK); err != nil {
		return err
	}
	if found {
		return writeByte(w, 1)
	}
	return writeByte(w, 0)
}

func handleGet(r io.Reader, w io.Writer, storage Storage, logger *Logger) error {
	key, err := readKey(r)
	if err != nil {
		return err
	}
	logger.Logf("GET request for key %x", key)
	valueBody, valueLen, found, err := storage.Get(key)
	if err != nil {
		logger.Logf("GET error: %v", err)
		return writeErr(w, err.Error())
	}
	if valueBody != nil {
		defer valueBody.Close()
	}
	if !found {
		logger.Logf("GET key not found")
		return writeByte(w, ResponseNoop)
	}
	if valueLen < 0 {
		value, err := io.ReadAll(valueBody)
		if err != nil {
			return err
		}
		logger.Logf("GET success (%d bytes)", len(value))
		if err := writeByte(w, ResponseOK); err != nil {
			return err
		}
		if err := writeUint64(w, uint64(len(value))); err != nil {
			return err
		}
		_, err = w.Write(value)
		return err
	}
	logger.Logf("GET success (%d bytes)", valueLen)
	if err := writeByte(w, ResponseOK); err != nil {
		return err
	}
	if err := writeUint64(w, uint64(valueLen)); err != nil {
		return err
	}
	_, err = io.Copy(w, valueBody)
	return err
}

func handleInfo(w io.Writer, cfg *Config, identity string, logger *Logger) error {
	logger.Logf("INFO request")
	if err := writeMsg(w, identity); err != nil {
		return err
	}
	diagnostics := cfg.Diagnostics
	if len(diagnostics) > 255 {
		diagnostics = diagnostics[:255]
	}
	if err := writeByte(w, uint8(len(diagnostics))); err != nil {
		return err
	}
	for _, diagnostic := range diagnostics {
		if err := writeMsg(w, diagnostic); err != nil {
			return err
		}
	}
	return nil
}

func handlePut(r io.Reader, w io.Writer, storage Storage, logger *Logger) error {
	key, err := readKey(r)
	if err != nil {
		return err
	}
	flags, err := readByte(r)
	if err != nil {
		return err
	}
	valueLen, err := readUint64(r)
	if err != nil {
		return err
	}
	if valueLen > math.MaxInt64 {
		return fmt.Errorf("value too large: %d", valueLen)
	}
	value := io.LimitReader(r, int64(valueLen))
	overwrite := flags&PutFlagOverwrite != 0
	logger.Logf("PUT request for key %x (%d bytes)", key, valueLen)
	stored, err := storage.Put(key, value, int64(valueLen), overwrite)
	_, drainErr := io.Copy(io.Discard, value)
	if drainErr != nil {
		return drainErr
	}
	if err != nil {
		logger.Logf("PUT error: %v", err)
		return writeErr(w, err.Error())
	}
	if !stored {
		logger.Logf("PUT not stored")
		return writeByte(w, ResponseNoop)
	}
	logger.Logf("PUT success")
	return writeByte(w, ResponseOK)
}

func handleRemove(r io.Reader, w io.Writer, storage Storage, logger *Logger) error {
	key, err := readKey(r)
	if err != nil {
		return err
	}
	logger.Logf("REMOVE request for key %x", key)
	removed, err := storage.Remove(key)
	if err != nil {
		logger.Logf("REMOVE error: %v", err)
		return writeErr(w, err.Error())
	}
	if !removed {
		logger.Logf("REMOVE key not found")
		return writeByte(w, ResponseNoop)
	}
	logger.Logf("REMOVE success")
	return writeByte(w, ResponseOK)
}

func processRequest(r io.Reader, w io.Writer, storage Storage, logger *Logger, cfg *Config, identity string) (bool, error) {
	request, err := readByte(r)
	if err != nil {
		return false, err
	}
	switch request {
	case RequestExists:
		err = handleExists(r, w, storage, logger)
	case RequestGet:
		err = handleGet(r, w, storage, logger)
	case RequestInfo:
		err = handleInfo(w, cfg, identity, logger)
	case RequestPut:
		err = handlePut(r, w, storage, logger)
	case RequestRemove:
		err = handleRemove(r, w, storage, logger)
	case RequestStop:
		logger.Logf("STOP request received")
		err = writeByte(w, ResponseOK)
		return true, err
	default:
		logger.Logf("Unknown request type: 0x%02x", request)
		err = writeErr(w, fmt.Sprintf("unknown request type: 0x%02x", request))
	}
	return false, err
}
