package session

import (
	"encoding/json"
	"testing"
)

func TestSessionSpec_MarshalJSON_RedactsEnv(t *testing.T) {
	spec := SessionSpec{
		Agent:    "claude",
		GitToken: "ghp_secret123",
		Env: map[string]string{
			"ANTHROPIC_API_KEY": "sk-ant-secret",
			"OPENAI_API_KEY":   "sk-openai-secret",
			"DEBUG":            "true",
		},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var got struct {
		Agent    string            `json:"agent"`
		GitToken string            `json:"git_token"`
		Env      map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if got.GitToken != "***" {
		t.Errorf("git_token not redacted: got %q", got.GitToken)
	}

	if len(got.Env) != 3 {
		t.Fatalf("expected 3 env keys, got %d", len(got.Env))
	}
	for k, v := range got.Env {
		if v != "***" {
			t.Errorf("env[%q] not redacted: got %q", k, v)
		}
	}
}

func TestSessionSpec_MarshalJSON_NilEnv(t *testing.T) {
	spec := SessionSpec{Agent: "shell"}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, exists := got["env"]; exists {
		t.Error("nil env should be omitted from JSON")
	}
}
