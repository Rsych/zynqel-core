package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Rsych/zynqel-core/internal/agentcfg"
	"github.com/Rsych/zynqel-core/internal/policy"
	"github.com/Rsych/zynqel-core/internal/sandbox"
	"github.com/Rsych/zynqel-core/internal/session"
)

type noopSandbox struct{}

func (noopSandbox) Create(context.Context, sandbox.Spec) (string, error)            { return "", nil }
func (noopSandbox) Start(context.Context, string) error                             { return nil }
func (noopSandbox) Stop(context.Context, string) error                              { return nil }
func (noopSandbox) Remove(context.Context, string) error                            { return nil }
func (noopSandbox) Attach(context.Context, string) (sandbox.PTYConn, error)         { return nil, nil }
func (noopSandbox) Exec(context.Context, string, []string) (sandbox.PTYConn, error) { return nil, nil }
func (noopSandbox) ExecRun(context.Context, string, []string) ([]byte, error)       { return nil, nil }
func (noopSandbox) Resize(context.Context, string, int, int) error                  { return nil }
func (noopSandbox) Stats(context.Context, string) (*sandbox.ContainerStats, error) {
	return &sandbox.ContainerStats{}, nil
}
func (noopSandbox) Commit(context.Context, string, string) error     { return nil }
func (noopSandbox) ImageExists(context.Context, string) bool         { return false }
func (noopSandbox) BuildImage(context.Context, string, string) error { return nil }
func (noopSandbox) ListVolumes(context.Context, string) ([]sandbox.VolumeInfo, error) {
	return nil, nil
}
func (noopSandbox) RemoveVolume(context.Context, string) error       { return nil }
func (noopSandbox) CopyVolume(context.Context, string, string) error { return nil }
func (noopSandbox) TagImage(context.Context, string, string) error   { return nil }
func (noopSandbox) RemoveImage(context.Context, string) error        { return nil }

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	sb := noopSandbox{}
	agents := agentcfg.NewStore(filepath.Join(t.TempDir(), "agents.json"))
	manager := session.NewManager(sb, policy.DefaultPolicy(), agents, nil)
	return httptest.NewServer(New(manager, agents, sb, nil, nil))
}

func TestCreateSessionRejectsInvalidInput(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "invalid agent",
			body: `{"agent":"BadAgent"}`,
		},
		{
			name: "invalid repo",
			body: `{"agent":"shell","repo_url":"file:///tmp/repo"}`,
		},
		{
			name: "invalid branch metacharacter",
			body: `{"agent":"shell","branch":"main;rm -rf"}`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/sessions", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatalf("post failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestCreateSessionRejectsLargeBody(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	defer ts.Close()

	large := strings.Repeat("a", 1<<20)
	body := `{"agent":"shell","repo_url":"https://github.com/` + large + `"}`

	resp, err := http.Post(ts.URL+"/sessions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !strings.Contains(got["error"], "too large") {
		t.Fatalf("error = %q, want contains %q", got["error"], "too large")
	}
}

func TestCreateAgentRejectsLargeBody(t *testing.T) {
	t.Parallel()

	ts := newTestServer(t)
	defer ts.Close()

	large := strings.Repeat("a", 1<<20)
	body := `{"name":"custom","command":["echo","ok"],"dockerfile":"` + large + `"}`

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/agents", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(raw), "too large") {
		t.Fatalf("body = %q, want contains %q", string(raw), "too large")
	}
}
