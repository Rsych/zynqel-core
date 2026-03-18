package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/Rsych/zynqel-core/internal/pty"
	"github.com/Rsych/zynqel-core/internal/session"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Dev tool — allow all origins
	},
}

// wsMessage is the envelope for all WebSocket messages.
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// handleSessionStream upgrades to WebSocket and bridges it with the container PTY.
//
// Server → Client messages:
//
//	{"type": "pty.output", "data": "..."}  — terminal output (raw string)
//	{"type": "session.state", "data": "running"} — sent on connect
//
// Client → Server messages:
//
//	{"type": "pty.input", "data": "..."}   — keyboard input (raw string)
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
	defer conn.Close()

	ptyConn, err := s.sessions.Attach(r.Context(), id)
	if err != nil {
		log.Printf("pty attach failed for session %s: %v", id, err)
		writeWSError(conn, "failed to attach to container")
		return
	}

	stream := pty.New(ptyConn)
	defer stream.Close()

	var wsMu sync.Mutex

	// Send current session state on connect.
	sendWSMessage(conn, &wsMu, "session.state", string(sess.Status))

	// Read loop: PTY → WebSocket
	go stream.Run(r.Context(), func(data []byte) {
		sendWSMessage(conn, &wsMu, "pty.output", string(data))
	})

	// Write loop: WebSocket → PTY
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
			var input string
			if err := json.Unmarshal(msg.Data, &input); err != nil {
				log.Printf("invalid pty.input data: %v", err)
				continue
			}
			if err := stream.Write([]byte(input)); err != nil {
				log.Printf("pty write error for session %s: %v", id, err)
				return
			}
		default:
			log.Printf("unknown message type: %s", msg.Type)
		}
	}
}

func sendWSMessage(conn *websocket.Conn, mu *sync.Mutex, msgType string, data string) {
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

func writeWSError(conn *websocket.Conn, message string) {
	raw, _ := json.Marshal(message)
	msg := wsMessage{Type: "error", Data: raw}
	payload, _ := json.Marshal(msg)
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}
