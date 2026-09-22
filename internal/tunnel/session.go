package tunnel

import (
	"fmt"
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

type Session struct {
	tcp net.Conn
	ws  *websocket.Conn

	mu   sync.Mutex
	once sync.Once
	done chan struct{}
}

func NewSession(c *websocket.Conn, tcp net.Conn) *Session {
	return &Session{done: make(chan struct{}), ws: c, tcp: tcp}
}

func (s *Session) Close() {
	s.once.Do(func() {
		close(s.done)

		s.mu.Lock()
		defer s.mu.Unlock()

		if s.tcp != nil {
			_ = s.tcp.Close()
		}
		if s.ws != nil {
			_ = s.ws.Close()
		}
	})
}

func (s *Session) SendWS(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ws == nil {
		return fmt.Errorf("websocket is not connected")
	}

	return s.ws.WriteMessage(websocket.BinaryMessage, data)
}

func (s *Session) ReadWS() (messageType int, p []byte, err error) {
	return s.ws.ReadMessage()
}

func (s *Session) SendTCP(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tcp == nil {
		return fmt.Errorf("tcp connection is not connected")
	}

	_, err := s.tcp.Write(data)
	return err
}

func (s *Session) ReadTCP(b []byte) (n int, err error) {
	return s.tcp.Read(b)
}
