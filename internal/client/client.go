package client

import (
	"crypto/tls"
	"fmt"
	"gossh/internal/log"
	"gossh/internal/tunnel"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// TODO: do we want these to be configurable? Then add to config
// Or are they shared invariants between client/server? Move to a shared package for using in both client.go and server.go
const (
	ReadBufSize    = 32 * 1024
	WriteBufSize   = 32 * 1024
	sshUnavailable = "SSH_UNAVAILABLE"

	HandShakeTime = 15
)

type GoSSHClient struct {
	conf   GoSSHClientConfiguration
	logger log.Logger
}

type GoSSHClientConfiguration struct {
	Port            int
	RawWebsocketURL string
}

func NewGoSSHClient(config GoSSHClientConfiguration, logger log.Logger) GoSSHClient {
	return GoSSHClient{
		conf:   config,
		logger: logger,
	}
}

func (gossh GoSSHClient) Run() error {
	listenAddr := fmt.Sprintf("localhost:%d", gossh.conf.Port)

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", listenAddr, err)
	}
	defer ln.Close()

	wsURL, err := convertToWSS(gossh.conf.RawWebsocketURL)
	if err != nil {
		return err
	}

	gossh.logger.Info("gossh started in CLIENT mode")
	gossh.logger.Info("Remote URL: %s", wsURL)
	gossh.logger.Info("Listening on %s", listenAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		gossh.logger.Info("Shutting down client...")
		_ = ln.Close()
	}()

	for {
		tcp, err := ln.Accept()
		if err != nil {
			if isClosedNetworkError(err) {
				return nil
			}
			return err
		}

		gossh.logger.Info("Client accepted via TCP successfully")

		go gossh.handleClientTCP(tcp, wsURL, gossh.conf.RawWebsocketURL)
	}
}

func (gossh GoSSHClient) handleClientTCP(tcp net.Conn, wsURL string, originalURL string) {

	host := tlsHost(originalURL)

	dialer := websocket.Dialer{
		ReadBufferSize:   ReadBufSize,
		WriteBufferSize:  WriteBufSize,
		HandshakeTimeout: HandShakeTime * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		},
	}

	gossh.logger.Info("Connecting to remote WebSocket: %s", wsURL)

	ws, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		gossh.logger.Error("Failed to create WebSocket connection: %v", err)
		return
	}

	session := tunnel.NewSession(ws, tcp)
	gossh.logger.Info("WebSocket connection successfully established")

	gossh.runClientSession(session)
}

func (gossh GoSSHClient) runClientSession(s *tunnel.Session) {
	defer s.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Local SSH client -> WebSocket
	go func() {
		defer wg.Done()

		buf := make([]byte, ReadBufSize)

		for {
			n, err := s.ReadTCP(buf)

			if n > 0 {
				gossh.logger.Debug("Client read %d bytes", n)

				if err := s.SendWS(buf[:n]); err != nil {
					gossh.logger.Debug("WebSocket write failed: %v", err)
					return
				}

				gossh.logger.Trace("Forwarded %d bytes TCP -> WS", n)
			}

			if err != nil {
				if err != io.EOF {
					gossh.logger.Debug("TCP read ended: %v", err)
				}
				return
			}
		}
	}()

	// WebSocket -> local SSH client
	go func() {
		defer wg.Done()

		for {
			messageType, data, err := s.ReadWS()
			if err != nil {
				gossh.logger.Debug("WebSocket read ended: %v", err)
				return
			}

			if messageType == websocket.TextMessage &&
				string(data) == sshUnavailable {

				gossh.logger.Error("SSH server is unavailable")

				s.Close()
				return
			}

			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}

			gossh.logger.Debug("Data from websocket in CLIENT mode: %d bytes", len(data))

			if err := s.SendTCP(data); err != nil {
				gossh.logger.Debug("TCP write failed: %v", err)
				return
			}

			gossh.logger.Trace("Forwarded %d bytes WS -> TCP", len(data))
		}
	}()

	wg.Wait()
	gossh.logger.Info("Client disconnected")
}

// TODO: these are copied directly from cmd/gossh/main.go
// find better packaging of these helper/until below

func tlsHost(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	return host
}

// convertToWSS Turns the remote URL into websocket format
func convertToWSS(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty URL")
	}

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	host := u.Host
	if host == "" {
		return "", fmt.Errorf("URL has no host")
	}

	// WebSocket URL normalization
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	if !strings.HasSuffix(u.Path, "/ws") {
		u.Path = strings.TrimRight(u.Path, "/") + "/ws"
	}

	u.RawQuery = ""
	u.Fragment = ""

	return u.String(), nil
}

func isClosedNetworkError(err error) bool {
	if err == net.ErrClosed {
		return true
	}

	s := strings.ToLower(err.Error())
	return strings.Contains(s, "use of closed network connection")
}
