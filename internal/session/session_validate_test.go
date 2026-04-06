package session

import (
	"strings"
	"testing"
)

func TestSessionSpecValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    SessionSpec
		wantErr bool
	}{
		{
			name: "valid minimum",
			spec: SessionSpec{Agent: "shell"},
		},
		{
			name: "valid http repo",
			spec: SessionSpec{
				Agent:   "claude",
				RepoURL: "https://github.com/Rsych/zynqel-core.git",
				Branch:  "feature/test_branch",
			},
		},
		{
			name: "valid ssh repo scp style",
			spec: SessionSpec{
				Agent:   "claude",
				RepoURL: "git@github.com:Rsych/zynqel-core.git",
			},
		},
		{
			name: "valid repo with empty branch",
			spec: SessionSpec{
				Agent:   "shell",
				RepoURL: "https://github.com/Rsych/zynqel-core.git",
				Branch:  "",
			},
		},
		{
			name: "valid image and workspace",
			spec: SessionSpec{
				Agent:       "custom_agent",
				Image:       "ghcr.io/rsych/zynqel-agent:latest",
				WorkspaceID: "ws_1",
			},
		},
		{
			name:    "invalid agent uppercase",
			spec:    SessionSpec{Agent: "Shell"},
			wantErr: true,
		},
		{
			name: "invalid repo scheme",
			spec: SessionSpec{
				Agent:   "shell",
				RepoURL: "file:///tmp/repo",
			},
			wantErr: true,
		},
		{
			name: "invalid repo malformed",
			spec: SessionSpec{
				Agent:   "shell",
				RepoURL: "https://",
			},
			wantErr: true,
		},
		{
			name: "invalid branch metachar",
			spec: SessionSpec{
				Agent:  "shell",
				Branch: "main;rm -rf",
			},
			wantErr: true,
		},
		{
			name: "invalid branch hash",
			spec: SessionSpec{
				Agent:  "shell",
				Branch: "feature/#1",
			},
			wantErr: true,
		},
		{
			name: "invalid branch unicode",
			spec: SessionSpec{
				Agent:  "shell",
				Branch: "기능/테스트",
			},
			wantErr: true,
		},
		{
			name: "invalid image reference",
			spec: SessionSpec{
				Agent: "shell",
				Image: "docker://bad",
			},
			wantErr: true,
		},
		{
			name: "invalid image too long",
			spec: SessionSpec{
				Agent: "shell",
				Image: strings.Repeat("a", 256),
			},
			wantErr: true,
		},
		{
			name: "invalid workspace id",
			spec: SessionSpec{
				Agent:       "shell",
				WorkspaceID: "Bad-ID",
			},
			wantErr: true,
		},
		{
			name: "invalid scp repo with dotdot path",
			spec: SessionSpec{
				Agent:   "shell",
				RepoURL: "git@github.com:../../etc/passwd",
			},
			wantErr: true,
		},
		{
			name: "invalid env key",
			spec: SessionSpec{
				Agent: "shell",
				Env: map[string]string{
					"BAD-KEY": "x",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.spec.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}
