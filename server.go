// SPDX-License-Identifier: MIT
// Copyright 2026 Joel Rosdahl

package storagehelper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

const (
	ipcBufferSize    = 64 << 10
	acceptRetryDelay = 100 * time.Millisecond
)

// Server serves the ccache remote storage helper protocol over the configured
// IPC endpoint.
type Server struct {
	config    *Config
	identity  string
	logger    *Logger
	storage   Storage
	listener  net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	idleTimer *time.Timer
	timerMu   sync.Mutex
}

// NewServer creates a Server using config, identity, storage, and logger. The
// identity is returned to ccache in response to INFO requests.
func NewServer(config *Config, identity string, storage Storage, logger *Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		config:   config,
		identity: identity,
		logger:   logger,
		storage:  storage,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Run listens on Config.IPCEndpoint and blocks until the helper receives a STOP
// request or its idle timeout expires. It returns an error if the IPC listener
// cannot be created.
func (s *Server) Run() error {
	listener, err := createListener(s.config.IPCEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener
	defer s.listener.Close()

	s.logger.Logf("Server listening on %s", s.config.IPCEndpoint)
	s.resetIdleTimer()
	go s.acceptLoop()
	<-s.ctx.Done()
	s.logger.Logf("Server shutting down")
	s.listener.Close()
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return
			default:
				s.logger.Logf("Accept error: %v", err)
				select {
				case <-s.ctx.Done():
					return
				case <-time.After(acceptRetryDelay):
				}
			}
			continue
		}

		s.logger.Logf("Client connected")
		s.resetIdleTimer()
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	writer := bufio.NewWriterSize(conn, ipcBufferSize)
	if err := writeGreeting(writer); err != nil {
		s.logger.Logf("Failed to send greeting: %v", err)
		return
	}
	if err := writer.Flush(); err != nil {
		s.logger.Logf("Failed to send greeting: %v", err)
		return
	}

	reader := bufio.NewReaderSize(conn, ipcBufferSize)
	for {
		shouldStop, err := processRequest(reader, writer, s.storage, s.logger, s.config, s.identity)
		if err == nil {
			err = writer.Flush()
		}
		if err != nil {
			if err == io.EOF {
				s.logger.Logf("Client disconnected")
			} else {
				s.logger.Logf("Request processing error: %v", err)
			}
			return
		}
		if shouldStop {
			s.logger.Logf("Stop requested, shutting down")
			s.cancel()
			return
		}

		// Intentionally reset the idle timer only after a request completes. Ccache
		// cancels slow requests before IdleTimeout expires.
		s.resetIdleTimer()
	}
}

func (s *Server) resetIdleTimer() {
	if s.config.IdleTimeout == 0 {
		return
	}
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(s.config.IdleTimeout, func() {
		s.logger.Logf("Idle timeout reached, shutting down")
		s.cancel()
	})
}
