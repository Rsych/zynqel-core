package intercept

import (
	"testing"
)

func TestIntercepter_YesNoParentheses(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("Allow Claude to edit file.go? (Y/n): "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	p := prompts[0]
	if p.Text != "Allow Claude to edit file.go?" {
		t.Errorf("text = %q", p.Text)
	}
	if p.Default != "Yes" {
		t.Errorf("default = %q, want Yes", p.Default)
	}
	if len(p.Options) != 2 || p.Options[0] != "Yes" || p.Options[1] != "No" {
		t.Errorf("options = %v", p.Options)
	}
}

func TestIntercepter_YesNoBrackets(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("Continue? [y/N]: "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	p := prompts[0]
	if p.Text != "Continue?" {
		t.Errorf("text = %q", p.Text)
	}
	if p.Default != "No" {
		t.Errorf("default = %q, want No", p.Default)
	}
}

func TestIntercepter_InquirerStyle(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("? Do you want to continue (Y/n) "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	p := prompts[0]
	if p.Text != "Do you want to continue" {
		t.Errorf("text = %q", p.Text)
	}
}

func TestIntercepter_WithNewline(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("Some output\nProceed? (y/N)\n"))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Default != "No" {
		t.Errorf("default = %q, want No", prompts[0].Default)
	}
}

func TestIntercepter_ANSIColors(t *testing.T) {
	i := New()
	// ANSI colored prompt.
	prompts := i.Scan([]byte("\x1b[1mAllow edit?\x1b[0m (Y/n): "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Text != "Allow edit?" {
		t.Errorf("text = %q", prompts[0].Text)
	}
}

func TestIntercepter_SplitAcrossChunks(t *testing.T) {
	i := New()

	// First chunk: partial prompt.
	prompts := i.Scan([]byte("Continue?"))
	if len(prompts) != 0 {
		t.Fatalf("expected 0 prompts from partial, got %d", len(prompts))
	}

	// Second chunk: completes the prompt.
	prompts = i.Scan([]byte(" (Y/n): "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt after completion, got %d", len(prompts))
	}
	if prompts[0].Text != "Continue?" {
		t.Errorf("text = %q", prompts[0].Text)
	}
}

func TestIntercepter_NoFalsePositive_ArrayIndex(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("arr[0] = value\n"))
	if len(prompts) != 0 {
		t.Errorf("false positive on array index: %v", prompts)
	}
}

func TestIntercepter_NoFalsePositive_ProgressBar(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("[=====>    ] 50%\n"))
	if len(prompts) != 0 {
		t.Errorf("false positive on progress bar: %v", prompts)
	}
}

func TestIntercepter_NoFalsePositive_FilePath(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("/path/to/[file].txt\n"))
	if len(prompts) != 0 {
		t.Errorf("false positive on file path: %v", prompts)
	}
}

func TestIntercepter_NoFalsePositive_NormalOutput(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("Building project...\nCompilation successful.\n"))
	if len(prompts) != 0 {
		t.Errorf("false positive on normal output: %v", prompts)
	}
}

func TestIntercepter_WordVariant(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte("Delete all files? (yes/no): "))
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Text != "Delete all files?" {
		t.Errorf("text = %q", prompts[0].Text)
	}
}

func TestIntercepter_NoCrashOnEmptyInput(t *testing.T) {
	i := New()
	prompts := i.Scan([]byte{})
	if len(prompts) != 0 {
		t.Errorf("expected 0 prompts from empty, got %d", len(prompts))
	}
}

func TestIntercepter_NoCrashOnBinaryData(t *testing.T) {
	i := New()
	// Random binary data should not crash or produce false positives.
	prompts := i.Scan([]byte{0x00, 0x01, 0xFF, 0xFE, 0x80})
	if len(prompts) != 0 {
		t.Errorf("false positive on binary data: %v", prompts)
	}
}
