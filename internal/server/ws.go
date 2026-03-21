package server

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/Rsych/zynqel-core/internal/session"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: restrict to allowed origins for production use
		return true
	},
}

// wsMessage is the envelope for all WebSocket messages.
//
// For pty.output, Data is a base64-encoded string (terminal output may contain
// arbitrary bytes). For pty.input, Data is a base64-encoded string that will
// be decoded and forwarded to the container's stdin.
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// handleSessionStream upgrades to WebSocket and bridges it with the session's
// output broadcaster. Supports multiple viewers and reconnect with replay.
//
// Server → Client messages:
//
//	{"type": "pty.output", "data": "<base64>"}                              — terminal output
//	{"type": "session.state", "data": "running"}                            — sent on connect
//	{"type": "intercept.event", "data": {"id":"evt_...","text":"...","options":[...]}} — detected prompt
//
// Client → Server messages:
//
//	{"type": "pty.input", "data": "<base64>"}      — keyboard input
func (s *Server) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.sessions.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if sess.Status != session.StatusRunning {
		writeError(w, http.StatusConflict, "session is not running")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	// Subscribe to the session's output broadcaster.
	replay, sub, err := s.sessions.Subscribe(id)
	if err != nil {
		log.Printf("subscribe failed for session %s: %v", id, err)
		writeWSError(conn, "failed to subscribe to session output")
		return
	}
	defer s.sessions.Unsubscribe(id, sub)

	var wsMu sync.Mutex

	// Send current session state.
	sendWSJSON(conn, &wsMu, "session.state", string(sess.Status))

	// Send buffered replay (reconnect support).
	if len(replay) > 0 {
		encoded := base64.StdEncoding.EncodeToString(replay)
		sendWSJSON(conn, &wsMu, "pty.output", encoded)
	}

	// Live output: read from subscriber channel.
	go func() {
		for data := range sub.Ch {
			encoded := base64.StdEncoding.EncodeToString(data)
			sendWSJSON(conn, &wsMu, "pty.output", encoded)
		}
		// Channel closed = PTY stream ended.
		sendWSJSON(conn, &wsMu, "session.state", "stopped")
	}()

	// Intercept events: detected CLI prompts.
	go func() {
		for prompt := range sub.Events {
			sendWSEvent(conn, &wsMu, "intercept.event", prompt)
		}
	}()

	// Input loop: WebSocket → PTY.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("websocket read error for session %s: %v", id, err)
			}
			return
		}

		var msg wsMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("invalid websocket message: %v", err)
			continue
		}

		switch msg.Type {
		case "pty.input":
			var b64Input string
			if err := json.Unmarshal(msg.Data, &b64Input); err != nil {
				log.Printf("invalid pty.input data: %v", err)
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(b64Input)
			if err != nil {
				log.Printf("invalid pty.input base64: %v", err)
				continue
			}
			if err := s.sessions.WriteInput(id, decoded); err != nil {
				log.Printf("pty write error for session %s: %v", id, err)
				return
			}
		default:
			log.Printf("unknown message type: %s", msg.Type)
		}
	}
}

// sendWSJSON sends a typed JSON message over the WebSocket connection.
func sendWSJSON(conn *websocket.Conn, mu *sync.Mutex, msgType string, data string) {
	raw, _ := json.Marshal(data)
	msg := wsMessage{Type: msgType, Data: raw}
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal ws message: %v", err)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Printf("websocket write error: %v", err)
	}
}

// sendWSEvent sends a typed message with a struct data payload.
func sendWSEvent(conn *websocket.Conn, mu *sync.Mutex, msgType string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		log.Printf("failed to marshal event data: %v", err)
		return
	}
	msg := wsMessage{Type: msgType, Data: raw}
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal ws event: %v", err)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Printf("websocket write error: %v", err)
	}
}

func writeWSError(conn *websocket.Conn, message string) {
	raw, _ := json.Marshal(message)
	msg := wsMessage{Type: "error", Data: raw}
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal ws error message: %v", err)
		return
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		log.Printf("failed to send ws error message: %v", err)
	}
}
