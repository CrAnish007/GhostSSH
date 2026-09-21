package server

import (
	"context"
	"fmt"
	"gossh/internal/log"
	"gossh/internal/tunnel"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
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
)

type GoSSHServer struct {
	conf   GoSSHServerConfiguration
	logger log.Logger
}

type GoSSHServerConfiguration struct {
	Port    int
	SSHPort int
	Timeout time.Duration
}

func NewGoSSHServer(config GoSSHServerConfiguration, logger log.Logger) GoSSHServer {
	return GoSSHServer{
		conf:   config,
		logger: logger,
	}
}

func (gossh GoSSHServer) Run() error {
	addr := fmt.Sprintf(":%d", gossh.conf.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		gossh.handleServerWebSocket(w, r)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	gossh.logger.Info("gossh started in SERVER mode")
	gossh.logger.Info("HTTP/WebSocket port: %d", gossh.conf.Port)
	gossh.logger.Info("SSH port: %d", gossh.conf.SSHPort)
	gossh.logger.Info("Checking SSH connection")

	sshUrl := fmt.Sprintf("127.0.0.1:%d", gossh.conf.SSHPort)
	conn, err := net.DialTimeout("tcp", sshUrl, gossh.conf.Timeout)

	if err != nil {
		gossh.logger.Error("SSH server is down or unreachable: %v", err)
		return err // this is unrecoverable
	} else {
		gossh.logger.Info("SSH server port is open and reachable")
		conn.Close()
	}

	gossh.logger.Info("Listening on http://localhost:%d", gossh.conf.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		gossh.logger.Info("Shutting down server...")
		ctx, cancel := shutdownContext()
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	err = server.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (gossh GoSSHServer) handleServerWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ws" {
		http.NotFound(w, r)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  ReadBufSize,
		WriteBufferSize: WriteBufSize,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		gossh.logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	sshAddr := fmt.Sprintf("localhost:%d", gossh.conf.SSHPort)
	tcp, err := net.Dial("tcp", sshAddr)
	if err != nil {
		gossh.logger.Error("Cannot connect to sshd on %s: %v", sshAddr, err)
		_ = ws.WriteMessage(websocket.TextMessage, []byte(sshUnavailable))
		return
	}
	gossh.logger.Info("Connected to sshd successfully")

	session := tunnel.NewSession(ws, tcp)
	gossh.logger.Info("WebSocket connection established")

	gossh.runServerSession(session)
}

func (gossh GoSSHServer) runServerSession(s *tunnel.Session) {
	defer s.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// WebSocket -> SSH
	go func() {
		defer wg.Done()

		for {
			messageType, data, err := s.ReadWS()
			if err != nil {
				gossh.logger.Debug("WebSocket read ended: %v", err)
				return
			}

			if messageType != websocket.BinaryMessage && messageType != websocket.TextMessage {
				continue
			}

			gossh.logger.Debug("Data from websocket in SERVER mode: %d bytes", len(data))

			if err := s.SendTCP(data); err != nil {
				gossh.logger.Debug("TCP write failed: %v", err)
				return
			}

			gossh.logger.Trace("Forwarded %d bytes WS -> TCP", len(data))
		}
	}()

	// SSH -> WebSocket
	go func() {
		defer wg.Done()

		buf := make([]byte, ReadBufSize)

		for {
			n, err := s.ReadTCP(buf)
			if n > 0 {
				gossh.logger.Debug("SSH read %d bytes", n)

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

	wg.Wait()
	gossh.logger.Info("Client disconnected")
}

func shutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
