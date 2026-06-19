package basremote

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type socketService struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	buffer string
	emit   func(event string, data interface{})
}

func newSocketService(emit func(string, interface{})) *socketService {
	return &socketService{emit: emit}
}

// start attempts to connect to ws://127.0.0.1:<port> retrying up to 60 times.
func (s *socketService) start(port int) error {
	addr := fmt.Sprintf("ws://127.0.0.1:%d", port)
	for attempt := 1; attempt <= 60; attempt++ {
		conn, _, err := websocket.DefaultDialer.Dial(addr, nil)
		if err == nil {
			s.conn = conn
			s.opened()
			return nil
		}
		if attempt == 60 {
			return ErrSocketNotConnected
		}
		time.Sleep(time.Second)
	}
	return ErrSocketNotConnected
}

func (s *socketService) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

func (s *socketService) opened() {
	s.emit("socket_open", nil)
	go s.listen()
}

func (s *socketService) listen() {
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			break
		}
		s.processData(string(data))
	}
	s.closed()
}

func (s *socketService) processData(data string) {
	s.mu.Lock()
	combined := s.buffer + data
	parts := strings.Split(combined, messageSeparator)
	// The last part is either empty or an incomplete message.
	s.buffer = parts[len(parts)-1]
	complete := parts[:len(parts)-1]
	s.mu.Unlock()

	for _, raw := range complete {
		if raw == "" {
			continue
		}
		msg, err := MessageFromJSON(raw)
		if err != nil {
			continue
		}
		s.emit("message_received", msg)
	}
}

func (s *socketService) closed() {
	s.emit("socket_close", nil)
	_ = s.close()
}

// send serialises msg, appends the separator and writes it to the WebSocket.
func (s *socketService) send(msg Message) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return 0, ErrSocketNotConnected
	}
	packet := msg.ToJSON() + messageSeparator
	if err := s.conn.WriteMessage(websocket.TextMessage, []byte(packet)); err != nil {
		return 0, err
	}
	s.emit("message_sent", msg)
	return msg.ID, nil
}

func (s *socketService) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn = nil
	return err
}
