package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/session"
	"github.com/Rsych/zynqel-core/internal/sessionlog"
	"github.com/gorilla/websocket"
)

type integrationPTY struct {
	mu     sync.Mutex
	readCh chan []byte
	closed chan struct{}
	writes [][]byte
}

func newIntegrationPTY() *integrationPTY {
	return &integrationPTY{
		readCh: make(chan []byte, 8),
		closed: make(chan struct{}),
	}
}

func (p *integrationPTY) Read(b []byte) (int, error) {
	select {
	case data := <-p.readCh:
		n := copy(b, data)
		return n, nil
	case <-p.closed:
		return 0, io.EOF
	}
}

func (p *integrationPTY) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	cp := append([]byte(nil), b...)
	p.writes = append(p.writes, cp)
	return len(b), nil
}

func (p *integrationPTY) Close() error {
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
	return nil
}

type integrationSandbox struct {
	mu      sync.Mutex
	nextID  int
	conns   map[string]*integrationPTY
	volumes map[string]sandbox.VolumeInfo
}

func newIntegrationSandbox() *integrationSandbox {
	return &integrationSandbox{
		conns:   make(map[string]*integrationPTY),
		volumes: make(map[string]sandbox.VolumeInfo),
	}
}

func (s *integrationSandbox) Create(_ context.Context, spec sandbox.Spec) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	id := fmt.Sprintf("container-%d", s.nextID)
	s.conns[id] = newIntegrationPTY()
	if spec.VolumeName != "" {
		s.volumes[spec.VolumeName] = sandbox.VolumeInfo{
			Name:      spec.VolumeName,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Image:     spec.Image,
			Agent:     spec.Labels["agent"],
		}
	}
	return id, nil
}

func (s *integrationSandbox) Start(context.Context, string) error { return nil }

func (s *integrationSandbox) Stop(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.conns[id]; ok {
		_ = c.Close()
	}
	return nil
}

func (s *integrationSandbox) Remove(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, id)
	return nil
}

func (s *integrationSandbox) Attach(_ context.Context, id string) (sandbox.PTYConn, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.conns[id]
	if !ok {
		return nil, fmt.Errorf("container not found: %s", id)
	}
	return c, nil
}

func (s *integrationSandbox) Exec(_ context.Context, _ string, _ []string) (sandbox.PTYConn, error) {
	return newIntegrationPTY(), nil
}

func (s *integrationSandbox) ExecRun(context.Context, string, []string) ([]byte, error) {
	return nil, nil
}
func (s *integrationSandbox) Resize(context.Context, string, int, int) error { return nil }
func (s *integrationSandbox) Stats(context.Context, string) (*sandbox.ContainerStats, error) {
	return &sandbox.ContainerStats{CPUPercent: 3.2, MemoryMB: 32, MemoryMax: 512}, nil
}
func (s *integrationSandbox) Commit(context.Context, string, string) error { return nil }
func (s *integrationSandbox) ImageExists(_ context.Context, _ string) bool { return false }
func (s *integrationSandbox) BuildImage(context.Context, string, string) error {
	return nil
}
func (s *integrationSandbox) ListVolumes(_ context.Context, prefix string) ([]sandbox.VolumeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []sandbox.VolumeInfo
	for _, v := range s.volumes {
		if strings.HasPrefix(v.Name, prefix) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *integrationSandbox) RemoveVolume(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.volumes, name)
	return nil
}
func (s *integrationSandbox) CopyVolume(_ context.Context, srcName, dstName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	src, ok := s.volumes[srcName]
	if !ok {
		return fmt.Errorf("source volume not found")
	}
	src.Name = dstName
	s.volumes[dstName] = src
	return nil
}
func (s *integrationSandbox) TagImage(context.Context, string, string) error { return nil }
func (s *integrationSandbox) RemoveImage(context.Context, string) error      { return nil }
func (s *integrationSandbox) addVolume(name string) {
	s.volumes[name] = sandbox.VolumeInfo{Name: name, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
}

func newHarness(t *testing.T) (*httptest.Server, *integrationSandbox) {
	t.Helper()
	sb := newIntegrationSandbox()
	agents := agentcfg.NewStore(filepath.Join(t.TempDir(), "agents.json"))
	logStore, err := sessionlog.NewStore(t.TempDir(), false, 1)
	if err != nil {
		t.Fatalf("new log store: %v", err)
	}
	sm := session.NewManager(sb, policy.DefaultPolicy(), agents, logStore)
	return httptest.NewServer(New(sm, agents, sb, logStore, nil)), sb
}

func postJSON(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	return resp
}

func createSession(t *testing.T, baseURL string) string {
	t.Helper()
	resp := postJSON(t, baseURL+"/sessions", `{"agent":"shell"}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("create session status=%d body=%s", resp.StatusCode, string(raw))
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode session: %v", err)
	}
	if got.ID == "" {
		t.Fatalf("empty session id")
	}
	return got.ID
}

func TestHealthAndRequestID(t *testing.T) {
	ts, _ := newHarness(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("X-Request-ID", "req-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("X-Request-ID"); got != "req-123" {
		t.Fatalf("x-request-id = %q, want %q", got, "req-123")
	}
}

func TestSessionCRUDAndStats(t *testing.T) {
	ts, _ := newHarness(t)
	defer ts.Close()

	id := createSession(t, ts.URL)

	resp := postJSON(t, ts.URL+"/sessions/"+id+"/stop", `{}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stop status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp = postJSON(t, ts.URL+"/sessions/"+id+"/restart", `{}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("restart status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/sessions", nil)
	if err != nil {
		t.Fatalf("new list request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/sessions/"+id+"/stats", nil)
	if err != nil {
		t.Fatalf("new stats request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stats request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("stats status = %d, want %d (old id after restart)", resp.StatusCode, http.StatusNotFound)
	}

	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/sessions/"+id, nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("delete status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestAgentsAndWorkspacesAndHistory(t *testing.T) {
	ts, sb := newHarness(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/agents", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list agents status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp = postJSON(t, ts.URL+"/agents", `{"name":"claude","command":["x"]}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("create reserved agent status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	resp = postJSON(t, ts.URL+"/agents", `{"name":"custom","command":["echo","hi"]}`)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create custom agent status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/agents/custom", bytes.NewBufferString(`{"command":["echo","bye"]}`))
	if err != nil {
		t.Fatalf("new update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update agent: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/agents/custom", nil)
	if err != nil {
		t.Fatalf("new delete request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete agent: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	sb.addVolume("zynqel-ws-old")
	req, err = http.NewRequest(http.MethodGet, ts.URL+"/workspaces", nil)
	if err != nil {
		t.Fatalf("new workspace list request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("list workspaces: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("workspaces status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err = http.NewRequest(http.MethodPut, ts.URL+"/workspaces/old", strings.NewReader(`{"id":"new"}`))
	if err != nil {
		t.Fatalf("new rename request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("rename workspace: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename workspace status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/workspaces/new", nil)
	if err != nil {
		t.Fatalf("new delete workspace request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete workspace: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete workspace status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	createdID := createSession(t, ts.URL)
	req, err = http.NewRequest(http.MethodDelete, ts.URL+"/sessions/"+createdID, nil)
	if err != nil {
		t.Fatalf("new delete session request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("delete session: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete session status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	req, err = http.NewRequest(http.MethodGet, ts.URL+"/session-history", nil)
	if err != nil {
		t.Fatalf("new history list request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("history list: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("history list status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSessionWebSocketLifecycle(t *testing.T) {
	ts, _ := newHarness(t)
	defer ts.Close()

	id := createSession(t, ts.URL)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/" + id + "/stream"

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			t.Fatalf("dial failed: %v (status=%d)", err, resp.StatusCode)
		}
		t.Fatalf("dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read first ws message: %v", err)
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(msg, &envelope); err != nil {
		t.Fatalf("unmarshal ws message: %v", err)
	}
	if string(envelope["type"]) != `"session.state"` {
		t.Fatalf("first ws type = %s, want %q", string(envelope["type"]), "session.state")
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"pty.input","data":"`+base64.StdEncoding.EncodeToString([]byte("ls\n"))+`"}`)); err != nil {
		t.Fatalf("write pty.input: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`not-json`)); err != nil {
		t.Fatalf("write invalid message: %v", err)
	}

	stopResp := postJSON(t, ts.URL+"/sessions/"+id+"/stop", `{}`)
	_ = stopResp.Body.Close()
}

func TestSessionWebSocketNotFoundAndNotRunning(t *testing.T) {
	ts, _ := newHarness(t)
	defer ts.Close()

	notFoundURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/not-found/stream"
	_, resp, err := websocket.DefaultDialer.Dial(notFoundURL, nil)
	if err == nil {
		t.Fatalf("expected not-found dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %v, want %d", statusCodeOrZero(resp), http.StatusNotFound)
	}

	id := createSession(t, ts.URL)
	stopResp := postJSON(t, ts.URL+"/sessions/"+id+"/stop", `{}`)
	_ = stopResp.Body.Close()

	notRunningURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/" + id + "/stream"
	_, resp, err = websocket.DefaultDialer.Dial(notRunningURL, nil)
	if err == nil {
		t.Fatalf("expected not-running dial error")
	}
	if resp == nil || resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %v, want %d", statusCodeOrZero(resp), http.StatusConflict)
	}
}

func statusCodeOrZero(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
