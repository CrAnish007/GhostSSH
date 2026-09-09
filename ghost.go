package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const wsPath = "/ws"

var logLevel int
var logger = log.New(os.Stderr, "", log.LstdFlags)

type config struct { mode string; httpPort, sshPort, localPort int; remoteURL string }
type session struct { tcp net.Conn; ws *websocket.Conn; once sync.Once }

func (s *session) close() { s.once.Do(func() { if s.tcp != nil { _ = s.tcp.Close() }; if s.ws != nil { _ = s.ws.Close() } }) }
func infof(f string, a ...any) { if logLevel >= 1 { logger.Printf("[INFO] "+f, a...) } }
func debugf(f string, a ...any) { if logLevel >= 2 { logger.Printf("[DEBUG] "+f, a...) } }
func verbosef(f string, a ...any) { if logLevel >= 3 { logger.Printf("[VERBOSE] "+f, a...) } }

func usage() { fmt.Println(`Usage: ghost <server|client> [options]

Server:
  --port PORT       HTTP/WebSocket listen port (default 7777)
  --ssh PORT        local sshd port (default 22)

Client:
  --connect URL     remote HTTPS URL (required)
  --port PORT       local TCP port (default 8888)

Logging:
  -q, --quiet       quiet
  -v                info
  -vv               debug
  -vvv              verbose`) }

func parse(args []string) (config, error) {
	if len(args) == 0 { return config{}, fmt.Errorf("mode is required") }
	c := config{httpPort: 7777, sshPort: 22, localPort: 8888}
	fs := flag.NewFlagSet("ghost", flag.ContinueOnError); fs.SetOutput(io.Discard)
	port := fs.Int("port", 0, "port"); ssh := fs.Int("ssh", 22, "ssh port"); connect := fs.String("connect", "", "remote URL")
	quiet := fs.Bool("quiet", false, "quiet"); verbose := fs.Bool("verbose", false, "verbose"); v := fs.Bool("v", false, "info"); vv := fs.Bool("vv", false, "debug"); vvv := fs.Bool("vvv", false, "verbose")
	if err := fs.Parse(args[1:]); err != nil { return c, err }
	c.mode, c.remoteURL = args[0], *connect
	if *quiet { logLevel = 0 } else if *vvv { logLevel = 3 } else if *vv { logLevel = 2 } else if *v || *verbose { logLevel = 1 } else { logLevel = 1 }
	if c.mode == "server" { c.sshPort = *ssh; if *port != 0 { c.httpPort = *port } } else if c.mode == "client" { if *port != 0 { c.localPort = *port }; if c.remoteURL == "" { return c, fmt.Errorf("--connect is required in client mode") } } else { return c, fmt.Errorf("unknown mode %q", c.mode) }
	return c, nil
}

func wsURL(raw string) (string, error) {
	if !strings.Contains(raw, "://") { raw = "https://" + raw }
	u, err := url.Parse(raw); if err != nil { return "", err }; if u.Host == "" { return "", fmt.Errorf("URL has no host") }
	u.Scheme = "wss"; u.Path = strings.TrimRight(u.Path, "/") + wsPath; u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

func runServer(c config) error {
	mux := http.NewServeMux(); mux.HandleFunc(wsPath, func(w http.ResponseWriter, r *http.Request) { serveWS(w, r, c) })
	srv := &http.Server{Addr: fmt.Sprintf(":%d", c.httpPort), Handler: mux}
	infof("GhostSSH server listening on :%d, forwarding to sshd :%d", c.httpPort, c.sshPort)
	stop := make(chan os.Signal, 1); signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() { <-stop; ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second); defer cancel(); _ = srv.Shutdown(ctx) }()
	err := srv.ListenAndServe(); if err == http.ErrServerClosed { return nil }; return err
}

func serveWS(w http.ResponseWriter, r *http.Request, c config) {
	up := websocket.Upgrader{ReadBufferSize: 32 * 1024, WriteBufferSize: 32 * 1024, CheckOrigin: func(*http.Request) bool { return true }}
	ws, err := up.Upgrade(w, r, nil); if err != nil { debugf("websocket upgrade: %v", err); return }
	tcp, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", c.sshPort)); if err != nil { _ = ws.Close(); debugf("connect sshd: %v", err); return }
	infof("accepted tunnel"); bridge(&session{tcp: tcp, ws: ws})
}

func runClient(c config) error {
	remote, err := wsURL(c.remoteURL); if err != nil { return err }
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", c.localPort)); if err != nil { return err }
	infof("GhostSSH client listening on localhost:%d", c.localPort); infof("remote websocket: %s", remote)
	stop := make(chan os.Signal, 1); signal.Notify(stop, os.Interrupt, syscall.SIGTERM); go func() { <-stop; _ = ln.Close() }()
	for { tcp, err := ln.Accept(); if err != nil { if errorsClosed(err) { return nil }; return err }; go clientSession(tcp, remote) }
}

func clientSession(tcp net.Conn, remote string) {
	u, _ := url.Parse(remote)
	d := websocket.Dialer{ReadBufferSize: 32 * 1024, WriteBufferSize: 32 * 1024, HandshakeTimeout: 15 * time.Second, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}}
	ws, _, err := d.Dial(remote, nil); if err != nil { _ = tcp.Close(); debugf("websocket dial: %v", err); return }
	infof("client tunnel established"); bridge(&session{tcp: tcp, ws: ws})
}

func bridge(s *session) {
	var wg sync.WaitGroup; wg.Add(2)
	go func() {
		defer wg.Done(); defer s.close(); buf := make([]byte, 32*1024)
		for { n, err := s.tcp.Read(buf); if n > 0 { debugf("TCP -> WS: %d bytes", n); if err := s.ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil { return }; verbosef("forwarded TCP -> WS: %d bytes", n) }; if err != nil { return } }
	}()
	go func() {
		defer wg.Done(); defer s.close()
		for { t, data, err := s.ws.ReadMessage(); if err != nil { return }; if t != websocket.BinaryMessage && t != websocket.TextMessage { continue }; debugf("WS -> TCP: %d bytes", len(data)); if _, err := s.tcp.Write(data); err != nil { return }; verbosef("forwarded WS -> TCP: %d bytes", len(data)) }
	}()
	wg.Wait()
}

func errorsClosed(err error) bool { if err == net.ErrClosed { return true }; return strings.Contains(strings.ToLower(err.Error()), "use of closed network connection") }

func main() { c, err := parse(os.Args[1:]); if err != nil { usage(); logger.Printf("error: %v", err); os.Exit(1) }; if c.mode == "server" { err = runServer(c) } else { err = runClient(c) }; if err != nil { logger.Printf("error: %v", err); os.Exit(1) } }
